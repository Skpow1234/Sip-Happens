package obslog

import "context"

type ctxKey string

const (
	requestIDKey   ctxKey = "request_id"
	customerIDKey  ctxKey = "customer_id"
	orderIDKey     ctxKey = "order_id"
)

func WithRequestID(ctx context.Context, requestID string) context.Context {
	if requestID == "" {
		return ctx
	}
	return context.WithValue(ctx, requestIDKey, requestID)
}

func WithCustomerID(ctx context.Context, customerID string) context.Context {
	if customerID == "" {
		return ctx
	}
	return context.WithValue(ctx, customerIDKey, customerID)
}

func WithOrderID(ctx context.Context, orderID string) context.Context {
	if orderID == "" {
		return ctx
	}
	return context.WithValue(ctx, orderIDKey, orderID)
}

func FieldsFromContext(ctx context.Context) map[string]any {
	if ctx == nil {
		return nil
	}
	out := map[string]any{}
	if v, ok := ctx.Value(requestIDKey).(string); ok && v != "" {
		out["request_id"] = v
	}
	if v, ok := ctx.Value(customerIDKey).(string); ok && v != "" {
		out["customer_id"] = v
	}
	if v, ok := ctx.Value(orderIDKey).(string); ok && v != "" {
		out["order_id"] = v
	}
	if len(out) == 0 {
		return nil
	}
	return out
}
