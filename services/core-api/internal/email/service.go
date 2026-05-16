package email

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"

	"sip-happens/pkg/domain"
	"sip-happens/pkg/featureflags"
)

// Service sends transactional emails when feature flags allow.
type Service struct {
	Mailer Mailer
	DB     *sql.DB
	From   string
	Flags  featureflags.Flags
	Compliance ComplianceData
}

func NewService(mailer Mailer, db *sql.DB, flags featureflags.Flags) (*Service, error) {
	if mailer == nil {
		return nil, fmt.Errorf("missing mailer")
	}
	cfg, err := LoadConfigFromEnv()
	if err != nil {
		return nil, err
	}
	return &Service{
		Mailer:     mailer,
		DB:         db,
		From:       cfg.From,
		Flags:      flags,
		Compliance: defaultCompliance(),
	}, nil
}

// SetCompliance configures legal footer URLs for outbound templates.
func (s *Service) SetCompliance(c ComplianceData) {
	if s == nil {
		return
	}
	s.Compliance = c.withDefaults()
}

func (s *Service) SendPaymentLink(ctx context.Context, order *domain.Order, customerEmail, customerName, paymentURL string) error {
	if s == nil || s.Mailer == nil {
		return nil
	}
	if !s.Flags.Has("email_payment_link") {
		return nil
	}
	to := strings.TrimSpace(customerEmail)
	if to == "" || strings.TrimSpace(paymentURL) == "" {
		return nil
	}
	expires := ""
	if order != nil && order.ExpiresAt != nil {
		expires = order.ExpiresAt.UTC().Format(time.RFC3339)
	}
	total := int64(0)
	currency := "COP"
	orderID := ""
	if order != nil {
		total = order.TotalMinorUnits
		currency = order.Currency
		orderID = string(order.ID)
	}
	text, html, err := renderPaymentLink(PaymentLinkData{
		CustomerName:   customerName,
		OrderID:        orderID,
		PaymentURL:     paymentURL,
		Currency:       currency,
		TotalDisplay:   formatTotal(currency, total),
		ExpiresAt:      expires,
		ComplianceData: s.Compliance,
	})
	if err != nil {
		return err
	}
	return s.Mailer.Send(ctx, Message{
		To:       to,
		Subject:  fmt.Sprintf("Completa tu pago — pedido %s", orderID),
		TextBody: text,
		HTMLBody: html,
	})
}

func (s *Service) SendReceiptForOrder(ctx context.Context, orderID domain.OrderID) error {
	if s == nil || s.Mailer == nil || s.DB == nil {
		return nil
	}
	if !s.Flags.Has("email_receipt") {
		return nil
	}
	if orderID == "" {
		return nil
	}

	var customerID, currency string
	var total int64
	var fullName, email sql.NullString
	var updatedAt time.Time
	err := s.DB.QueryRowContext(ctx, `
select o.customer_id, o.currency, o.total_minor_units, o.updated_at,
       c.full_name, c.email
from orders o
join customers c on c.id = o.customer_id
where o.id = $1 and o.status in ('PAID','FULFILLING','SHIPPED')
`, string(orderID)).Scan(&customerID, &currency, &total, &updatedAt, &fullName, &email)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil
		}
		return err
	}
	to := strings.TrimSpace(email.String)
	if to == "" {
		return nil
	}

	rows, err := s.DB.QueryContext(ctx, `
select name, qty, line_total_minor_units from order_items where order_id=$1 order by name
`, string(orderID))
	if err != nil {
		return err
	}
	defer rows.Close()
	var lines []string
	for rows.Next() {
		var name string
		var qty int
		var line int64
		if err := rows.Scan(&name, &qty, &line); err != nil {
			return err
		}
		lines = append(lines, fmt.Sprintf("- %s x%d (%s %d)", name, qty, currency, line))
	}
	if err := rows.Err(); err != nil {
		return err
	}
	itemsSummary := strings.Join(lines, "\n")
	if itemsSummary == "" {
		itemsSummary = "(sin detalle de ítems)"
	}

	text, html, err := renderReceipt(ReceiptData{
		CustomerName:   fullName.String,
		OrderID:        string(orderID),
		Currency:       currency,
		TotalDisplay:   formatTotal(currency, total),
		PaidAt:         updatedAt.UTC().Format(time.RFC3339),
		ItemsSummary:   itemsSummary,
		ComplianceData: s.Compliance,
	})
	if err != nil {
		return err
	}
	return s.Mailer.Send(ctx, Message{
		To:       to,
		Subject:  fmt.Sprintf("Recibo de pago — pedido %s", orderID),
		TextBody: text,
		HTMLBody: html,
	})
}
