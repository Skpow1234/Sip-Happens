// Package emailsubscribe implements the public POST /email newsletter
// subscription endpoint with double-opt-in confirmation. Compliance with
// Ley 1581 (Colombia data protection) and Decreto 1377 of 2013 (informed,
// verifiable consent for marketing) is the reason for the confirmation
// step: clicking the link in the confirmation email is what makes the
// consent "verifiable".
package emailsubscribe

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"database/sql"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
	"time"

	"sip-happens/pkg/domain"
	"sip-happens/pkg/ids"
)

// ConfirmationTokenTTL is the validity window for a double-opt-in link.
// 48 hours matches industry norm (Mailchimp uses 24h, SendGrid 48h);
// short enough that an old leaked DB dump can't be replayed, long enough
// for someone on a long weekend.
const ConfirmationTokenTTL = 48 * time.Hour

// Subscriber statuses. Free-form strings so we can add new states without
// a migration; the HTTP layer validates known values.
const (
	StatusPendingConfirmation = "PENDING_CONFIRMATION"
	StatusConfirmed           = "CONFIRMED"
	StatusUnsubscribed        = "UNSUBSCRIBED"
	StatusExpired             = "EXPIRED"
)

// Subscriber is one row in email_subscribers.
type Subscriber struct {
	ID                string
	Email             string
	Status            string
	Source            string
	ConsentIP         string
	ConsentUserAgent  string
	ConfirmedAt       *time.Time
	UnsubscribedAt    *time.Time
	CreatedAt         time.Time
	UpdatedAt         time.Time
}

// SubscribeInput is what the POST /email handler builds from the request.
type SubscribeInput struct {
	Email            string
	Source           string // "web" | "ssh-gateway" | "admin-seed"
	ConsentIP        string
	ConsentUserAgent string
}

// SubscribeResult is what the service returns on success.
type SubscribeResult struct {
	Subscriber Subscriber
	// RawConfirmationToken is non-empty for NEW PENDING_CONFIRMATION
	// subscribers — the HTTP layer hands it to the email service to
	// generate the confirmation link. For idempotent re-submissions
	// (already-pending or already-confirmed) it is empty.
	RawConfirmationToken string
	// AlreadyConfirmed signals the email is already CONFIRMED; the
	// handler should return 200 with a "already subscribed" message
	// instead of generating a new confirmation email.
	AlreadyConfirmed bool
}

// Service is the public surface. SQLService is the only implementation.
type Service interface {
	Subscribe(ctx context.Context, in SubscribeInput) (*SubscribeResult, error)

	// Confirm flips a PENDING_CONFIRMATION subscriber to CONFIRMED if
	// the raw token is valid (right prefix + hash matches + not expired
	// + not used). Idempotent on already-confirmed.
	Confirm(ctx context.Context, rawToken string) (*Subscriber, error)

	// Unsubscribe flips a CONFIRMED subscriber to UNSUBSCRIBED. Looked
	// up by email rather than token so user-typed/footer links work.
	// Idempotent on already-unsubscribed.
	Unsubscribe(ctx context.Context, email string) (*Subscriber, error)
}

type SQLService struct {
	DB *sql.DB
}

func NewService(db *sql.DB) *SQLService {
	return &SQLService{DB: db}
}

var (
	// ErrInvalidEmail is returned when the input email fails basic shape
	// validation. We don't attempt RFC-5322 — just non-empty + contains "@".
	ErrInvalidEmail = errors.New("emailsubscribe: invalid email")
	// ErrInvalidToken is returned by Confirm when the token is malformed
	// OR refers to a row that doesn't exist (we return the same error so
	// confirmation links can't be used to enumerate subscribers).
	ErrInvalidToken = errors.New("emailsubscribe: invalid or expired token")
)

func (s *SQLService) Subscribe(ctx context.Context, in SubscribeInput) (*SubscribeResult, error) {
	email := strings.ToLower(strings.TrimSpace(in.Email))
	if email == "" || !strings.Contains(email, "@") {
		return nil, ErrInvalidEmail
	}
	source := strings.TrimSpace(in.Source)
	if source == "" {
		source = "web"
	}

	// Atomic upsert-by-email. If a subscriber row already exists in any
	// status we DO NOT overwrite it — that would lose the confirmed/
	// unsubscribed signal for repeat submissions. Instead we branch:
	//   PENDING_CONFIRMATION  → reissue a fresh token (old one still
	//                           hashes the same — but mark as used).
	//   CONFIRMED             → no-op, set AlreadyConfirmed.
	//   UNSUBSCRIBED          → re-open: flip to PENDING_CONFIRMATION
	//                           and issue a fresh token. Treat as new
	//                           consent (Ley 1581: re-consent required
	//                           after revocation).
	existing, err := s.findByEmail(ctx, email)
	if err != nil && err != domain.ErrNotFound {
		return nil, err
	}

	if existing != nil && existing.Status == StatusConfirmed {
		return &SubscribeResult{
			Subscriber:       *existing,
			AlreadyConfirmed: true,
		}, nil
	}

	now := time.Now().UTC()
	var sub *Subscriber

	if existing == nil {
		newID, err := ids.UUIDv4()
		if err != nil {
			return nil, err
		}
		newID = "sub_" + newID
		if _, err := s.DB.ExecContext(ctx, `
insert into email_subscribers
  (id, email, status, source, consent_ip, consent_user_agent, created_at, updated_at)
values ($1,$2,$3,$4,$5,$6,$7,$7)
`, newID, email, StatusPendingConfirmation, source,
			nullIfEmpty(in.ConsentIP), nullIfEmpty(in.ConsentUserAgent), now); err != nil {
			return nil, err
		}
		sub, err = s.findByID(ctx, newID)
		if err != nil {
			return nil, err
		}
	} else {
		// Re-open or refresh the existing row. Update consent
		// metadata too so we always have the most recent capture.
		if _, err := s.DB.ExecContext(ctx, `
update email_subscribers
set status = $1,
    source = $2,
    consent_ip = coalesce($3, consent_ip),
    consent_user_agent = coalesce($4, consent_user_agent),
    unsubscribed_at = null,
    updated_at = $5
where id = $6
`, StatusPendingConfirmation, source,
			nullIfEmpty(in.ConsentIP), nullIfEmpty(in.ConsentUserAgent), now, existing.ID); err != nil {
			return nil, err
		}
		// Invalidate any still-pending tokens so the old email's link
		// stops working. Best-effort.
		_, _ = s.DB.ExecContext(ctx, `
update email_confirmation_tokens
set used_at = $1
where subscriber_id = $2 and used_at is null
`, now, existing.ID)
		sub, err = s.findByID(ctx, existing.ID)
		if err != nil {
			return nil, err
		}
	}

	// Issue a fresh confirmation token. Same shape as PATs (secret.go):
	// prefix used for indexed lookup, full string SHA-256-hashed for
	// constant-time verification.
	raw, prefix, hash, err := generateConfirmationToken()
	if err != nil {
		return nil, err
	}
	tokID, err := ids.UUIDv4()
	if err != nil {
		return nil, err
	}
	expires := now.Add(ConfirmationTokenTTL)
	if _, err := s.DB.ExecContext(ctx, `
insert into email_confirmation_tokens
  (id, subscriber_id, token_prefix, token_hash, expires_at, created_at)
values ($1,$2,$3,$4,$5,$6)
`, "tok_"+tokID, sub.ID, prefix, hex.EncodeToString(hash), expires, now); err != nil {
		return nil, err
	}

	return &SubscribeResult{
		Subscriber:           *sub,
		RawConfirmationToken: raw,
	}, nil
}

func (s *SQLService) Confirm(ctx context.Context, rawToken string) (*Subscriber, error) {
	prefix, err := parseConfirmationToken(rawToken)
	if err != nil {
		return nil, ErrInvalidToken
	}
	row := s.DB.QueryRowContext(ctx, `
select id, subscriber_id, token_hash, expires_at, used_at
from email_confirmation_tokens
where token_prefix = $1
order by created_at desc
limit 1
`, prefix)
	var (
		tokID, subID, hashHex string
		expiresAt             time.Time
		usedAt                sql.NullTime
	)
	if err := row.Scan(&tokID, &subID, &hashHex, &expiresAt, &usedAt); err != nil {
		if err == sql.ErrNoRows {
			return nil, ErrInvalidToken
		}
		return nil, err
	}
	hash, err := hex.DecodeString(hashHex)
	if err != nil {
		return nil, ErrInvalidToken
	}
	if !verifyConfirmationToken(rawToken, hash) {
		return nil, ErrInvalidToken
	}
	now := time.Now().UTC()
	if now.After(expiresAt) {
		_, _ = s.DB.ExecContext(ctx, `
update email_subscribers set status = $1, updated_at = $2
where id = $3 and status = $4
`, StatusExpired, now, subID, StatusPendingConfirmation)
		return nil, ErrInvalidToken
	}
	// Already-confirmed via this same token: idempotent.
	if usedAt.Valid {
		return s.findByID(ctx, subID)
	}
	// Single-tx flip: mark token used + subscriber confirmed.
	tx, err := s.DB.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback() }()
	if _, err := tx.ExecContext(ctx, `
update email_confirmation_tokens set used_at = $1 where id = $2
`, now, tokID); err != nil {
		return nil, err
	}
	if _, err := tx.ExecContext(ctx, `
update email_subscribers
set status = $1, confirmed_at = $2, updated_at = $2
where id = $3 and status in ($4, $5)
`, StatusConfirmed, now, subID, StatusPendingConfirmation, StatusConfirmed); err != nil {
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return s.findByID(ctx, subID)
}

func (s *SQLService) Unsubscribe(ctx context.Context, email string) (*Subscriber, error) {
	email = strings.ToLower(strings.TrimSpace(email))
	if email == "" {
		return nil, ErrInvalidEmail
	}
	now := time.Now().UTC()
	res, err := s.DB.ExecContext(ctx, `
update email_subscribers
set status = $1, unsubscribed_at = $2, updated_at = $2
where email = $3 and status = $4
`, StatusUnsubscribed, now, email, StatusConfirmed)
	if err != nil {
		return nil, err
	}
	n, _ := res.RowsAffected()
	sub, err := s.findByEmail(ctx, email)
	if err != nil {
		return nil, err
	}
	_ = n
	return sub, nil
}

// --- internals ---

func (s *SQLService) findByEmail(ctx context.Context, email string) (*Subscriber, error) {
	row := s.DB.QueryRowContext(ctx, `
select id, email, status, source,
       coalesce(consent_ip,''), coalesce(consent_user_agent,''),
       confirmed_at, unsubscribed_at, created_at, updated_at
from email_subscribers where email = $1
`, email)
	return scanSubscriber(row)
}

func (s *SQLService) findByID(ctx context.Context, id string) (*Subscriber, error) {
	row := s.DB.QueryRowContext(ctx, `
select id, email, status, source,
       coalesce(consent_ip,''), coalesce(consent_user_agent,''),
       confirmed_at, unsubscribed_at, created_at, updated_at
from email_subscribers where id = $1
`, id)
	return scanSubscriber(row)
}

func scanSubscriber(row *sql.Row) (*Subscriber, error) {
	var s Subscriber
	var confirmed, unsubscribed sql.NullTime
	if err := row.Scan(&s.ID, &s.Email, &s.Status, &s.Source,
		&s.ConsentIP, &s.ConsentUserAgent,
		&confirmed, &unsubscribed, &s.CreatedAt, &s.UpdatedAt); err != nil {
		if err == sql.ErrNoRows {
			return nil, domain.ErrNotFound
		}
		return nil, err
	}
	if confirmed.Valid {
		t := confirmed.Time
		s.ConfirmedAt = &t
	}
	if unsubscribed.Valid {
		t := unsubscribed.Time
		s.UnsubscribedAt = &t
	}
	return &s, nil
}

func nullIfEmpty(v string) any {
	if strings.TrimSpace(v) == "" {
		return nil
	}
	return v
}

// Token shape mirrors tokens/secret.go: `subc_<32-hex-prefix>_<43-char-base64-secret>`.
// Re-implemented locally (rather than reusing the tokens package) to keep
// the prefix label distinct in audit logs and so we can rotate the format
// independently of PATs.

const (
	confirmTokenPrefixLabel = "subc_"
	confirmPrefixBytes      = 16
	confirmSecretBytes      = 32
)

func generateConfirmationToken() (raw, prefix string, hash []byte, err error) {
	prefixBuf := make([]byte, confirmPrefixBytes)
	if _, err := rand.Read(prefixBuf); err != nil {
		return "", "", nil, err
	}
	secretBuf := make([]byte, confirmSecretBytes)
	if _, err := rand.Read(secretBuf); err != nil {
		return "", "", nil, err
	}
	prefix = hex.EncodeToString(prefixBuf)
	secret := base64.RawURLEncoding.EncodeToString(secretBuf)
	raw = confirmTokenPrefixLabel + prefix + "_" + secret
	sum := sha256.Sum256([]byte(raw))
	return raw, prefix, sum[:], nil
}

func parseConfirmationToken(raw string) (string, error) {
	raw = strings.TrimSpace(raw)
	if !strings.HasPrefix(raw, confirmTokenPrefixLabel) {
		return "", fmt.Errorf("invalid confirmation token")
	}
	body := strings.TrimPrefix(raw, confirmTokenPrefixLabel)
	parts := strings.SplitN(body, "_", 2)
	if len(parts) != 2 || len(parts[0]) != confirmPrefixBytes*2 || len(parts[1]) == 0 {
		return "", fmt.Errorf("invalid confirmation token")
	}
	if _, err := hex.DecodeString(parts[0]); err != nil {
		return "", fmt.Errorf("invalid confirmation token")
	}
	return parts[0], nil
}

func verifyConfirmationToken(raw string, storedHash []byte) bool {
	if len(storedHash) != sha256.Size {
		return false
	}
	sum := sha256.Sum256([]byte(strings.TrimSpace(raw)))
	return subtle.ConstantTimeCompare(sum[:], storedHash) == 1
}
