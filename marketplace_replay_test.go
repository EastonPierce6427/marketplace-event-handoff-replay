package main

import "testing"

func TestHandoffRequiresBothStreams(t *testing.T) {
	l := newLedger()
	cases := []struct {
		event  Event
		action string
	}{
		{Event{ID: "handoff", Kind: "order_handoff", OrderID: "order-7"}, "awaiting_inputs"},
		{Event{ID: "asset", Kind: "seller_asset", OrderID: "order-7", SellerID: "seller-2"}, "asset_recorded"},
		{Event{ID: "handoff", Kind: "order_handoff", OrderID: "order-7"}, "awaiting_inputs"},
		{Event{ID: "buyer", Kind: "buyer_update", OrderID: "order-7", BuyerID: "buyer-3"}, "buyer_recorded"},
		{Event{ID: "handoff", Kind: "order_handoff", OrderID: "order-7"}, "handed_off"},
		{Event{ID: "handoff", Kind: "order_handoff", OrderID: "order-7"}, "already_applied"},
	}
	for i, tc := range cases {
		got, err := l.Apply(tc.event)
		if err != nil || got.Action != tc.action {
			t.Fatalf("step %d: got %q, err %v; want %q", i, got.Action, err, tc.action)
		}
	}
}
