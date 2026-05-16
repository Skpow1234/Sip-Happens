package email

import (
	"bytes"
	"embed"
	"fmt"
	"html/template"
	"strings"
	texttemplate "text/template"
)

//go:embed templates/*
var templateFS embed.FS

type PaymentLinkData struct {
	CustomerName string
	OrderID      string
	PaymentURL   string
	Currency     string
	TotalDisplay string
	ExpiresAt    string
}

type ReceiptData struct {
	CustomerName string
	OrderID      string
	Currency     string
	TotalDisplay string
	PaidAt       string
	ItemsSummary string
}

func renderPaymentLink(data PaymentLinkData) (text, html string, err error) {
	text, err = renderTemplate("templates/payment_link.txt", data)
	if err != nil {
		return "", "", err
	}
	html, err = renderTemplate("templates/payment_link.html", data)
	return text, html, err
}

func renderReceipt(data ReceiptData) (text, html string, err error) {
	text, err = renderTemplate("templates/receipt.txt", data)
	if err != nil {
		return "", "", err
	}
	html, err = renderTemplate("templates/receipt.html", data)
	return text, html, err
}

func renderTemplate(name string, data any) (string, error) {
	raw, err := templateFS.ReadFile(name)
	if err != nil {
		return "", err
	}
	if strings.HasSuffix(name, ".html") {
		t, err := template.New(name).Parse(string(raw))
		if err != nil {
			return "", err
		}
		var buf bytes.Buffer
		if err := t.Execute(&buf, data); err != nil {
			return "", err
		}
		return buf.String(), nil
	}
	t, err := texttemplate.New(name).Parse(string(raw))
	if err != nil {
		return "", err
	}
	var buf bytes.Buffer
	if err := t.Execute(&buf, data); err != nil {
		return "", err
	}
	return buf.String(), nil
}

func formatTotal(currency string, minor int64) string {
	if currency == "" {
		currency = "COP"
	}
	return fmt.Sprintf("%s %d", currency, minor)
}
