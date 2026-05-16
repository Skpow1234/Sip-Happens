package fulfillment

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"sip-happens/pkg/domain"
	"sip-happens/pkg/ids"
)

type Service interface {
	// StartFulfillment creates a shipment (NEW) and moves a PAID order to FULFILLING (idempotent).
	StartFulfillment(ctx context.Context, now time.Time, orderID domain.OrderID) (domain.Shipment, error)
	CreateShipment(ctx context.Context, now time.Time, orderID domain.OrderID) (domain.Shipment, error)
	UpdateTracking(ctx context.Context, now time.Time, orderID domain.OrderID, carrier, trackingCode string) (domain.Shipment, error)
	RunOnce(ctx context.Context, now time.Time, limit int) (int, error)
}

type SQLService struct {
	DB *sql.DB
}

func scanShipment(row *sql.Row) (domain.Shipment, error) {
	var sh domain.Shipment
	var carrier, tracking, label sql.NullString
	if err := row.Scan(&sh.ID, &sh.OrderID, &carrier, &tracking, &label, &sh.Status, &sh.CreatedAt, &sh.UpdatedAt); err != nil {
		return domain.Shipment{}, err
	}
	sh.Carrier = carrier.String
	sh.TrackingCode = tracking.String
	sh.LabelPath = label.String
	return sh, nil
}

func (s *SQLService) CreateShipment(ctx context.Context, now time.Time, orderID domain.OrderID) (domain.Shipment, error) {
	return s.StartFulfillment(ctx, now, orderID)
}

func (s *SQLService) StartFulfillment(ctx context.Context, now time.Time, orderID domain.OrderID) (domain.Shipment, error) {
	if s == nil || s.DB == nil {
		return domain.Shipment{}, errors.New("missing db")
	}
	if orderID == "" {
		return domain.Shipment{}, domain.ErrInvalidInput
	}
	if now.IsZero() {
		now = time.Now().UTC()
	}

	tx, err := s.DB.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelSerializable})
	if err != nil {
		return domain.Shipment{}, err
	}
	defer func() { _ = tx.Rollback() }()

	var orderStatus string
	err = tx.QueryRowContext(ctx, `select status from orders where id=$1 for update`, string(orderID)).Scan(&orderStatus)
	if err != nil {
		if err == sql.ErrNoRows {
			return domain.Shipment{}, domain.ErrNotFound
		}
		return domain.Shipment{}, err
	}
	switch orderStatus {
	case string(domain.OrderStatusPaid), string(domain.OrderStatusFulfilling):
		// ok
	default:
		return domain.Shipment{}, domain.ErrConflict
	}

	sh, err := s.ensureShipmentNEW(ctx, tx, now, orderID)
	if err != nil {
		return domain.Shipment{}, err
	}

	if orderStatus == string(domain.OrderStatusPaid) {
		_, err = tx.ExecContext(ctx, `
update orders set status=$2, updated_at=$3 where id=$1 and status=$4
`, string(orderID), string(domain.OrderStatusFulfilling), now, string(domain.OrderStatusPaid))
		if err != nil {
			return domain.Shipment{}, err
		}
	}

	if err := tx.Commit(); err != nil {
		return domain.Shipment{}, err
	}
	return sh, nil
}

func (s *SQLService) ensureShipmentNEW(ctx context.Context, tx *sql.Tx, now time.Time, orderID domain.OrderID) (domain.Shipment, error) {
	row := tx.QueryRowContext(ctx, `
select id, order_id, carrier, tracking_code, label_path, status, created_at, updated_at
from shipments where order_id=$1
`, string(orderID))
	sh, scanErr := scanShipment(row)
	if scanErr == nil {
		return sh, nil
	} else if scanErr != sql.ErrNoRows {
		return domain.Shipment{}, scanErr
	}

	newID, err := ids.UUIDv4()
	if err != nil {
		return domain.Shipment{}, err
	}
	_, err = tx.ExecContext(ctx, `
insert into shipments (id, order_id, status, created_at, updated_at)
values ($1,$2,$3,$4,$4)
on conflict (order_id) do nothing
`, newID, string(orderID), string(domain.ShipmentStatusNew), now)
	if err != nil {
		return domain.Shipment{}, err
	}

	row2 := tx.QueryRowContext(ctx, `
select id, order_id, carrier, tracking_code, label_path, status, created_at, updated_at
from shipments where order_id=$1
`, string(orderID))
	return scanShipment(row2)
}

type UpdateTrackingInput struct {
	Carrier      string
	TrackingCode string
}

func (s *SQLService) UpdateTracking(ctx context.Context, now time.Time, orderID domain.OrderID, carrier, trackingCode string) (domain.Shipment, error) {
	if s == nil || s.DB == nil {
		return domain.Shipment{}, errors.New("missing db")
	}
	carrier = strings.TrimSpace(carrier)
	trackingCode = strings.TrimSpace(trackingCode)
	if orderID == "" || carrier == "" || trackingCode == "" {
		return domain.Shipment{}, domain.ErrInvalidInput
	}
	if now.IsZero() {
		now = time.Now().UTC()
	}

	tx, err := s.DB.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelSerializable})
	if err != nil {
		return domain.Shipment{}, err
	}
	defer func() { _ = tx.Rollback() }()

	row := tx.QueryRowContext(ctx, `
select id, order_id, carrier, tracking_code, label_path, status, created_at, updated_at
from shipments where order_id=$1 for update
`, string(orderID))
	sh, err := scanShipment(row)
	if err != nil {
		if err == sql.ErrNoRows {
			return domain.Shipment{}, domain.ErrNotFound
		}
		return domain.Shipment{}, err
	}

	nextStatus := sh.Status
	if sh.Status == domain.ShipmentStatusNew || sh.Status == domain.ShipmentStatusPacked {
		nextStatus = domain.ShipmentStatusHandedOver
	}

	_, err = tx.ExecContext(ctx, `
update shipments
set carrier=$2, tracking_code=$3, status=$4, updated_at=$5
where order_id=$1
`, string(orderID), carrier, trackingCode, string(nextStatus), now)
	if err != nil {
		return domain.Shipment{}, err
	}

	if nextStatus == domain.ShipmentStatusHandedOver {
		_, _ = tx.ExecContext(ctx, `
update orders set status=$2, updated_at=$3
where id=$1 and status in ($4, $5)
`, string(orderID), string(domain.OrderStatusShipped), now, string(domain.OrderStatusFulfilling), string(domain.OrderStatusPaid))
	}

	if err := tx.Commit(); err != nil {
		return domain.Shipment{}, err
	}

	row2 := s.DB.QueryRowContext(ctx, `
select id, order_id, carrier, tracking_code, label_path, status, created_at, updated_at
from shipments where order_id=$1
`, string(orderID))
	return scanShipment(row2)
}

func (s *SQLService) RunOnce(ctx context.Context, now time.Time, limit int) (int, error) {
	if s == nil || s.DB == nil {
		return 0, errors.New("missing db")
	}
	if limit <= 0 || limit > 10000 {
		limit = 100
	}
	if now.IsZero() {
		now = time.Now().UTC()
	}

	updated := 0
	n, err := s.advanceStatus(ctx, now, limit, string(domain.ShipmentStatusNew), string(domain.ShipmentStatusPacked), true)
	if err != nil {
		return updated, err
	}
	updated += n

	n, err = s.advanceStatus(ctx, now, limit, string(domain.ShipmentStatusHandedOver), string(domain.ShipmentStatusInTransit), false)
	if err != nil {
		return updated, err
	}
	updated += n

	n, err = s.advanceStatus(ctx, now, limit, string(domain.ShipmentStatusInTransit), string(domain.ShipmentStatusDelivered), false)
	if err != nil {
		return updated, err
	}
	updated += n

	return updated, nil
}

func (s *SQLService) advanceStatus(ctx context.Context, now time.Time, limit int, from, to string, setLabel bool) (int, error) {
	rows, err := s.DB.QueryContext(ctx, `
select id, order_id from shipments where status=$1 order by created_at asc limit $2
`, from, limit)
	if err != nil {
		return 0, err
	}
	defer rows.Close()

	type row struct {
		id, orderID string
	}
	var batch []row
	for rows.Next() {
		var r row
		if err := rows.Scan(&r.id, &r.orderID); err != nil {
			return 0, err
		}
		batch = append(batch, r)
	}
	if err := rows.Err(); err != nil {
		return 0, err
	}

	updated := 0
	for _, r := range batch {
		var res sql.Result
		var err error
		if setLabel {
			res, err = s.DB.ExecContext(ctx, `
update shipments set status=$3, label_path=coalesce(label_path, $2), updated_at=$4
where id=$1 and status=$5
`, r.id, fmt.Sprintf("/labels/%s.pdf", r.id), to, now, from)
		} else {
			res, err = s.DB.ExecContext(ctx, `
update shipments set status=$2, updated_at=$3 where id=$1 and status=$4
`, r.id, to, now, from)
		}
		if err != nil {
			return updated, err
		}
		n, _ := res.RowsAffected()
		if n == 0 {
			continue
		}
		updated++
		if to == string(domain.ShipmentStatusInTransit) {
			_, _ = s.DB.ExecContext(ctx, `
update orders set status=$2, updated_at=$3
where id=$1 and status in ($4, $5)
`, r.orderID, string(domain.OrderStatusShipped), now, string(domain.OrderStatusFulfilling), string(domain.OrderStatusPaid))
		}
	}
	return updated, nil
}
