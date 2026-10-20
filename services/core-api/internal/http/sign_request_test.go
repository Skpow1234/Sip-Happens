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

func TestSignRequest_UniqueNonces(t *testing.T) {
	req1, _ := http.NewRequest(http.MethodGet, "http://example.test/addresses", nil)
	req2, _ := http.NewRequest(http.MethodGet, "http://example.test/addresses", nil)
	if err := SignRequest(req1, []byte("test-secret"), "dev", "fp"); err != nil {
		t.Fatal(err)
	}
	if err := SignRequest(req2, []byte("test-secret"), "dev", "fp"); err != nil {
		t.Fatal(err)
	}
	if req1.Header.Get(auth.HeaderNonce) == req2.Header.Get(auth.HeaderNonce) {
		t.Fatal("expected distinct nonces across SignRequest calls")
	}
	if req1.Header.Get(auth.HeaderRequestID) == req2.Header.Get(auth.HeaderRequestID) {
		t.Fatal("expected distinct request ids across SignRequest calls")
	}
}
