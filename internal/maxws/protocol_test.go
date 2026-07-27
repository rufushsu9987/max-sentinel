package maxws

import "testing"

func TestNewSubscriptionRequestUsesSupportedDepth(t *testing.T) {
	request := newSubscriptionRequest([]string{"btctwd"}, 50)
	if len(request.Subscriptions) != 3 {
		t.Fatalf("subscriptions = %d, want 3", len(request.Subscriptions))
	}
	if request.Subscriptions[0].Channel != "book" || request.Subscriptions[0].Depth != 50 {
		t.Fatalf("unexpected book subscription: %#v", request.Subscriptions[0])
	}
	if request.Subscriptions[1].Depth != 0 || request.Subscriptions[2].Depth != 0 {
		t.Fatalf("trade/ticker depth must be omitted: %#v", request.Subscriptions)
	}
}

func TestNewSubscriptionRequestFallsBackForUnsupportedDepth(t *testing.T) {
	request := newSubscriptionRequest([]string{"btctwd"}, 42)
	if got := request.Subscriptions[0].Depth; got != 20 {
		t.Fatalf("depth = %d, want 20", got)
	}
}
