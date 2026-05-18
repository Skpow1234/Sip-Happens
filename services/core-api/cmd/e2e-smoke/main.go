// Command e2e-smoke runs a minimal HTTP smoke suite against a running core-api.
package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"

	httpserver "sip-happens/services/core-api/internal/http"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintf(os.Stderr, "e2e-smoke failed: %v\n", err)
		os.Exit(1)
	}
	fmt.Println("e2e-smoke: ok")
}

func run() error {
	base := strings.TrimRight(getenv("BASE_URL", "http://localhost:8080"), "/")
	fp := getenv("SSH_FINGERPRINT", "demo:ssh-fingerprint")
	keyID := getenv("CORE_API_HMAC_KEY_ID", "dev")
	secret := getenv("CORE_API_HMAC_SECRET", "")
	if secret == "" {
		// dev:dev-hmac-secret from Compose defaults
		keys := getenv("CORE_API_HMAC_KEYS", "dev:dev-hmac-secret")
		for _, part := range strings.Split(keys, ",") {
			part = strings.TrimSpace(part)
			kid, sec, ok := strings.Cut(part, ":")
			if ok && strings.TrimSpace(kid) == keyID {
				secret = strings.TrimSpace(sec)
				break
			}
		}
	}
	if secret == "" {
		return fmt.Errorf("set CORE_API_HMAC_SECRET or CORE_API_HMAC_KEYS")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	client := &http.Client{Timeout: 15 * time.Second}

	if err := checkJSON(ctx, client, base+"/healthz", http.MethodGet, nil, false, secret, keyID, fp, func(b []byte) error {
		var out struct {
			OK bool `json:"ok"`
		}
		if err := json.Unmarshal(b, &out); err != nil {
			return err
		}
		if !out.OK {
			return fmt.Errorf("healthz not ok")
		}
		return nil
	}); err != nil {
		return fmt.Errorf("healthz: %w", err)
	}

	if err := waitReady(ctx, client, base, secret, keyID, fp); err != nil {
		return fmt.Errorf("readyz: %w", err)
	}

	var catalog struct {
		Items []struct {
			Product struct {
				ID string `json:"ID"`
			} `json:"Product"`
			AvailableQty int `json:"available_qty"`
		} `json:"items"`
	}
	if err := checkJSON(ctx, client, base+"/catalog?currency=COP", http.MethodGet, nil, false, secret, keyID, fp, func(b []byte) error {
		return json.Unmarshal(b, &catalog)
	}); err != nil {
		return fmt.Errorf("catalog: %w", err)
	}
	if len(catalog.Items) == 0 {
		return fmt.Errorf("catalog: expected at least one item (run seed demo?)")
	}

	productID := ""
	for _, it := range catalog.Items {
		if it.AvailableQty > 0 && it.Product.ID != "" {
			productID = it.Product.ID
			break
		}
	}
	if productID == "" {
		return fmt.Errorf("catalog: no in-stock product")
	}

	if err := checkJSON(ctx, client, base+"/customers/me", http.MethodGet, nil, true, secret, keyID, fp, func(b []byte) error {
		var out struct {
			Customer struct {
				ID string `json:"id"`
			} `json:"customer"`
		}
		if err := json.Unmarshal(b, &out); err != nil {
			return err
		}
		if out.Customer.ID == "" {
			return fmt.Errorf("missing customer.id")
		}
		return nil
	}); err != nil {
		return fmt.Errorf("customers/me: %w", err)
	}

	if err := checkJSON(ctx, client, base+"/orders", http.MethodGet, nil, true, secret, keyID, fp, func(b []byte) error {
		var out struct {
			Orders []any `json:"orders"`
		}
		if err := json.Unmarshal(b, &out); err != nil {
			return err
		}
		if out.Orders == nil {
			return fmt.Errorf("missing orders array")
		}
		return nil
	}); err != nil {
		return fmt.Errorf("orders: %w", err)
	}

	checkoutBody := fmt.Sprintf(`{
  "currency":"COP",
  "city":"Bogotá",
  "neighborhood":"Chapinero",
  "address_line1":"Calle 1",
  "phone":"3001234567",
  "legal_id":"123",
  "full_name":"Smoke Test",
  "birth_date":"1995-01-01",
  "privacy_consent":true,
  "items":[{"product_id":%q,"qty":1}]
}`, productID)

	if err := checkJSON(ctx, client, base+"/checkout", http.MethodPost, []byte(checkoutBody), true, secret, keyID, fp, func(b []byte) error {
		var out struct {
			Order struct {
				ID     string `json:"ID"`
				Status string `json:"Status"`
			} `json:"order"`
			Stripe struct {
				URL string `json:"URL"`
			} `json:"stripe"`
		}
		if err := json.Unmarshal(b, &out); err != nil {
			return err
		}
		if out.Order.ID == "" || out.Order.Status != "PENDING_PAYMENT" {
			return fmt.Errorf("unexpected order: %+v", out.Order)
		}
		if out.Stripe.URL == "" {
			return fmt.Errorf("missing stripe.URL")
		}
		return nil
	}); err != nil {
		return fmt.Errorf("checkout: %w", err)
	}

	return nil
}

func waitReady(ctx context.Context, client *http.Client, base, secret, keyID, fp string) error {
	deadline := time.Now().Add(30 * time.Second)
	var lastErr error
	for time.Now().Before(deadline) {
		err := checkJSON(ctx, client, base+"/readyz", http.MethodGet, nil, false, secret, keyID, fp, func(b []byte) error {
			var out struct {
				OK bool `json:"ok"`
			}
			if err := json.Unmarshal(b, &out); err != nil {
				return err
			}
			if !out.OK {
				return fmt.Errorf("not ready")
			}
			return nil
		})
		if err == nil {
			return nil
		}
		lastErr = err
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(1 * time.Second):
		}
	}
	return lastErr
}

func checkJSON(ctx context.Context, client *http.Client, url, method string, body []byte, signed bool, secret, keyID, fp string, validate func([]byte) error) error {
	var r io.Reader
	if body != nil {
		r = bytes.NewReader(body)
	}
	req, err := http.NewRequestWithContext(ctx, method, url, r)
	if err != nil {
		return err
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if method == http.MethodPost {
		req.Header.Set("Idempotency-Key", "e2e-smoke-"+time.Now().Format("20060102150405"))
	}
	if signed {
		if err := httpserver.SignRequest(req, []byte(secret), keyID, fp); err != nil {
			return err
		}
	}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	b, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("status %d body=%s", resp.StatusCode, string(b))
	}
	return validate(b)
}

func getenv(k, def string) string {
	if v := strings.TrimSpace(os.Getenv(k)); v != "" {
		return v
	}
	return def
}
