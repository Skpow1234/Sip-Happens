package api

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"
)

type Client struct {
	baseURL string
	http    *http.Client
}

func NewClient(baseURL string) *Client {
	baseURL = strings.TrimRight(baseURL, "/")
	return &Client{
		baseURL: baseURL,
		http: &http.Client{
			Timeout: 10 * time.Second,
		},
	}
}

func NewSignedClient(baseURL string, hmacSecret string, sshFingerprint string) *Client {
	baseURL = strings.TrimRight(baseURL, "/")
	secret := []byte(hmacSecret)
	return &Client{
		baseURL: baseURL,
		http: &http.Client{
			Timeout: 10 * time.Second,
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
			Timeout: 10 * time.Second,
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
		ID       string
		SKU      string
		Name     string
		Brewery  string
		Style    string
		ABV      float64
		VolumeML int
		InvimaReg string
	} `json:"Product"`
	Price struct {
		Currency         string
		AmountMinorUnits int64
	} `json:"Price"`
}

func (c *Client) GetCatalog(ctx context.Context) ([]CatalogItem, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL+"/catalog", nil)
	if err != nil {
		return nil, err
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode/100 != 2 {
		return nil, fmt.Errorf("catalog http %d", resp.StatusCode)
	}

	var out struct {
		Currency string       `json:"currency"`
		Items    []CatalogItem `json:"items"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return nil, err
	}
	return out.Items, nil
}

