package bus

import (
	"context"
	"crypto/tls"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/segmentio/kafka-go"
	"github.com/segmentio/kafka-go/sasl"
	"github.com/segmentio/kafka-go/sasl/plain"
	"github.com/segmentio/kafka-go/sasl/scram"
)

type Options struct {
	TLSEnabled    bool
	SASLMechanism string
	SASLUsername  string
	SASLPassword  string
}

type Publisher struct {
	writer *kafka.Writer
}

func NewPublisher(brokers []string, topic string, options Options) (*Publisher, error) {
	writer, err := newWriter(brokers, topic, options)
	if err != nil {
		return nil, err
	}
	return &Publisher{writer: writer}, nil
}

func newWriter(brokers []string, topic string, options Options) (*kafka.Writer, error) {
	tlsConfig, mechanism, err := security(options)
	if err != nil {
		return nil, err
	}
	writer := &kafka.Writer{
		Addr:                   kafka.TCP(brokers...),
		Topic:                  topic,
		Balancer:               &kafka.Hash{},
		BatchTimeout:           10 * time.Millisecond,
		BatchSize:              100,
		RequiredAcks:           kafka.RequireAll,
		AllowAutoTopicCreation: true,
	}
	if tlsConfig != nil || mechanism != nil {
		writer.Transport = &kafka.Transport{TLS: tlsConfig, SASL: mechanism}
	}
	return writer, nil
}

func (p *Publisher) Publish(ctx context.Context, key string, value any) error {
	payload, err := json.Marshal(value)
	if err != nil {
		return fmt.Errorf("marshal event: %w", err)
	}
	return p.writer.WriteMessages(ctx, kafka.Message{
		Key:   []byte(key),
		Value: payload,
		Time:  time.Now().UTC(),
	})
}

func (p *Publisher) Close() error { return p.writer.Close() }

type Handler func(context.Context, kafka.Message) error

type PermanentError struct {
	Err error
}

func (e *PermanentError) Error() string { return e.Err.Error() }
func (e *PermanentError) Unwrap() error { return e.Err }

// Permanent marks a handler failure as non-retryable. The consumer publishes the
// failed record to <source-topic>.dlq before committing its source offset.
func Permanent(err error) error {
	if err == nil {
		return nil
	}
	return &PermanentError{Err: err}
}

type Consumer struct {
	reader     *kafka.Reader
	deadLetter *kafka.Writer
	logger     *slog.Logger
}

func NewConsumer(brokers []string, topic, groupID string, logger *slog.Logger, options Options) (*Consumer, error) {
	if logger == nil {
		logger = slog.Default()
	}
	tlsConfig, mechanism, err := security(options)
	if err != nil {
		return nil, err
	}
	dialer := &kafka.Dialer{
		Timeout:       10 * time.Second,
		DualStack:     true,
		TLS:           tlsConfig,
		SASLMechanism: mechanism,
	}
	deadLetter, err := newWriter(brokers, topic+".dlq", options)
	if err != nil {
		return nil, fmt.Errorf("configure dead-letter writer: %w", err)
	}
	return &Consumer{
		reader: kafka.NewReader(kafka.ReaderConfig{
			Brokers:        brokers,
			Topic:          topic,
			GroupID:        groupID,
			MinBytes:       1,
			MaxBytes:       10e6,
			CommitInterval: 0,
			StartOffset:    kafka.LastOffset,
			Dialer:         dialer,
		}),
		deadLetter: deadLetter,
		logger:     logger,
	}, nil
}

func (c *Consumer) Run(ctx context.Context, handler Handler) error {
	for {
		message, err := c.reader.FetchMessage(ctx)
		if err != nil {
			return err
		}
		started := time.Now()
		for {
			err := handler(ctx, message)
			if err == nil {
				break
			}
			if ctx.Err() != nil {
				return ctx.Err()
			}

			var permanent *PermanentError
			if errors.As(err, &permanent) {
				if publishErr := c.publishDeadLetter(ctx, message, permanent.Err); publishErr != nil {
					return fmt.Errorf("publish Kafka dead-letter record: %w", publishErr)
				}
				c.logger.Error("Kafka message sent to dead-letter topic", "topic", message.Topic, "partition", message.Partition, "offset", message.Offset, "error", permanent.Err)
				break
			}

			c.logger.Error("Kafka handler failed; retrying the same message", "topic", message.Topic, "partition", message.Partition, "offset", message.Offset, "error", err)
			timer := time.NewTimer(time.Second)
			select {
			case <-ctx.Done():
				if !timer.Stop() {
					<-timer.C
				}
				return ctx.Err()
			case <-timer.C:
			}
		}
		if err := c.reader.CommitMessages(ctx, message); err != nil {
			return fmt.Errorf("commit Kafka message: %w", err)
		}
		c.logger.Debug("Kafka message handled", "topic", message.Topic, "offset", message.Offset, "duration", time.Since(started))
	}
}

type deadLetterEnvelope struct {
	SchemaVersion int                `json:"schema_version"`
	FailedAt      time.Time          `json:"failed_at"`
	SourceTopic   string             `json:"source_topic"`
	Partition     int                `json:"partition"`
	Offset        int64              `json:"offset"`
	KeyBase64     string             `json:"key_base64,omitempty"`
	ValueBase64   string             `json:"value_base64"`
	Headers       []deadLetterHeader `json:"headers,omitempty"`
	Error         string             `json:"error"`
}

type deadLetterHeader struct {
	Key         string `json:"key"`
	ValueBase64 string `json:"value_base64"`
}

func (c *Consumer) publishDeadLetter(ctx context.Context, message kafka.Message, cause error) error {
	headers := make([]deadLetterHeader, 0, len(message.Headers))
	for _, header := range message.Headers {
		headers = append(headers, deadLetterHeader{Key: header.Key, ValueBase64: base64.StdEncoding.EncodeToString(header.Value)})
	}
	envelope := deadLetterEnvelope{
		SchemaVersion: 1,
		FailedAt:      time.Now().UTC(),
		SourceTopic:   message.Topic,
		Partition:     message.Partition,
		Offset:        message.Offset,
		KeyBase64:     base64.StdEncoding.EncodeToString(message.Key),
		ValueBase64:   base64.StdEncoding.EncodeToString(message.Value),
		Headers:       headers,
		Error:         cause.Error(),
	}
	payload, err := json.Marshal(envelope)
	if err != nil {
		return fmt.Errorf("marshal dead-letter envelope: %w", err)
	}
	return c.deadLetter.WriteMessages(ctx, kafka.Message{
		Key:   message.Key,
		Value: payload,
		Time:  envelope.FailedAt,
	})
}

func (c *Consumer) Close() error {
	return errors.Join(c.reader.Close(), c.deadLetter.Close())
}

func security(options Options) (*tls.Config, sasl.Mechanism, error) {
	var tlsConfig *tls.Config
	if options.TLSEnabled {
		tlsConfig = &tls.Config{MinVersion: tls.VersionTLS12}
	}
	mechanismName := strings.ToUpper(strings.TrimSpace(options.SASLMechanism))
	if mechanismName == "" || mechanismName == "NONE" {
		return tlsConfig, nil, nil
	}
	if options.SASLUsername == "" || options.SASLPassword == "" {
		return nil, nil, fmt.Errorf("Kafka SASL username and password are required for %s", mechanismName)
	}
	switch mechanismName {
	case "PLAIN":
		return tlsConfig, plain.Mechanism{Username: options.SASLUsername, Password: options.SASLPassword}, nil
	case "SCRAM-SHA-256":
		mechanism, err := scram.Mechanism(scram.SHA256, options.SASLUsername, options.SASLPassword)
		return tlsConfig, mechanism, err
	case "SCRAM-SHA-512":
		mechanism, err := scram.Mechanism(scram.SHA512, options.SASLUsername, options.SASLPassword)
		return tlsConfig, mechanism, err
	default:
		return nil, nil, fmt.Errorf("unsupported Kafka SASL mechanism %q", options.SASLMechanism)
	}
}
