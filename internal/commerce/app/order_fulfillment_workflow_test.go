package app

import "testing"

func TestOrderFulfillmentWorkflowIDUsesTradeNo(t *testing.T) {
	if got, want := orderFulfillmentWorkflowID("order-123"), "order-fulfillment-order-123"; got != want {
		t.Fatalf("workflow id = %q, want %q", got, want)
	}
}
