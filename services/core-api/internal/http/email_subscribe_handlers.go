package http

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/url"
	"strings"
	"time"

	"sip-happens/pkg/obslog"
	"sip-happens/services/core-api/internal/emailsubscribe"
)

// confirmEmailSendTimeout bounds the background send of the double-opt-in
// email so a misbehaving SMTP server can't pile up goroutines.
const confirmEmailSendTimeout = 10 * time.Second

// handleEmailSubscribe implements POST /email — the public newsletter
// subscription endpoint, with Ley 1581-compliant double-opt-in.
//
//	POST /email
//	{
//	  "email": "user@example.co",
//	  "source": "web"                          # optional, default "web"
//	}
//
// Public (unsigned) endpoint — this is the entry point for marketing
// pages that don't have an authenticated session. Rate-limited via
// checkoutRL to prevent enumeration / mass-signup spam. Idempotent on
// repeat submissions: an already-CONFIRMED email returns 200 with
// `{"already_confirmed": true}` instead of re-sending email; a still-
// PENDING email gets a fresh confirmation token (old one invalidated).
//
// Side effects:
//   - Insert/update email_subscribers row.
//   - Insert email_confirmation_tokens row.
//   - Capture consent IP + User-Agent (Ley 1581 §9: informed, verifiable
//     consent — the IP/UA are how we prove "informed origin" if SIC asks).
//   - Send the confirmation email (fire-and-forget; failures are logged
//     but don't fail the API response — operator can manually resend via
//     a future admin endpoint).
//
// Response shape on success:
//
//	200 {
//	  "subscriber": { "id": "sub_…", "email": "…", "status": "PENDING_CONFIRMATION", ... },
//	  "already_confirmed": false
//	}
func (s *Server) handleEmailSubscribe(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	if s.EmailSubscribe == nil {
		writeAPIError(w, r, http.StatusInternalServerError, "misconfigured", "email subscribe service not configured")
		return
	}

	var body struct {
		Email  string `json:"email"`
		Source string `json:"source"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeAPIError(w, r, http.StatusBadRequest, "bad_request", "invalid json body")
		return
	}

	in := emailsubscribe.SubscribeInput{
		Email:            body.Email,
		Source:           body.Source,
		ConsentIP:        clientIP(r),
		ConsentUserAgent: r.Header.Get("User-Agent"),
	}
	result, err := s.EmailSubscribe.Subscribe(r.Context(), in)
	if err != nil {
		if errors.Is(err, emailsubscribe.ErrInvalidEmail) {
			writeAPIError(w, r, http.StatusUnprocessableEntity, "invalid_input", "invalid email")
			return
		}
		writeAPIError(w, r, http.StatusInternalServerError, "internal", "internal error")
		return
	}

	// Fire the confirmation email outside the response path. We
	// intentionally swallow errors here: SMTP being down should not
	// fail the subscribe API (the row is already in the DB and an
	// operator can resend).
	if result.RawConfirmationToken != "" && s.Email != nil {
		go s.sendSubscribeConfirmation(result.Subscriber.Email, result.RawConfirmationToken)
	}

	w.Header().Set("content-type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{
		"subscriber":        result.Subscriber,
		"already_confirmed": result.AlreadyConfirmed,
	})
}

// handleEmailConfirm implements GET /email/confirm?token=<raw> — the
// landing page for the double-opt-in confirmation link. Public, no rate
// limit (each token is single-use and tied to a specific row).
//
// On success returns 200 JSON. On invalid/expired token returns 400. To
// keep responses indistinguishable (don't leak whether a token "exists"
// vs is expired) we return the same generic message for both errors and
// HTTP 400 in both cases.
func (s *Server) handleEmailConfirm(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	if s.EmailSubscribe == nil {
		writeAPIError(w, r, http.StatusInternalServerError, "misconfigured", "email subscribe service not configured")
		return
	}
	tok := strings.TrimSpace(r.URL.Query().Get("token"))
	if tok == "" {
		writeAPIError(w, r, http.StatusBadRequest, "bad_request", "missing token")
		return
	}
	sub, err := s.EmailSubscribe.Confirm(r.Context(), tok)
	if err != nil {
		writeAPIError(w, r, http.StatusBadRequest, "invalid_token", "confirmation link is invalid or expired")
		return
	}
	w.Header().Set("content-type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{
		"subscriber": sub,
		"status":     "CONFIRMED",
	})
}

// handleEmailUnsubscribe implements POST /email/unsubscribe — exposed in
// the compliance footer of every marketing email.
//
//	POST /email/unsubscribe
//	{ "email": "user@example.co" }
func (s *Server) handleEmailUnsubscribe(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	if s.EmailSubscribe == nil {
		writeAPIError(w, r, http.StatusInternalServerError, "misconfigured", "email subscribe service not configured")
		return
	}
	var body struct {
		Email string `json:"email"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeAPIError(w, r, http.StatusBadRequest, "bad_request", "invalid json body")
		return
	}
	sub, err := s.EmailSubscribe.Unsubscribe(r.Context(), body.Email)
	if err != nil {
		if errors.Is(err, emailsubscribe.ErrInvalidEmail) {
			writeAPIError(w, r, http.StatusUnprocessableEntity, "invalid_input", "invalid email")
			return
		}
		writeAPIError(w, r, http.StatusInternalServerError, "internal", "internal error")
		return
	}
	w.Header().Set("content-type", "application/json")
	if sub == nil {
		// No row at all for that email — treat as success (don't reveal
		// whether the email was ever subscribed).
		_ = json.NewEncoder(w).Encode(map[string]any{"status": "UNSUBSCRIBED"})
		return
	}
	_ = json.NewEncoder(w).Encode(map[string]any{
		"subscriber": sub,
		"status":     "UNSUBSCRIBED",
	})
}

func (s *Server) sendSubscribeConfirmation(email, rawToken string) {
	if s.Email == nil {
		return
	}
	base := strings.TrimRight(s.PublicBaseURL, "/")
	if base == "" {
		base = "http://localhost:8080"
	}
	link := base + "/email/confirm?token=" + url.QueryEscape(rawToken)
	ctx, cancel := context.WithTimeout(context.Background(), confirmEmailSendTimeout)
	defer cancel()
	if err := s.Email.SendSubscribeConfirmation(ctx, email, link, int(emailsubscribe.ConfirmationTokenTTL.Hours())); err != nil {
		obslog.Warn(ctx, "email_subscribe_confirmation_failed", map[string]any{
			"email": email,
			"err":   err.Error(),
		})
	}
}

// clientIP is defined in observability.go and reused here for consent capture.
