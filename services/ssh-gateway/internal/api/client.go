package api

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
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
	FullName         string         `json:"full_name"`
	BirthDate        string         `json:"birth_date,omitempty"`
	PrivacyConsent   bool           `json:"privacy_consent"`
	SuccessURL       string         `json:"success_url"`
	CancelURL        string         `json:"cancel_url"`
	Items            []CheckoutItem `json:"items"`
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

type ShipmentView struct {
	Status       string `json:"Status"`
	Carrier      string `json:"Carrier"`
	TrackingCode string `json:"TrackingCode"`
}

type OrderResult struct {
	Order    OrderView     `json:"order"`
	Shipment *ShipmentView `json:"shipment"`
}

// CatalogQuery lists catalog products with optional filters and pagination.
type CatalogQuery struct {
	Currency string
	Limit    int
	Offset   int
	Q        string
	Style    string
	Brewery  string
	ABVMin   *float64
	ABVMax   *float64
}

type CatalogPage struct {
	Items    []CatalogItem
	Limit    int
	Offset   int
	HasMore  bool
	Currency string
}

func (c *Client) GetCatalog(ctx context.Context, q CatalogQuery) (*CatalogPage, error) {
	if q.Currency == "" {
		q.Currency = "COP"
	}
	if q.Limit <= 0 {
		q.Limit = 15
	}
	params := url.Values{}
	params.Set("currency", q.Currency)
	params.Set("limit", strconv.Itoa(q.Limit))
	params.Set("offset", strconv.Itoa(q.Offset))
	if s := strings.TrimSpace(q.Q); s != "" {
		params.Set("q", s)
	}
	if s := strings.TrimSpace(q.Style); s != "" {
		params.Set("style", s)
	}
	if s := strings.TrimSpace(q.Brewery); s != "" {
		params.Set("brewery", s)
	}
	if q.ABVMin != nil {
		params.Set("abv_min", strconv.FormatFloat(*q.ABVMin, 'f', -1, 64))
	}
	if q.ABVMax != nil {
		params.Set("abv_max", strconv.FormatFloat(*q.ABVMax, 'f', -1, 64))
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL+"/catalog?"+params.Encode(), nil)
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
		Limit    int           `json:"limit"`
		Offset   int           `json:"offset"`
		HasMore  bool          `json:"has_more"`
	}
	if err := json.Unmarshal(body, &out); err != nil {
		return nil, err
	}
	return &CatalogPage{
		Items:    out.Items,
		Limit:    out.Limit,
		Offset:   out.Offset,
		HasMore:  out.HasMore,
		Currency: out.Currency,
	}, nil
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
	BirthDate    string `json:"BirthDate"`
}

type PatchCustomerRequest struct {
	City         *string `json:"city,omitempty"`
	Neighborhood *string `json:"neighborhood,omitempty"`
	AddressLine1 *string `json:"address_line1,omitempty"`
	Phone        *string `json:"phone,omitempty"`
	LegalID      *string `json:"legal_id,omitempty"`
	FullName     *string `json:"full_name,omitempty"`
	BirthDate    *string `json:"birth_date,omitempty"`
}

type CartLine struct {
	ProductID    string `json:"product_id"`
	SKU          string `json:"sku,omitempty"`
	Name         string `json:"name,omitempty"`
	Currency     string `json:"currency,omitempty"`
	UnitPrice    int64  `json:"unit_price_minor_units,omitempty"`
	Qty          int    `json:"qty"`
	AvailableQty int    `json:"available_qty,omitempty"`
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

func (c *Client) PatchCustomerMe(ctx context.Context, patch PatchCustomerRequest) (*CustomerView, error) {
	payload, err := json.Marshal(patch)
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPatch, c.baseURL+"/customers/me", bytes.NewReader(payload))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
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

func (c *Client) GetCart(ctx context.Context) ([]CartLine, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL+"/cart", nil)
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
		Items []CartLine `json:"items"`
	}
	if err := json.Unmarshal(body, &out); err != nil {
		return nil, err
	}
	return out.Items, nil
}

func (c *Client) PutCart(ctx context.Context, items []CartLine) error {
	if items == nil {
		items = []CartLine{}
	}
	payload, err := json.Marshal(map[string]any{"items": items})
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPut, c.baseURL+"/cart", bytes.NewReader(payload))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := c.http.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode/100 != 2 {
		return parseAPIError(resp.StatusCode, body)
	}
	return nil
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

	var out struct {
		Order    OrderView     `json:"order"`
		Shipment *ShipmentView `json:"shipment"`
	}
	if err := json.Unmarshal(body, &out); err != nil {
		return nil, err
	}
	return &OrderResult{Order: out.Order, Shipment: out.Shipment}, nil
}
