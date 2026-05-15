package api

import (
	"encoding/json"
	"fmt"
	"net/http"
)

// APIError is a parsed core-api error response.
type APIError struct {
	Status  int
	Code    string
	Message string
}

func (e *APIError) Error() string {
	if e.Message != "" {
		return e.Message
	}
	return fmt.Sprintf("api error %d (%s)", e.Status, e.Code)
}

// FriendlyMessage maps API errors to TUI-safe copy.
func (e *APIError) FriendlyMessage() string {
	if e == nil {
		return "Something went wrong."
	}
	switch e.Code {
	case "unauthorized":
		return "Could not authenticate this session. Check gateway HMAC configuration."
	case "rate_limited":
		return "Too many requests. Wait a moment and try again."
	case "conflict":
		return "Not enough stock for one or more items."
	case "invalid_input":
		return "Some fields are invalid. Review your cart and address."
	case "missing_idempotency_key":
		return "Checkout misconfigured (missing idempotency key)."
	case "idempotency_key_reuse":
		return "Checkout already submitted with different data."
	case "not_found":
		return "Order or product not found."
	case "bad_gateway":
		return "Payment provider error. Try again shortly."
	default:
		if e.Message != "" {
			return e.Message
		}
		return "Request failed."
	}
}

func parseAPIError(status int, body []byte) error {
	var env struct {
		Error struct {
			Code    string `json:"code"`
			Message string `json:"message"`
		} `json:"error"`
	}
	if err := json.Unmarshal(body, &env); err == nil && env.Error.Code != "" {
		return &APIError{Status: status, Code: env.Error.Code, Message: env.Error.Message}
	}
	return &APIError{Status: status, Code: http.StatusText(status), Message: string(body)}
}
