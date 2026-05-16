package customers

import (
	"context"
	"strings"
	"time"

	"sip-happens/pkg/age"
	"sip-happens/pkg/domain"
	"sip-happens/pkg/ids"
)

// RecordAgeVerification stores checkout age affirmation evidence (DESIGN §5.1).
func (s *service) RecordAgeVerification(ctx context.Context, customerID domain.CustomerID, orderID domain.OrderID, birthDate, method string) error {
	if s.db == nil {
		return domain.ErrInvalidInput
	}
	if customerID == "" || orderID == "" {
		return domain.ErrInvalidInput
	}
	method = strings.TrimSpace(method)
	if method == "" {
		method = "checkout"
	}
	bd, err := parseBirthDate(birthDate)
	if err != nil {
		return domain.ErrInvalidInput
	}
	if !age.IsAdult(bd, time.Now()) {
		return domain.ErrInvalidInput
	}
	id, err := ids.UUIDv4()
	if err != nil {
		return err
	}
	return s.repo.InsertAgeVerification(ctx, s.db, id, customerID, orderID, bd, time.Now().UTC(), method)
}
