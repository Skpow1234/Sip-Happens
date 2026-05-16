package http

import (
	"encoding/json"
	"net/http"
)

// Stable error response shape for all endpoints.
//
// Example:
// {
//   "error": {
//     "code": "invalid_input",
//     "message": "one or more fields are invalid",
//     "request_id": "...."
//   }
// }
type apiError struct {
	Code      string `json:"code"`
	Message   string `json:"message"`
	RequestID string `json:"request_id,omitempty"`
}

type apiErrorEnvelope struct {
	Error apiError `json:"error"`
}

func writeAPIError(w http.ResponseWriter, r *http.Request, status int, code, message string) {
	if code == "" {
		code = "internal"
	}
	if message == "" {
		message = http.StatusText(status)
	}
	w.Header().Set("content-type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(apiErrorEnvelope{
		Error: apiError{
			Code:      code,
			Message:   message,
			RequestID: RequestIDFromHeader(r),
		},
	})
}
