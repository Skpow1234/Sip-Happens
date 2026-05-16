package obslog

import (
	"context"
	"testing"
)

func TestFieldsFromContext(t *testing.T) {
	ctx := WithRequestID(context.Background(), "req-1")
	ctx = WithOrderID(ctx, "ord-1")
	ctx = WithCustomerID(ctx, "cust-1")
	f := FieldsFromContext(ctx)
	if f["request_id"] != "req-1" || f["order_id"] != "ord-1" || f["customer_id"] != "cust-1" {
		t.Fatalf("unexpected fields: %#v", f)
	}
}
