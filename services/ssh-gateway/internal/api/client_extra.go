package api

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// NewBearerClient constructs a Client that authenticates outgoing requests
// with a personal access token (PAT) instead of HMAC. The fingerprint is
// optional — when non-empty it's forwarded as X-SSH-Fingerprint so audit
// rows on core-api remain attributable.
//
// Use this for non-SSH integrations (CI scripts, partner backends) that
// hold a long-lived PAT issued via POST /tokens.
func NewBearerClient(baseURL string, token string, sshFingerprint string) *Client {
	baseURL = strings.TrimRight(baseURL, "/")
	return &Client{
		baseURL: baseURL,
		http: &http.Client{
			Timeout: 15 * time.Second,
			Transport: BearerRoundTripper{
				Token:       token,
				Fingerprint: sshFingerprint,
			},
		},
	}
}

// CancelOwnOrder calls POST /orders/{ref}/cancel on the customer's own
// PENDING_PAYMENT order. Returns 409 Conflict when the order is past
// PENDING_PAYMENT — at that point the customer must request a refund
// (admin operation) rather than self-cancel.
func (c *Client) CancelOwnOrder(ctx context.Context, ref, reason, idempotencyKey string) (*OrderResult, error) {
	ref = strings.TrimSpace(ref)
	if ref == "" {
		return nil, fmt.Errorf("missing order ref")
	}
	body, _ := json.Marshal(map[string]string{"reason": strings.TrimSpace(reason)})
	req, err := http.NewRequestWithContext(ctx, http.MethodPost,
		c.baseURL+"/orders/"+ref+"/cancel", bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	if k := strings.TrimSpace(idempotencyKey); k != "" {
		req.Header.Set(headerIdempotencyKey, k)
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(resp.Body)
	if resp.StatusCode/100 != 2 {
		return nil, parseAPIError(resp.StatusCode, raw)
	}
	var out OrderResult
	if err := json.Unmarshal(raw, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// SubscribeEmail calls POST /email — the public newsletter subscribe
// endpoint with double-opt-in. Unsigned (the route is public), but using
// a signed Client still works because the server doesn't reject extra
// auth headers.
func (c *Client) SubscribeEmail(ctx context.Context, email, source string) (*EmailSubscribeResult, error) {
	body, _ := json.Marshal(map[string]string{
		"email":  strings.TrimSpace(email),
		"source": strings.TrimSpace(source),
	})
	req, err := http.NewRequestWithContext(ctx, http.MethodPost,
		c.baseURL+"/email", bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := c.http.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(resp.Body)
	if resp.StatusCode/100 != 2 {
		return nil, parseAPIError(resp.StatusCode, raw)
	}
	var out EmailSubscribeResult
	if err := json.Unmarshal(raw, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// EmailSubscribeResult mirrors POST /email's response.
type EmailSubscribeResult struct {
	Subscriber       EmailSubscriberView `json:"subscriber"`
	AlreadyConfirmed bool                `json:"already_confirmed"`
}

// EmailSubscriberView mirrors the persisted row.
type EmailSubscriberView struct {
	ID             string     `json:"id"`
	Email          string     `json:"email"`
	Status         string     `json:"status"`
	Source         string     `json:"source"`
	ConfirmedAt    *time.Time `json:"confirmed_at,omitempty"`
	UnsubscribedAt *time.Time `json:"unsubscribed_at,omitempty"`
	CreatedAt      time.Time  `json:"created_at"`
	UpdatedAt      time.Time  `json:"updated_at"`
}

// --- OAuth client CRUD (programmatic apps that need to register / list / revoke
// OAuth clients on behalf of a customer). The gateway TUI doesn't expose
// these today, but partner integrations and the admin console use them. ---

type OAuthClientView struct {
	ID          string    `json:"id"`
	Name        string    `json:"name"`
	RedirectURI string    `json:"redirect_uri"`
	Scopes      []string  `json:"scopes"`
	CreatedAt   time.Time `json:"created_at"`
	RevokedAt   *time.Time `json:"revoked_at,omitempty"`
}

type OAuthClientCreateRequest struct {
	Name        string   `json:"name"`
	RedirectURI string   `json:"redirect_uri"`
	Scopes      []string `json:"scopes"`
}

type OAuthClientCreateResult struct {
	Client       OAuthClientView `json:"client"`
	ClientSecret string          `json:"client_secret,omitempty"`
}

type OAuthClientsPage struct {
	Clients []OAuthClientView `json:"clients"`
}

func (c *Client) ListOAuthClients(ctx context.Context) ([]OAuthClientView, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL+"/oauth/clients", nil)
	if err != nil {
		return nil, err
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(resp.Body)
	if resp.StatusCode/100 != 2 {
		return nil, parseAPIError(resp.StatusCode, raw)
	}
	var out OAuthClientsPage
	if err := json.Unmarshal(raw, &out); err != nil {
		return nil, err
	}
	return out.Clients, nil
}

func (c *Client) CreateOAuthClient(ctx context.Context, in OAuthClientCreateRequest) (*OAuthClientCreateResult, error) {
	body, _ := json.Marshal(in)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost,
		c.baseURL+"/oauth/clients", bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := c.http.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(resp.Body)
	if resp.StatusCode/100 != 2 {
		return nil, parseAPIError(resp.StatusCode, raw)
	}
	var out OAuthClientCreateResult
	if err := json.Unmarshal(raw, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

func (c *Client) GetOAuthClient(ctx context.Context, id string) (*OAuthClientView, error) {
	id = strings.TrimSpace(id)
	if id == "" {
		return nil, fmt.Errorf("missing oauth client id")
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL+"/oauth/clients/"+id, nil)
	if err != nil {
		return nil, err
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(resp.Body)
	if resp.StatusCode/100 != 2 {
		return nil, parseAPIError(resp.StatusCode, raw)
	}
	var out struct {
		Client OAuthClientView `json:"client"`
	}
	if err := json.Unmarshal(raw, &out); err != nil {
		return nil, err
	}
	return &out.Client, nil
}

func (c *Client) RevokeOAuthClient(ctx context.Context, id string) error {
	id = strings.TrimSpace(id)
	if id == "" {
		return fmt.Errorf("missing oauth client id")
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodDelete, c.baseURL+"/oauth/clients/"+id, nil)
	if err != nil {
		return err
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(resp.Body)
	if resp.StatusCode/100 != 2 {
		return parseAPIError(resp.StatusCode, raw)
	}
	return nil
}
