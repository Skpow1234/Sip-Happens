package emailsubscribe

import (
	"context"
	"testing"
	"time"

	"sip-happens/services/core-api/internal/testutil"
)

// TestSubscribeConfirmUnsubscribe_Flow walks the full happy path against
// a real Postgres so the migration `031_email_subscribers.sql` schema is
// also exercised (PENDING → CONFIRMED → UNSUBSCRIBED).
func TestSubscribeConfirmUnsubscribe_Flow(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	tdb, err := testutil.OpenMigrated(ctx)
	if err != nil {
		t.Skipf("skipping: %v", err)
		return
	}
	defer func() { _ = tdb.CloseFn() }()

	svc := NewService(tdb.DB)

	const email = "flow@example.co"
	out, err := svc.Subscribe(ctx, SubscribeInput{
		Email:            email,
		Source:           "test",
		ConsentIP:        "127.0.0.1",
		ConsentUserAgent: "go-test",
	})
	if err != nil {
		t.Fatalf("subscribe: %v", err)
	}
	if out.AlreadyConfirmed {
		t.Fatal("new subscriber should not be AlreadyConfirmed")
	}
	if out.RawConfirmationToken == "" {
		t.Fatal("new subscriber should get a confirmation token")
	}
	if got, want := out.Subscriber.Status, StatusPendingConfirmation; got != want {
		t.Fatalf("status: got %q want %q", got, want)
	}

	// Re-submitting the same PENDING email reissues a fresh token and
	// invalidates the old one. The old token must no longer confirm.
	oldToken := out.RawConfirmationToken
	out2, err := svc.Subscribe(ctx, SubscribeInput{Email: email, Source: "test"})
	if err != nil {
		t.Fatalf("re-subscribe pending: %v", err)
	}
	if out2.RawConfirmationToken == "" || out2.RawConfirmationToken == oldToken {
		t.Fatalf("expected fresh token on re-subscribe pending, got %q", out2.RawConfirmationToken)
	}
	if _, err := svc.Confirm(ctx, oldToken); err == nil {
		t.Fatal("expected old PENDING token to be rejected after reissue")
	}

	// Confirm with the fresh token.
	sub, err := svc.Confirm(ctx, out2.RawConfirmationToken)
	if err != nil {
		t.Fatalf("confirm: %v", err)
	}
	if sub.Status != StatusConfirmed || sub.ConfirmedAt == nil {
		t.Fatalf("expected CONFIRMED with timestamp, got %+v", sub)
	}

	// Re-submitting after CONFIRMED is a no-op (AlreadyConfirmed=true,
	// empty token — handler must not resend the email).
	out3, err := svc.Subscribe(ctx, SubscribeInput{Email: email})
	if err != nil {
		t.Fatalf("re-subscribe confirmed: %v", err)
	}
	if !out3.AlreadyConfirmed || out3.RawConfirmationToken != "" {
		t.Fatalf("re-subscribe of CONFIRMED: got %+v", out3)
	}

	// Confirming a second time with the already-used token must be
	// idempotent (returns subscriber, no error) — protects against double
	// clicks on confirmation links.
	if _, err := svc.Confirm(ctx, out2.RawConfirmationToken); err != nil {
		t.Fatalf("idempotent re-confirm: %v", err)
	}

	// Unsubscribe → status UNSUBSCRIBED.
	out4, err := svc.Unsubscribe(ctx, email)
	if err != nil {
		t.Fatalf("unsubscribe: %v", err)
	}
	if out4 == nil || out4.Status != StatusUnsubscribed {
		t.Fatalf("expected UNSUBSCRIBED, got %+v", out4)
	}

	// Re-subscribing after unsubscribe re-opens to PENDING (Ley 1581:
	// re-consent required after revocation).
	out5, err := svc.Subscribe(ctx, SubscribeInput{Email: email, Source: "test"})
	if err != nil {
		t.Fatalf("re-subscribe after unsubscribe: %v", err)
	}
	if out5.Subscriber.Status != StatusPendingConfirmation || out5.RawConfirmationToken == "" {
		t.Fatalf("expected new PENDING + fresh token, got %+v", out5)
	}
}

// TestSubscribe_RejectsInvalidEmail pins the validation gate.
func TestSubscribe_RejectsInvalidEmail(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	tdb, err := testutil.OpenMigrated(ctx)
	if err != nil {
		t.Skipf("skipping: %v", err)
		return
	}
	defer func() { _ = tdb.CloseFn() }()

	svc := NewService(tdb.DB)

	cases := []string{"", "   ", "noatsign"}
	for _, c := range cases {
		if _, err := svc.Subscribe(ctx, SubscribeInput{Email: c}); err != ErrInvalidEmail {
			t.Fatalf("email %q: expected ErrInvalidEmail, got %v", c, err)
		}
	}
}

// TestConfirm_ExpiredTokenRejected verifies the 48h TTL fence.
//
// We seed the row directly with `expires_at` in the past instead of
// waiting 48h. The service must reject the token AND flip the subscriber
// to EXPIRED so the operator can tell apart "never clicked" from
// "still pending".
func TestConfirm_ExpiredTokenRejected(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	tdb, err := testutil.OpenMigrated(ctx)
	if err != nil {
		t.Skipf("skipping: %v", err)
		return
	}
	defer func() { _ = tdb.CloseFn() }()

	svc := NewService(tdb.DB)
	const email = "expired@example.co"
	out, err := svc.Subscribe(ctx, SubscribeInput{Email: email})
	if err != nil {
		t.Fatalf("subscribe: %v", err)
	}
	// Force-expire the most recent token row.
	if _, err := tdb.DB.ExecContext(ctx, `
update email_confirmation_tokens
set expires_at = now() - interval '1 hour'
where subscriber_id = $1
`, out.Subscriber.ID); err != nil {
		t.Fatalf("force expire: %v", err)
	}
	if _, err := svc.Confirm(ctx, out.RawConfirmationToken); err == nil {
		t.Fatal("expected expired token to be rejected")
	}
	// Subscriber should now be EXPIRED (was PENDING).
	row := tdb.DB.QueryRowContext(ctx, `select status from email_subscribers where id = $1`, out.Subscriber.ID)
	var status string
	if err := row.Scan(&status); err != nil {
		t.Fatal(err)
	}
	if status != StatusExpired {
		t.Fatalf("status after expired confirm: got %q want %q", status, StatusExpired)
	}
}
