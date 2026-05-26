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
	ComplianceData
}

type ReceiptData struct {
	CustomerName string
	OrderID      string
	Currency     string
	TotalDisplay string
	PaidAt       string
	ItemsSummary string
	ComplianceData
}

// SubscribeConfirmData is the template payload for the double-opt-in
// confirmation email triggered by POST /email.
type SubscribeConfirmData struct {
	Email      string
	ConfirmURL string
	TTLHours   int
	ComplianceData
}

func renderPaymentLink(data PaymentLinkData) (text, html string, err error) {
	data.ComplianceData = data.ComplianceData.withDefaults()
	text, err = renderTemplate("templates/payment_link.txt", data)
	if err != nil {
		return "", "", err
	}
	html, err = renderTemplate("templates/payment_link.html", data)
	return text, html, err
}

func renderReceipt(data ReceiptData) (text, html string, err error) {
	data.ComplianceData = data.ComplianceData.withDefaults()
	text, err = renderTemplate("templates/receipt.txt", data)
	if err != nil {
		return "", "", err
	}
	html, err = renderTemplate("templates/receipt.html", data)
	return text, html, err
}

func renderSubscribeConfirm(data SubscribeConfirmData) (text, html string, err error) {
	data.ComplianceData = data.ComplianceData.withDefaults()
	text, err = renderTemplate("templates/subscribe_confirm.txt", data)
	if err != nil {
		return "", "", err
	}
	html, err = renderTemplate("templates/subscribe_confirm.html", data)
	return text, html, err
}

// renderTemplate parses the named template AND the matching footer
// partial (`templates/_footer.{html,txt}`) so every template can invoke
// `{{template "footer" .}}` without each call site re-wiring partials.
// The partial defines a single `footer` block; mixing extra partials in
// the future is fine — add them to the parse list below.
func renderTemplate(name string, data any) (string, error) {
	raw, err := templateFS.ReadFile(name)
	if err != nil {
		return "", err
	}
	if strings.HasSuffix(name, ".html") {
		footer, err := templateFS.ReadFile("templates/_footer.html")
		if err != nil {
			return "", err
		}
		t, err := template.New(name).Parse(string(raw))
		if err != nil {
			return "", err
		}
		if _, err := t.Parse(string(footer)); err != nil {
			return "", err
		}
		var buf bytes.Buffer
		if err := t.Execute(&buf, data); err != nil {
			return "", err
		}
		return buf.String(), nil
	}
	footer, err := templateFS.ReadFile("templates/_footer.txt")
	if err != nil {
		return "", err
	}
	t, err := texttemplate.New(name).Parse(string(raw))
	if err != nil {
		return "", err
	}
	if _, err := t.Parse(string(footer)); err != nil {
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
