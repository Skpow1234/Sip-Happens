package http

import (
	"net/http"
	"testing"

	"sip-happens/pkg/auth"
)

func TestSignRequest_NilBody(t *testing.T) {
	req, err := http.NewRequest(http.MethodGet, "http://example.test/addresses", nil)
	if err != nil {
		t.Fatal(err)
	}
	if err := SignRequest(req, []byte("test-secret"), "dev", "fp"); err != nil {
		t.Fatalf("SignRequest nil body: %v", err)
	}
	if req.Header.Get(auth.HeaderSignature) == "" {
		t.Fatal("expected signature header")
	}
	if req.Body == nil {
		t.Fatal("expected Body to be replaced with empty reader")
	}
}
