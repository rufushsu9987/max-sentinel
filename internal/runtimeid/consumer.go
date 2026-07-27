package runtimeid

import (
	"crypto/sha256"
	"encoding/hex"
	"os"
	"strings"
	"unicode"
)

// Instance returns a stable-enough identity for the current process lifetime.
// Kubernetes should provide POD_NAME; local and Compose runs fall back to the
// container/host name.
func Instance() string {
	for _, key := range []string{"POD_NAME", "HOSTNAME"} {
		if value := strings.TrimSpace(os.Getenv(key)); value != "" {
			return value
		}
	}
	if hostname, err := os.Hostname(); err == nil && strings.TrimSpace(hostname) != "" {
		return hostname
	}
	return "unknown"
}

// ConsumerGroup creates an instance-scoped Kafka consumer group. Gateway
// replicas need broadcast semantics because each Pod owns an independent
// in-memory latest-snapshot cache.
func ConsumerGroup(prefix, instance string) string {
	prefix = sanitize(prefix)
	if prefix == "" {
		prefix = "max-sentinel-gateway"
	}
	instance = sanitize(instance)
	if instance == "" {
		instance = "unknown"
	}
	candidate := prefix + "-" + instance
	if len(candidate) <= 240 {
		return candidate
	}
	sum := sha256.Sum256([]byte(candidate))
	suffix := hex.EncodeToString(sum[:8])
	maxPrefix := 240 - len(suffix) - 1
	return strings.TrimRight(candidate[:maxPrefix], "-._") + "-" + suffix
}

func sanitize(value string) string {
	value = strings.TrimSpace(strings.ToLower(value))
	var builder strings.Builder
	builder.Grow(len(value))
	lastSeparator := false
	for _, r := range value {
		if unicode.IsLetter(r) || unicode.IsDigit(r) || r == '.' || r == '_' {
			builder.WriteRune(r)
			lastSeparator = false
			continue
		}
		if !lastSeparator {
			builder.WriteByte('-')
			lastSeparator = true
		}
	}
	return strings.Trim(builder.String(), "-._")
}
