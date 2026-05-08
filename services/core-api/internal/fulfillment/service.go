package fulfillment

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"sip-happens/pkg/domain"
	"sip-happens/pkg/ids"
)

type Service interface {
	CreateShipment(ctx context.Context, now time.Time, orderID domain.OrderID) (domain.Shipment, error)
	RunOnce(ctx context.Context, now time.Time, limit int) (int, error)
}

type SQLService struct {
	DB *sql.DB
}

func (s *SQLService) CreateShipment(ctx context.Context, now time.Time, orderID domain.OrderID) (domain.Shipment, error) {
	if s == nil || s.DB == nil {
		return domain.Shipment{}, errors.New("missing db")
	}
	if orderID == "" {
		return domain.Shipment{}, domain.ErrInvalidInput
	}
	// Idempotent: one shipment per order (v1).
	row := s.DB.QueryRowContext(ctx, `select id, order_id, carrier, tracking_code, label_path, status, created_at, updated_at from shipments where order_id=$1`, string(orderID))
	var sh domain.Shipment
	if err := row.Scan(&sh.ID, &sh.OrderID, &sh.Carrier, &sh.TrackingCode, &sh.LabelPath, &sh.Status, &sh.CreatedAt, &sh.UpdatedAt); err == nil {
		return sh, nil
	}

	uuid, err := ids.UUIDv4()
	if err != nil {
		return domain.Shipment{}, err
	}
	newID := uuid
	_, err = s.DB.ExecContext(ctx, `
insert into shipments (id, order_id, status, created_at, updated_at)
values ($1,$2,'PENDING', $3,$3)
on conflict (order_id) do nothing
`, newID, string(orderID), now)
	if err != nil {
		return domain.Shipment{}, err
	}
	// Read back canonical.
	row2 := s.DB.QueryRowContext(ctx, `select id, order_id, carrier, tracking_code, label_path, status, created_at, updated_at from shipments where order_id=$1`, string(orderID))
	if err := row2.Scan(&sh.ID, &sh.OrderID, &sh.Carrier, &sh.TrackingCode, &sh.LabelPath, &sh.Status, &sh.CreatedAt, &sh.UpdatedAt); err != nil {
		return domain.Shipment{}, err
	}
	return sh, nil
}

func (s *SQLService) RunOnce(ctx context.Context, now time.Time, limit int) (int, error) {
	if s == nil || s.DB == nil {
		return 0, errors.New("missing db")
	}
	if limit <= 0 || limit > 10000 {
		limit = 100
	}
	// Minimal skeleton: advance PENDING -> LABEL_READY with a fake label path.
	rows, err := s.DB.QueryContext(ctx, `
select id
from shipments
where status='PENDING'
order by created_at asc
limit $1
`, limit)
	if err != nil {
		return 0, err
	}
	defer rows.Close()

	ids := []string{}
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return 0, err
		}
		ids = append(ids, id)
	}
	if err := rows.Err(); err != nil {
		return 0, err
	}
	if len(ids) == 0 {
		return 0, nil
	}

	updated := 0
	for _, id := range ids {
		res, err := s.DB.ExecContext(ctx, `
update shipments
set status='LABEL_READY',
    label_path=coalesce(label_path, $2),
    updated_at=$3
where id=$1 and status='PENDING'
`, id, "/labels/"+id+".pdf", now)
		if err != nil {
			return updated, err
		}
		n, _ := res.RowsAffected()
		if n > 0 {
			updated++
		}
	}
	return updated, nil
}

