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

// CatalogProduct mirrors core-api catalog JSON (public id + snake_case).
type CatalogProduct struct {
	ID          string  `json:"id"`
	SKU         string  `json:"sku"`
	Name        string  `json:"name"`
	Brewery     string  `json:"brewery"`
	Style       string  `json:"style"`
	ABV         float64 `json:"abv"`
	VolumeML    int     `json:"volume_ml"`
	InvimaReg   string  `json:"invima_reg"`
	Description string  `json:"description"`
	IBU         *int    `json:"ibu"`
	ImageURL    string  `json:"image_url"`
}

type catalogPrice struct {
	Currency         string `json:"currency"`
	AmountMinorUnits int64  `json:"amount_minor_units"`
}

type CatalogItem struct {
	Product      CatalogProduct `json:"product"`
	Price        catalogPrice   `json:"price"`
	AvailableQty int            `json:"available_qty"`
}

type CatalogDetail struct {
	Product      CatalogProduct `json:"product"`
	Price        catalogPrice   `json:"price"`
	AvailableQty int            `json:"available_qty"`
}

func (d CatalogDetail) AsCatalogItem() CatalogItem {
	return CatalogItem{
		Product:      d.Product,
		Price:        d.Price,
		AvailableQty: d.AvailableQty,
	}
}

type CheckoutItem struct {
	ProductID string `json:"product_id"`
	Qty       int    `json:"qty"`
}

type CheckoutRequest struct {
	Currency       string         `json:"currency"`
	City           string         `json:"city"`
	Neighborhood   string         `json:"neighborhood"`
	AddressLine1   string         `json:"address_line1"`
	Phone          string         `json:"phone"`
	LegalID        string         `json:"legal_id"`
	FullName       string         `json:"full_name"`
	BirthDate      string         `json:"birth_date,omitempty"`
	PrivacyConsent bool           `json:"privacy_consent"`
	SuccessURL     string         `json:"success_url"`
	CancelURL      string         `json:"cancel_url"`
	Items          []CheckoutItem `json:"items"`
}

type OrderView struct {
	ID                 string     `json:"ID"`
	DisplayRef         string     `json:"DisplayRef"`
	Status             string     `json:"Status"`
	SubtotalMinorUnits int64      `json:"SubtotalMinorUnits"`
	ShippingMinorUnits int64      `json:"ShippingMinorUnits"`
	TotalMinorUnits    int64      `json:"TotalMinorUnits"`
	Currency           string     `json:"Currency"`
	ExpiresAt          *time.Time `json:"ExpiresAt"`
	CreatedAt          time.Time  `json:"CreatedAt"`
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

func (c *Client) GetCatalogDetail(ctx context.Context, ref string) (*CatalogDetail, error) {
	ref = strings.TrimSpace(ref)
	if ref == "" {
		return nil, fmt.Errorf("missing product id")
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL+"/catalog/"+url.PathEscape(ref)+"?currency=COP", nil)
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
	ID           string `json:"id"`
	Email        string `json:"email"`
	City         string `json:"city"`
	Neighborhood string `json:"neighborhood"`
	AddressLine1 string `json:"address_line1"`
	Phone        string `json:"phone"`
	LegalID      string `json:"legal_id"`
	FullName     string `json:"full_name"`
	BirthDate    string `json:"birth_date"`
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

// CartView is the core-api GET/PUT /cart response body.
type CartView struct {
	Currency           string       `json:"currency"`
	SubtotalMinorUnits int64        `json:"subtotal_minor_units"`
	ItemCount          int          `json:"item_count"`
	Items              []CartItemView `json:"items"`
	AddressID          string       `json:"address_id,omitempty"`
	Address            *AddressView `json:"address,omitempty"`
}

// CartItemView is one line in cart with nested product and price.
type CartItemView struct {
	Qty                 int `json:"qty"`
	LineTotalMinorUnits int64 `json:"line_total_minor_units"`
	AvailableQty        int `json:"available_qty"`
	Product             struct {
		ID   string `json:"id"`
		SKU  string `json:"sku"`
		Name string `json:"name"`
	} `json:"product"`
	Price struct {
		Currency         string `json:"currency"`
		AmountMinorUnits int64  `json:"amount_minor_units"`
	} `json:"price"`
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

// GetProfile is a Terminal.shop alias for GET /profile.
func (c *Client) GetProfile(ctx context.Context) (*CustomerView, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL+"/profile", nil)
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
		Profile CustomerView `json:"profile"`
	}
	if err := json.Unmarshal(body, &out); err != nil {
		return nil, err
	}
	return &out.Profile, nil
}

// PutProfile is a Terminal.shop alias for PUT /profile (partial update, same as PATCH /customers/me).
func (c *Client) PutProfile(ctx context.Context, patch PatchCustomerRequest) (*CustomerView, error) {
	return c.patchProfile(ctx, http.MethodPut, patch)
}

func (c *Client) PatchCustomerMe(ctx context.Context, patch PatchCustomerRequest) (*CustomerView, error) {
	return c.patchProfileAt(ctx, http.MethodPatch, "/customers/me", patch, "customer")
}

func (c *Client) patchProfileAt(ctx context.Context, method, path string, patch PatchCustomerRequest, wrapKey string) (*CustomerView, error) {
	payload, err := json.Marshal(patch)
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequestWithContext(ctx, method, c.baseURL+path, bytes.NewReader(payload))
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
	var out map[string]json.RawMessage
	if err := json.Unmarshal(body, &out); err != nil {
		return nil, err
	}
	raw, ok := out[wrapKey]
	if !ok {
		return nil, fmt.Errorf("missing %q in response", wrapKey)
	}
	var v CustomerView
	if err := json.Unmarshal(raw, &v); err != nil {
		return nil, err
	}
	return &v, nil
}

func (c *Client) patchProfile(ctx context.Context, method string, patch PatchCustomerRequest) (*CustomerView, error) {
	return c.patchProfileAt(ctx, method, "/profile", patch, "profile")
}

// AddressView mirrors core-api saved address JSON.
type AddressView struct {
	ID           string `json:"id"`
	Label        string `json:"label"`
	FullName     string `json:"full_name"`
	Phone        string `json:"phone"`
	LegalID      string `json:"legal_id"`
	AddressLine1 string `json:"address_line1"`
	Neighborhood string `json:"neighborhood"`
	City         string `json:"city"`
	IsDefault    bool   `json:"is_default"`
}

type CreateAddressRequest struct {
	Label        string `json:"label,omitempty"`
	FullName     string `json:"full_name,omitempty"`
	Phone        string `json:"phone,omitempty"`
	LegalID      string `json:"legal_id,omitempty"`
	AddressLine1 string `json:"address_line1"`
	Neighborhood string `json:"neighborhood"`
	City         string `json:"city"`
	IsDefault    *bool  `json:"is_default,omitempty"`
}

func (c *Client) ListAddresses(ctx context.Context) ([]AddressView, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL+"/addresses", nil)
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
		Addresses []AddressView `json:"addresses"`
	}
	if err := json.Unmarshal(body, &out); err != nil {
		return nil, err
	}
	return out.Addresses, nil
}

func (c *Client) GetAddress(ctx context.Context, id string) (*AddressView, error) {
	id = strings.TrimSpace(id)
	if id == "" {
		return nil, fmt.Errorf("missing address id")
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL+"/addresses/"+url.PathEscape(id), nil)
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
		Address AddressView `json:"address"`
	}
	if err := json.Unmarshal(body, &out); err != nil {
		return nil, err
	}
	return &out.Address, nil
}

func (c *Client) CreateAddress(ctx context.Context, in CreateAddressRequest) (*AddressView, error) {
	payload, err := json.Marshal(in)
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/addresses", bytes.NewReader(payload))
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
		Address AddressView `json:"address"`
	}
	if err := json.Unmarshal(body, &out); err != nil {
		return nil, err
	}
	return &out.Address, nil
}

func (c *Client) DeleteAddress(ctx context.Context, id string) error {
	id = strings.TrimSpace(id)
	if id == "" {
		return fmt.Errorf("missing address id")
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodDelete, c.baseURL+"/addresses/"+url.PathEscape(id), nil)
	if err != nil {
		return err
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusNoContent {
		return nil
	}
	body, _ := io.ReadAll(resp.Body)
	return parseAPIError(resp.StatusCode, body)
}

// PaymentMethodCollectResult is the hosted Stripe setup URL from POST /payment-methods/collect.
type PaymentMethodCollectResult struct {
	URL               string `json:"url"`
	CheckoutSessionID string `json:"checkout_session_id"`
}

type CollectPaymentMethodRequest struct {
	SuccessURL string `json:"success_url,omitempty"`
	CancelURL  string `json:"cancel_url,omitempty"`
}

// CollectPaymentMethod starts off-terminal Stripe Checkout (setup mode) to save a card.
func (c *Client) CollectPaymentMethod(ctx context.Context, in CollectPaymentMethodRequest) (*PaymentMethodCollectResult, error) {
	payload, err := json.Marshal(in)
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/payment-methods/collect", bytes.NewReader(payload))
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
		Collect PaymentMethodCollectResult `json:"collect"`
	}
	if err := json.Unmarshal(body, &out); err != nil {
		return nil, err
	}
	if out.Collect.URL == "" {
		return nil, fmt.Errorf("missing collect url")
	}
	return &out.Collect, nil
}

// PaymentMethodCard is saved card metadata from core-api (no PAN).
type PaymentMethodCard struct {
	ID        string `json:"id"`
	Type      string `json:"type"`
	Brand     string `json:"brand,omitempty"`
	Last4     string `json:"last4,omitempty"`
	ExpMonth  int64  `json:"exp_month,omitempty"`
	ExpYear   int64  `json:"exp_year,omitempty"`
	IsDefault bool   `json:"is_default"`
}

func (c *Client) ListPaymentMethods(ctx context.Context) ([]PaymentMethodCard, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL+"/payment-methods", nil)
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
		PaymentMethods []PaymentMethodCard `json:"payment_methods"`
	}
	if err := json.Unmarshal(body, &out); err != nil {
		return nil, err
	}
	return out.PaymentMethods, nil
}

func (c *Client) GetPaymentMethod(ctx context.Context, id string) (*PaymentMethodCard, error) {
	id = strings.TrimSpace(id)
	if id == "" {
		return nil, fmt.Errorf("missing payment method id")
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL+"/payment-methods/"+url.PathEscape(id), nil)
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
		PaymentMethod PaymentMethodCard `json:"payment_method"`
	}
	if err := json.Unmarshal(body, &out); err != nil {
		return nil, err
	}
	return &out.PaymentMethod, nil
}

func (c *Client) DeletePaymentMethod(ctx context.Context, id string) error {
	id = strings.TrimSpace(id)
	if id == "" {
		return fmt.Errorf("missing payment method id")
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodDelete, c.baseURL+"/payment-methods/"+url.PathEscape(id), nil)
	if err != nil {
		return err
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusNoContent {
		return nil
	}
	body, _ := io.ReadAll(resp.Body)
	return parseAPIError(resp.StatusCode, body)
}

func (c *Client) GetCart(ctx context.Context) (*CartView, error) {
	return c.GetCartCurrency(ctx, "COP")
}

func (c *Client) GetCartCurrency(ctx context.Context, currency string) (*CartView, error) {
	u := c.baseURL + "/cart"
	if strings.TrimSpace(currency) != "" {
		u += "?currency=" + url.QueryEscape(strings.TrimSpace(currency))
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
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
		Cart CartView `json:"cart"`
	}
	if err := json.Unmarshal(body, &out); err != nil {
		return nil, err
	}
	return &out.Cart, nil
}

// PutCartItem sets quantity for one product (qty 0 removes). Returns the updated cart.
func (c *Client) PutCartItem(ctx context.Context, productID string, qty int, currency string) (*CartView, error) {
	productID = strings.TrimSpace(productID)
	if productID == "" {
		return nil, fmt.Errorf("missing product id")
	}
	if strings.TrimSpace(currency) == "" {
		currency = "COP"
	}
	payload, err := json.Marshal(map[string]int{"qty": qty})
	if err != nil {
		return nil, err
	}
	u := c.baseURL + "/cart/items/" + url.PathEscape(productID) + "?currency=" + url.QueryEscape(currency)
	req, err := http.NewRequestWithContext(ctx, http.MethodPut, u, bytes.NewReader(payload))
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
		Cart CartView `json:"cart"`
	}
	if err := json.Unmarshal(body, &out); err != nil {
		return nil, err
	}
	return &out.Cart, nil
}

// PutCartAddress sets or clears the saved delivery address on the cart.
func (c *Client) PutCartAddress(ctx context.Context, addressID string, currency string) (*CartView, error) {
	if strings.TrimSpace(currency) == "" {
		currency = "COP"
	}
	payload, err := json.Marshal(map[string]string{"address_id": addressID})
	if err != nil {
		return nil, err
	}
	u := c.baseURL + "/cart/address?currency=" + url.QueryEscape(currency)
	req, err := http.NewRequestWithContext(ctx, http.MethodPut, u, bytes.NewReader(payload))
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
		Cart CartView `json:"cart"`
	}
	if err := json.Unmarshal(body, &out); err != nil {
		return nil, err
	}
	return &out.Cart, nil
}

// CartLinesFromView flattens a CartView for TUI/state use.
func CartLinesFromView(v *CartView) []CartLine {
	if v == nil {
		return nil
	}
	out := make([]CartLine, 0, len(v.Items))
	for _, it := range v.Items {
		out = append(out, CartLine{
			ProductID:    it.Product.ID,
			SKU:          it.Product.SKU,
			Name:         it.Product.Name,
			Currency:     it.Price.Currency,
			UnitPrice:    it.Price.AmountMinorUnits,
			Qty:          it.Qty,
			AvailableQty: it.AvailableQty,
		})
	}
	return out
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

// ShippingEstimate is the JSON shape of GET /shipping/estimate.
type ShippingEstimate struct {
	City               string `json:"city"`
	Currency           string `json:"currency"`
	ShippingMinorUnits int64  `json:"shipping_minor_units"`
}

func (c *Client) GetShippingEstimate(ctx context.Context, city, currency string) (*ShippingEstimate, error) {
	if currency == "" {
		currency = "COP"
	}
	q := url.Values{}
	q.Set("city", city)
	q.Set("currency", currency)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL+"/shipping/estimate?"+q.Encode(), nil)
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
	var out ShippingEstimate
	if err := json.Unmarshal(body, &out); err != nil {
		return nil, err
	}
	return &out, nil
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
