package maxws

type subscription struct {
	Channel string `json:"channel"`
	Market  string `json:"market"`
	Depth   int    `json:"depth,omitempty"`
}

type subscribeRequest struct {
	Action        string         `json:"action"`
	Subscriptions []subscription `json:"subscriptions"`
	ID            string         `json:"id"`
}

func newSubscriptionRequest(markets []string, requestedDepth int) subscribeRequest {
	depth := supportedDepth(requestedDepth)
	subscriptions := make([]subscription, 0, len(markets)*3)
	for _, market := range markets {
		subscriptions = append(subscriptions,
			subscription{Channel: "book", Market: market, Depth: depth},
			subscription{Channel: "trade", Market: market},
			subscription{Channel: "ticker", Market: market},
		)
	}
	return subscribeRequest{Action: "sub", Subscriptions: subscriptions, ID: "max-sentinel"}
}

func supportedDepth(depth int) int {
	switch depth {
	case 1, 5, 10, 20, 50:
		return depth
	default:
		return 20
	}
}
