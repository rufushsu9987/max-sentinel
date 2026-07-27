package main

import (
	"testing"
	"time"
)

func TestRunBacktest(t *testing.T) {
	started := time.Date(2026, 7, 27, 0, 0, 0, 0, time.UTC)
	prices := []float64{100, 101, 100, 99, 98, 99, 100, 101, 102, 101, 100, 99, 98, 99, 100, 101, 103, 102, 101, 100, 99, 97, 98, 99, 100, 102, 101, 100, 99, 100}
	ticks := make([]tick, 0, len(prices))
	for index, price := range prices {
		ticks = append(ticks, tick{Timestamp: started.Add(time.Duration(index) * time.Minute), Price: price})
	}
	output, err := run(ticks, "btctwd", 5, 1, 5)
	if err != nil {
		t.Fatal(err)
	}
	if output.Samples != len(prices) {
		t.Fatalf("expected %d samples, got %d", len(prices), output.Samples)
	}
	if output.Trades == 0 {
		t.Fatal("expected at least one simulated trade")
	}
}
