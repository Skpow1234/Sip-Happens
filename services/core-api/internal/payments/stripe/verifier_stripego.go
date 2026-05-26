package stripe

import (
	"errors"
	"time"

	stripewebhook "github.com/stripe/stripe-go/v82/webhook"
)

type StripeGoVerifier struct {
	SigningSecret string
	ToleranceSec  int64 // 0 uses stripe-go default
}

func (v StripeGoVerifier) Verify(payload []byte, stripeSignatureHeader string) error {
	if v.SigningSecret == "" {
		return errors.New("missing stripe webhook signing secret")
	}
	if stripeSignatureHeader == "" {
		return errors.New("missing stripe signature header")
	}
	if v.ToleranceSec > 0 {
		return stripewebhook.ValidatePayloadWithTolerance(payload, stripeSignatureHeader, v.SigningSecret, time.Duration(v.ToleranceSec)*time.Second)
	}
	return stripewebhook.ValidatePayload(payload, stripeSignatureHeader, v.SigningSecret)
}
