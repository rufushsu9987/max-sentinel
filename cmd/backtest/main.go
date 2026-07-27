package main

import (
	"encoding/csv"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"math"
	"os"
	"strconv"
	"strings"
	"time"
)

type tick struct {
	Timestamp time.Time
	Price     float64
}

type result struct {
	Market           string  `json:"market"`
	Samples          int     `json:"samples"`
	Window           int     `json:"window"`
	EntryZScore      float64 `json:"entry_z_score"`
	Trades           int     `json:"trades"`
	WinningTrades    int     `json:"winning_trades"`
	WinRate          float64 `json:"win_rate"`
	GrossReturn      float64 `json:"gross_return"`
	TransactionCosts float64 `json:"transaction_costs"`
	NetReturn        float64 `json:"net_return"`
	MaxDrawdown      float64 `json:"max_drawdown"`
	AnnualizedSharpe float64 `json:"annualized_sharpe"`
	Disclaimer       string  `json:"disclaimer"`
}

func main() {
	input := flag.String("input", "testdata/btctwd_ticks.csv", "CSV with timestamp,market,price")
	window := flag.Int("window", 20, "rolling mean window")
	entry := flag.Float64("entry-z", 1.25, "entry z-score")
	feeBPS := flag.Float64("fee-bps", 5, "one-way simulated fee in basis points")
	market := flag.String("market", "btctwd", "market filter")
	flag.Parse()

	ticks, err := readTicks(*input, strings.ToLower(*market))
	if err != nil {
		fmt.Fprintln(os.Stderr, "backtest:", err)
		os.Exit(1)
	}
	out, err := run(ticks, strings.ToLower(*market), *window, *entry, *feeBPS)
	if err != nil {
		fmt.Fprintln(os.Stderr, "backtest:", err)
		os.Exit(1)
	}
	encoder := json.NewEncoder(os.Stdout)
	encoder.SetIndent("", "  ")
	_ = encoder.Encode(out)
}

func readTicks(path, market string) ([]tick, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	reader := csv.NewReader(file)
	header, err := reader.Read()
	if err != nil {
		return nil, err
	}
	columns := map[string]int{}
	for index, value := range header {
		columns[strings.ToLower(strings.TrimSpace(value))] = index
	}
	for _, required := range []string{"timestamp", "market", "price"} {
		if _, ok := columns[required]; !ok {
			return nil, fmt.Errorf("missing %q column", required)
		}
	}
	var ticks []tick
	for {
		record, err := reader.Read()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return nil, err
		}
		if strings.ToLower(record[columns["market"]]) != market {
			continue
		}
		at, err := time.Parse(time.RFC3339, record[columns["timestamp"]])
		if err != nil {
			return nil, fmt.Errorf("parse timestamp: %w", err)
		}
		price, err := strconv.ParseFloat(record[columns["price"]], 64)
		if err != nil || price <= 0 {
			return nil, fmt.Errorf("invalid price %q", record[columns["price"]])
		}
		ticks = append(ticks, tick{Timestamp: at, Price: price})
	}
	return ticks, nil
}

func run(ticks []tick, market string, window int, entryZ, feeBPS float64) (result, error) {
	if window < 3 || len(ticks) <= window {
		return result{}, errors.New("not enough samples for configured window")
	}
	position := 0
	entryPrice := 0.0
	gross := 0.0
	costs := 0.0
	trades := 0
	wins := 0
	equity := 1.0
	peak := equity
	maxDrawdown := 0.0
	returns := make([]float64, 0, len(ticks)-window)

	closePosition := func(price float64) {
		if position == 0 {
			return
		}
		tradeReturn := float64(position) * (price - entryPrice) / entryPrice
		gross += tradeReturn
		cost := 2 * feeBPS / 10_000
		costs += cost
		net := tradeReturn - cost
		equity *= 1 + net
		returns = append(returns, net)
		trades++
		if net > 0 {
			wins++
		}
		position = 0
		entryPrice = 0
		if equity > peak {
			peak = equity
		}
		drawdown := (peak - equity) / peak
		if drawdown > maxDrawdown {
			maxDrawdown = drawdown
		}
	}

	for index := window; index < len(ticks); index++ {
		mean, stddev := statistics(ticks[index-window : index])
		if stddev == 0 {
			continue
		}
		price := ticks[index].Price
		z := (price - mean) / stddev
		switch {
		case position == 0 && z <= -entryZ:
			position = 1
			entryPrice = price
		case position == 0 && z >= entryZ:
			position = -1
			entryPrice = price
		case position == 1 && z >= 0:
			closePosition(price)
		case position == -1 && z <= 0:
			closePosition(price)
		}
	}
	if position != 0 {
		closePosition(ticks[len(ticks)-1].Price)
	}
	winRate := 0.0
	if trades > 0 {
		winRate = float64(wins) / float64(trades)
	}
	return result{
		Market:           market,
		Samples:          len(ticks),
		Window:           window,
		EntryZScore:      entryZ,
		Trades:           trades,
		WinningTrades:    wins,
		WinRate:          winRate,
		GrossReturn:      gross,
		TransactionCosts: costs,
		NetReturn:        equity - 1,
		MaxDrawdown:      maxDrawdown,
		AnnualizedSharpe: sharpe(returns),
		Disclaimer:       "Educational simulation only. Results are not investment advice and omit live-market execution effects.",
	}, nil
}

func statistics(values []tick) (float64, float64) {
	var mean float64
	for _, value := range values {
		mean += value.Price
	}
	mean /= float64(len(values))
	var variance float64
	for _, value := range values {
		delta := value.Price - mean
		variance += delta * delta
	}
	variance /= float64(len(values) - 1)
	return mean, math.Sqrt(variance)
}

func sharpe(returns []float64) float64 {
	if len(returns) < 2 {
		return 0
	}
	var mean float64
	for _, value := range returns {
		mean += value
	}
	mean /= float64(len(returns))
	var variance float64
	for _, value := range returns {
		delta := value - mean
		variance += delta * delta
	}
	variance /= float64(len(returns) - 1)
	if variance == 0 {
		return 0
	}
	return mean / math.Sqrt(variance) * math.Sqrt(252)
}
