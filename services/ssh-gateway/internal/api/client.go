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

const headerIdempotencyKey = "Idempotency-Key"

type Client struct {
	baseURL string
	http    *http.Client
}

func NewClient(baseURL string) *Client {
	baseURL = strings.TrimRight(baseURL, "/")
	return &Client{
		baseURL: baseURL,
		http: &http.Client{
			Timeout: 15 * time.Second,
		},
	}
}

func NewSignedClient(baseURL string, hmacSecret string, sshFingerprint string) *Client {
	baseURL = strings.TrimRight(baseURL, "/")
	secret := []byte(hmacSecret)
	return &Client{
		baseURL: baseURL,
		http: &http.Client{
			Timeout: 15 * time.Second,
			Transport: SigningRoundTripper{
				Secret:      secret,
				KeyID:       "",
				Fingerprint: sshFingerprint,
			},
		},
	}
}

func NewSignedClientV2(baseURL string, keyID string, hmacSecret string, sshFingerprint string) *Client {
	baseURL = strings.TrimRight(baseURL, "/")
	secret := []byte(hmacSecret)
	return &Client{
		baseURL: baseURL,
		http: &http.Client{
			Timeout: 15 * time.Second,
			Transport: SigningRoundTripper{
				Secret:      secret,
				KeyID:       keyID,
				Fingerprint: sshFingerprint,
			},
		},
	}
}

type CatalogItem struct {
	Product struct {
		ID        string  `json:"ID"`
		SKU       string  `json:"SKU"`
		Name      string  `json:"Name"`
		Brewery   string  `json:"Brewery"`
		Style     string  `json:"Style"`
		ABV       float64 `json:"ABV"`
		VolumeML  int     `json:"VolumeML"`
		InvimaReg string  `json:"InvimaReg"`
	} `json:"Product"`
	Price struct {
		Currency         string `json:"Currency"`
		AmountMinorUnits int64  `json:"AmountMinorUnits"`
	} `json:"Price"`
	AvailableQty int `json:"available_qty"`
}

type CatalogDetail struct {
	Product struct {
		ID        string  `json:"ID"`
		SKU       string  `json:"SKU"`
		Name      string  `json:"Name"`
		Brewery   string  `json:"Brewery"`
		Style     string  `json:"Style"`
		ABV       float64 `json:"ABV"`
		VolumeML  int     `json:"VolumeML"`
		InvimaReg string  `json:"InvimaReg"`
	} `json:"product"`
	Price struct {
		Currency         string `json:"Currency"`
		AmountMinorUnits int64  `json:"AmountMinorUnits"`
	} `json:"price"`
	AvailableQty int `json:"available_qty"`
}

func (d CatalogDetail) AsCatalogItem() CatalogItem {
	var it CatalogItem
	it.Product = d.Product
	it.Price = d.Price
	it.AvailableQty = d.AvailableQty
	return it
}

type CheckoutItem struct {
	ProductID string `json:"product_id"`
	Qty       int    `json:"qty"`
}

type CheckoutRequest struct {
	Currency     string         `json:"currency"`
	City         string         `json:"city"`
	Neighborhood string         `json:"neighborhood"`
	AddressLine1 string         `json:"address_line1"`
	Phone        string         `json:"phone"`
	LegalID      string         `json:"legal_id"`
	FullName     string         `json:"full_name"`
	SuccessURL   string         `json:"success_url"`
	CancelURL    string         `json:"cancel_url"`
	Items        []CheckoutItem `json:"items"`
}

type OrderView struct {
	ID              string     `json:"ID"`
	Status          string     `json:"Status"`
	TotalMinorUnits int64      `json:"TotalMinorUnits"`
	Currency        string     `json:"Currency"`
	ExpiresAt       *time.Time `json:"ExpiresAt"`
	CreatedAt       time.Time  `json:"CreatedAt"`
}

type StripeCheckoutView struct {
	CheckoutSessionID string `json:"CheckoutSessionID"`
	PaymentIntentID   string `json:"PaymentIntentID"`
	URL               string `json:"URL"`
}

type CheckoutResult struct {
	Order  OrderView           `json:"order"`
	Stripe *StripeCheckoutView `json:"stripe"`
}

type OrderResult struct {
	Order OrderView `json:"order"`
}

func (c *Client) GetCatalog(ctx context.Context) ([]CatalogItem, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL+"/catalog?currency=COP", nil)
	if err != nil {
		return nil, err
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode/100 != 2 {
		return nil, parseAPIError(resp.StatusCode, body)
	}

	var out struct {
		Currency string        `json:"currency"`
		Items    []CatalogItem `json:"items"`
	}
	if err := json.Unmarshal(body, &out); err != nil {
		return nil, err
	}
	return out.Items, nil
}

func (c *Client) GetCatalogDetail(ctx context.Context, sku string) (*CatalogDetail, error) {
	sku = strings.TrimSpace(sku)
	if sku == "" {
		return nil, fmt.Errorf("missing sku")
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL+"/catalog/"+sku+"?currency=COP", nil)
	if err != nil {
		return nil, err
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode/100 != 2 {
		return nil, parseAPIError(resp.StatusCode, body)
	}

	var out CatalogDetail
	if err := json.Unmarshal(body, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

func (c *Client) Checkout(ctx context.Context, idempotencyKey string, in CheckoutRequest) (*CheckoutResult, error) {
	if strings.TrimSpace(idempotencyKey) == "" {
		idempotencyKey = randomHex(16)
	}
	if in.Currency == "" {
		in.Currency = "COP"
	}
	if in.SuccessURL == "" {
		in.SuccessURL = "https://sip-happens.local/success"
	}
	if in.CancelURL == "" {
		in.CancelURL = "https://sip-happens.local/cancel"
	}

	payload, err := json.Marshal(in)
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/checkout", bytes.NewReader(payload))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set(headerIdempotencyKey, idempotencyKey)

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode/100 != 2 {
		return nil, parseAPIError(resp.StatusCode, body)
	}

	var out struct {
		Order  OrderView           `json:"order"`
		Stripe *StripeCheckoutView `json:"stripe"`
	}
	if err := json.Unmarshal(body, &out); err != nil {
		return nil, err
	}
	return &CheckoutResult{Order: out.Order, Stripe: out.Stripe}, nil
}

type CustomerView struct {
	ID           string `json:"ID"`
	City         string `json:"City"`
	Neighborhood string `json:"Neighborhood"`
	AddressLine1 string `json:"AddressLine1"`
	Phone        string `json:"Phone"`
	LegalID      string `json:"LegalID"`
	FullName     string `json:"FullName"`
}

func (c *Client) GetCustomerMe(ctx context.Context) (*CustomerView, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL+"/customers/me", nil)
	if err != nil {
		return nil, err
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode/100 != 2 {
		return nil, parseAPIError(resp.StatusCode, body)
	}
	var out struct {
		Customer CustomerView `json:"customer"`
	}
	if err := json.Unmarshal(body, &out); err != nil {
		return nil, err
	}
	return &out.Customer, nil
}

func (c *Client) ListOrders(ctx context.Context, limit int) ([]OrderView, error) {
	if limit <= 0 {
		limit = 20
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, fmt.Sprintf("%s/orders?limit=%d", c.baseURL, limit), nil)
	if err != nil {
		return nil, err
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode/100 != 2 {
		return nil, parseAPIError(resp.StatusCode, body)
	}
	var out struct {
		Orders []OrderView `json:"orders"`
	}
	if err := json.Unmarshal(body, &out); err != nil {
		return nil, err
	}
	return out.Orders, nil
}

func (c *Client) GetOrder(ctx context.Context, orderID string) (*OrderResult, error) {
	orderID = strings.TrimSpace(orderID)
	if orderID == "" {
		return nil, fmt.Errorf("missing order id")
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL+"/orders/"+orderID, nil)
	if err != nil {
		return nil, err
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode/100 != 2 {
		return nil, parseAPIError(resp.StatusCode, body)
	}

	var out OrderResult
	if err := json.Unmarshal(body, &out); err != nil {
		return nil, err
	}
	return &out, nil
}
