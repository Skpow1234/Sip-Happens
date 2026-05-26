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
	Mailer     Mailer
	DB         *sql.DB
	From       string
	Flags      featureflags.Flags
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

// SendSubscribeConfirmation delivers the double-opt-in email for a newly
// pending newsletter subscriber. Gated by the email_subscribe_confirm
// feature flag so dev/test setups without SMTP don't try to send.
func (s *Service) SendSubscribeConfirmation(ctx context.Context, recipientEmail, confirmURL string, ttlHours int) error {
	if s == nil || s.Mailer == nil {
		return nil
	}
	if !s.Flags.Has("email_subscribe_confirm") {
		return nil
	}
	to := strings.TrimSpace(recipientEmail)
	if to == "" || strings.TrimSpace(confirmURL) == "" {
		return nil
	}
	if ttlHours <= 0 {
		ttlHours = 48
	}
	text, html, err := renderSubscribeConfirm(SubscribeConfirmData{
		Email:          to,
		ConfirmURL:     confirmURL,
		TTLHours:       ttlHours,
		ComplianceData: s.Compliance,
	})
	if err != nil {
		return err
	}
	return s.Mailer.Send(ctx, Message{
		To:       to,
		Subject:  "Confirma tu suscripción al boletín de Sip-Happens",
		TextBody: text,
		HTMLBody: html,
	})
}

// SendShippingUpdateForOrder dispatches a shipment-status email to the
// customer who owns the order. Gated by feature flag email_shipping_update.
//
// Inputs:
//   - status:  the NEW shipment status (already persisted by the caller).
//   - carrier, trackingCode: optional, only populated when known.
//   - reason:  required for FAILED, otherwise informational only.
//
// Skips silently (returns nil) when:
//   - the feature flag is off,
//   - mailer / db is missing,
//   - the customer has no email on file,
//   - the order doesn't exist or has no shipment.
//
// The per-state copy lives in `shippingUpdateCopyFor`; adding a new
// shipment state means adding a row there. FAILED uses the operator's
// `reason` directly as the description so the customer sees the real
// cause ("paquete perdido en tránsito") rather than a generic message.
func (s *Service) SendShippingUpdateForOrder(ctx context.Context, orderID domain.OrderID, status domain.ShipmentStatus, carrier, trackingCode, reason string) error {
	if s == nil || s.Mailer == nil || s.DB == nil {
		return nil
	}
	if !s.Flags.Has("email_shipping_update") {
		return nil
	}
	if orderID == "" || status == "" {
		return nil
	}
	var fullName, emailAddr sql.NullString
	err := s.DB.QueryRowContext(ctx, `
select c.full_name, c.email
from orders o join customers c on c.id = o.customer_id
where o.id = $1
`, string(orderID)).Scan(&fullName, &emailAddr)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil
		}
		return err
	}
	to := strings.TrimSpace(emailAddr.String)
	if to == "" {
		return nil
	}

	copy := shippingUpdateCopyFor(status, strings.TrimSpace(reason))
	data := ShippingUpdateData{
		CustomerName:      fullName.String,
		OrderID:           string(orderID),
		StatusLabel:       copy.Label,
		StatusDescription: copy.Description,
		Carrier:           strings.TrimSpace(carrier),
		TrackingCode:      strings.TrimSpace(trackingCode),
		TrackingURL:       trackingURLFor(carrier, trackingCode),
		SupportNote:       copy.SupportNote,
		ComplianceData:    s.Compliance,
	}
	text, html, err := renderShippingUpdate(data)
	if err != nil {
		return err
	}
	return s.Mailer.Send(ctx, Message{
		To:       to,
		Subject:  fmt.Sprintf("Actualización de envío — pedido %s (%s)", orderID, copy.ShortStatus),
		TextBody: text,
		HTMLBody: html,
	})
}

type shippingUpdateCopy struct {
	ShortStatus string
	Label       string
	Description string
	SupportNote string
}

// shippingUpdateCopyFor returns Spanish per-state copy. PACKED is the
// only one that's not necessarily customer-relevant; we still send it so
// the customer knows their order is moving (it also serves as a
// dead-man-switch — silence = ops should investigate). FAILED uses the
// operator's reason as the description, fallback to a generic message
// if reason is empty.
func shippingUpdateCopyFor(status domain.ShipmentStatus, reason string) shippingUpdateCopy {
	switch status {
	case domain.ShipmentStatusPacked:
		return shippingUpdateCopy{
			ShortStatus: "preparado",
			Label:       "Tu pedido está empacado",
			Description: "Empacamos tu pedido y está listo para ser entregado a la transportadora.",
		}
	case domain.ShipmentStatusHandedOver:
		return shippingUpdateCopy{
			ShortStatus: "en transportadora",
			Label:       "Entregamos tu pedido a la transportadora",
			Description: "La transportadora ya recogió tu pedido. Te enviaremos la guía de seguimiento apenas la registren.",
		}
	case domain.ShipmentStatusInTransit:
		return shippingUpdateCopy{
			ShortStatus: "en camino",
			Label:       "Tu pedido está en camino",
			Description: "Tu pedido salió hacia tu dirección. Sigue su trayecto con la guía a continuación.",
		}
	case domain.ShipmentStatusDelivered:
		return shippingUpdateCopy{
			ShortStatus: "entregado",
			Label:       "Tu pedido fue entregado",
			Description: "La transportadora reporta que tu pedido fue entregado. ¡Gracias por tu compra!",
			SupportNote: "Si no recibiste tu pedido, contáctanos lo antes posible para abrir una reclamación.",
		}
	case domain.ShipmentStatusFailed:
		desc := "Hubo un inconveniente con el envío de tu pedido."
		if reason != "" {
			desc = "Hubo un inconveniente con el envío de tu pedido: " + reason
		}
		return shippingUpdateCopy{
			ShortStatus: "incidente",
			Label:       "Hubo un problema con tu envío",
			Description: desc,
			SupportNote: "Nuestro equipo te contactará en las próximas horas para coordinar reenvío o reembolso. También puedes escribirnos al correo del pie de página.",
		}
	default:
		return shippingUpdateCopy{
			ShortStatus: string(status),
			Label:       "Actualización de envío",
			Description: "El estado de tu envío cambió.",
		}
	}
}

// trackingURLFor returns the carrier's public tracking URL for the given
// code when we know the format, otherwise empty. Today we only ship
// inside Colombia via Servientrega / 4-72 / TCC; extend the switch as
// you add more carriers. Keeping the URL logic here (rather than in the
// template) means a typo'd carrier doesn't render a broken link.
func trackingURLFor(carrier, code string) string {
	c := strings.ToLower(strings.TrimSpace(carrier))
	code = strings.TrimSpace(code)
	if c == "" || code == "" {
		return ""
	}
	switch c {
	case "servientrega":
		return "https://www.servientrega.com/wps/portal/rastreo-envio?guia=" + code
	case "472", "4-72", "cuatro setenta y dos":
		return "https://www.4-72.com.co/rastreo?guia=" + code
	case "tcc":
		return "https://tcc.com.co/rastreo?guia=" + code
	default:
		return ""
	}
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
