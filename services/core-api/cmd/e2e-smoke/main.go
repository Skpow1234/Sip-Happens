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

	var lastOrder struct {
		ID     string
		Status string
	}
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
		lastOrder.ID = out.Order.ID
		lastOrder.Status = out.Order.Status
		return nil
	}); err != nil {
		return fmt.Errorf("checkout: %w", err)
	}

	// ---- Parity additions (Terminal.shop / API parity batch) ----

	// /view/init bootstrap
	if err := checkJSON(ctx, client, base+"/view/init?currency=COP&catalog_limit=5&orders_limit=5",
		http.MethodGet, nil, true, secret, keyID, fp, func(b []byte) error {
			var out struct {
				Customer json.RawMessage `json:"customer"`
				Catalog  json.RawMessage `json:"catalog"`
				Orders   json.RawMessage `json:"orders"`
			}
			if err := json.Unmarshal(b, &out); err != nil {
				return err
			}
			if len(out.Customer) == 0 || len(out.Catalog) == 0 || len(out.Orders) == 0 {
				return fmt.Errorf("view/init missing one of customer/catalog/orders")
			}
			return nil
		}); err != nil {
		return fmt.Errorf("view/init: %w", err)
	}

	// /shipping/estimate
	if err := checkJSON(ctx, client, base+"/shipping/estimate?city=Bogot%C3%A1&currency=COP",
		http.MethodGet, nil, true, secret, keyID, fp, func(b []byte) error {
			var out struct {
				City               string `json:"city"`
				Currency           string `json:"currency"`
				ShippingMinorUnits int64  `json:"shipping_minor_units"`
			}
			if err := json.Unmarshal(b, &out); err != nil {
				return err
			}
			if out.City == "" || out.Currency == "" {
				return fmt.Errorf("missing city/currency")
			}
			if out.ShippingMinorUnits <= 0 {
				return fmt.Errorf("shipping_minor_units should be > 0 for Bogotá")
			}
			return nil
		}); err != nil {
		return fmt.Errorf("shipping/estimate: %w", err)
	}

	// /cart lifecycle: GET → PUT item → GET (subtotal > 0) → DELETE → GET (empty)
	if err := checkJSON(ctx, client, base+"/cart?currency=COP", http.MethodGet, nil, true, secret, keyID, fp, func(b []byte) error {
		var v struct {
			Currency string `json:"currency"`
			Items    []any  `json:"items"`
		}
		if err := json.Unmarshal(b, &v); err != nil {
			return err
		}
		if v.Currency == "" {
			return fmt.Errorf("missing currency")
		}
		return nil
	}); err != nil {
		return fmt.Errorf("cart GET: %w", err)
	}
	putItemBody := `{"qty":1}`
	if err := checkJSON(ctx, client, base+"/cart/items/"+productID+"?currency=COP",
		http.MethodPut, []byte(putItemBody), true, secret, keyID, fp, func(b []byte) error {
			var v struct {
				SubtotalMinorUnits int64 `json:"subtotal_minor_units"`
				ItemCount          int   `json:"item_count"`
			}
			if err := json.Unmarshal(b, &v); err != nil {
				return err
			}
			if v.ItemCount < 1 {
				return fmt.Errorf("expected item_count >= 1 after PUT, got %d", v.ItemCount)
			}
			return nil
		}); err != nil {
		return fmt.Errorf("cart PUT item: %w", err)
	}
	if err := checkJSON(ctx, client, base+"/cart?currency=COP",
		http.MethodDelete, nil, true, secret, keyID, fp, func(b []byte) error {
			return nil
		}); err != nil {
		return fmt.Errorf("cart DELETE: %w", err)
	}

	// /tokens CRUD: list → create → list → revoke
	if err := checkJSON(ctx, client, base+"/tokens",
		http.MethodGet, nil, true, secret, keyID, fp, func(b []byte) error {
			return nil
		}); err != nil {
		return fmt.Errorf("tokens GET: %w", err)
	}
	createTokBody := `{"name":"e2e-smoke","scopes":["catalog:read","orders:read"]}`
	var createdTokenID string
	if err := checkJSON(ctx, client, base+"/tokens",
		http.MethodPost, []byte(createTokBody), true, secret, keyID, fp, func(b []byte) error {
			var v struct {
				Token               string `json:"token"`
				PersonalAccessToken struct {
					ID string `json:"id"`
				} `json:"personal_access_token"`
			}
			if err := json.Unmarshal(b, &v); err != nil {
				return err
			}
			if v.Token == "" {
				return fmt.Errorf("missing raw token in response")
			}
			createdTokenID = v.PersonalAccessToken.ID
			return nil
		}); err != nil {
		return fmt.Errorf("tokens POST: %w", err)
	}
	if createdTokenID != "" {
		if err := checkJSON(ctx, client, base+"/tokens/"+createdTokenID,
			http.MethodDelete, nil, true, secret, keyID, fp, func(b []byte) error {
				return nil
			}); err != nil {
			return fmt.Errorf("tokens DELETE: %w", err)
		}
	}

	// Customer-cancel-own-order on the PENDING_PAYMENT order we just made.
	if lastOrder.ID != "" {
		cancelBody := `{"reason":"e2e_smoke"}`
		if err := checkJSON(ctx, client, base+"/orders/"+lastOrder.ID+"/cancel",
			http.MethodPost, []byte(cancelBody), true, secret, keyID, fp, func(b []byte) error {
				var v struct {
					Order struct {
						Status string `json:"Status"`
					} `json:"order"`
				}
				if err := json.Unmarshal(b, &v); err != nil {
					return err
				}
				if v.Order.Status != "CANCELLED" {
					return fmt.Errorf("expected CANCELLED, got %q", v.Order.Status)
				}
				return nil
			}); err != nil {
			return fmt.Errorf("orders/cancel: %w", err)
		}
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
