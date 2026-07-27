package runtimeid

import (
	"strings"
	"testing"
)

func TestConsumerGroupSanitizesAndScopesInstance(t *testing.T) {
	got := ConsumerGroup("MAX Sentinel Gateway", "gateway-7f9d8c/pod")
	want := "max-sentinel-gateway-gateway-7f9d8c-pod"
	if got != want {
		t.Fatalf("ConsumerGroup() = %q, want %q", got, want)
	}
}

func TestConsumerGroupBoundsLength(t *testing.T) {
	got := ConsumerGroup(strings.Repeat("prefix", 60), strings.Repeat("instance", 60))
	if len(got) > 240 {
		t.Fatalf("ConsumerGroup() length = %d, want <= 240", len(got))
	}
	if !strings.Contains(got, "-") {
		t.Fatalf("ConsumerGroup() = %q, expected hash suffix", got)
	}
}
