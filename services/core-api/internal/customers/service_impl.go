package customers

import (
	"context"
	"database/sql"
	"encoding/json"
	"strings"
	"time"

	"sip-happens/pkg/age"
	"sip-happens/pkg/domain"
	"sip-happens/pkg/ids"
)

type service struct {
	db   *sql.DB
	repo Repository
}

func NewService(db *sql.DB, repo Repository) Service {
	if repo == nil {
		repo = NewSQLRepository()
	}
	return &service{db: db, repo: repo}
}

func (s *service) GetByID(ctx context.Context, id domain.CustomerID) (*domain.Customer, error) {
	if s.db == nil {
		return nil, domain.ErrInvalidInput
	}
	return s.repo.GetByID(ctx, s.db, id)
}

func (s *service) IDByFingerprint(ctx context.Context, fp string) (domain.CustomerID, error) {
	if s.db == nil {
		return "", domain.ErrInvalidInput
	}
	if strings.TrimSpace(fp) == "" {
		return "", domain.ErrInvalidInput
	}
	return s.repo.IDByFingerprint(ctx, s.db, fp)
}

func (s *service) GetOrCreateByFingerprint(ctx context.Context, fp string) (domain.CustomerID, error) {
	if s.db == nil {
		return "", domain.ErrInvalidInput
	}
	if strings.TrimSpace(fp) == "" {
		return "", domain.ErrInvalidInput
	}
	if id, err := s.repo.IDByFingerprint(ctx, s.db, fp); err == nil {
		return id, nil
	} else if err != domain.ErrNotFound {
		return "", err
	}
	uuid, err := ids.UUIDv4()
	if err != nil {
		return "", err
	}
	newID := domain.CustomerID(uuid)
	if err := s.repo.InsertByFingerprint(ctx, s.db, newID, fp); err != nil {
		return "", err
	}
	return s.repo.IDByFingerprint(ctx, s.db, fp)
}

func (s *service) Update(ctx context.Context, c *domain.Customer) error {
	if s.db == nil || c == nil {
		return domain.ErrInvalidInput
	}
	return s.repo.Update(ctx, s.db, c)
}

func (s *service) SaveCheckoutProfile(ctx context.Context, id domain.CustomerID, in CheckoutProfile) error {
	if s.db == nil {
		return nil
	}
	var birth any
	if strings.TrimSpace(in.BirthDate) != "" {
		bd, err := parseBirthDate(in.BirthDate)
		if err != nil {
			return domain.ErrInvalidInput
		}
		if !age.IsAdult(bd, time.Now()) {
			return domain.ErrInvalidInput
		}
		birth = bd
	}
	return s.repo.SaveCheckoutProfile(ctx, s.db, id, in.City, in.Neighborhood, in.AddressLine1, in.Phone, in.LegalID, in.FullName, birth)
}

func (s *service) LoadCart(ctx context.Context, customerID domain.CustomerID) ([]CartLine, error) {
	if s.db == nil {
		return nil, nil
	}
	return s.repo.LoadCart(ctx, s.db, customerID)
}

func (s *service) SaveCart(ctx context.Context, customerID domain.CustomerID, items []CartLine) error {
	if s.db == nil {
		return nil
	}
	if items == nil {
		items = []CartLine{}
	}
	b, err := json.Marshal(items)
	if err != nil {
		return err
	}
	return s.repo.SaveCart(ctx, s.db, customerID, b)
}

func (s *service) RecordConsent(ctx context.Context, customerID domain.CustomerID, purpose string) error {
	if s.db == nil || purpose == "" {
		return nil
	}
	return s.repo.RecordConsent(ctx, s.db, customerID, purpose)
}

func (s *service) StripeCustomerID(ctx context.Context, id domain.CustomerID) (string, error) {
	if s.db == nil || id == "" {
		return "", domain.ErrInvalidInput
	}
	return s.repo.StripeCustomerID(ctx, s.db, id)
}

func (s *service) SetStripeCustomerID(ctx context.Context, id domain.CustomerID, stripeCustomerID string) error {
	if s.db == nil || id == "" {
		return domain.ErrInvalidInput
	}
	return s.repo.SetStripeCustomerID(ctx, s.db, id, stripeCustomerID)
}

func parseBirthDate(s string) (time.Time, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return time.Time{}, domain.ErrInvalidInput
	}
	if t, err := time.Parse("2006-01-02", s); err == nil {
		return t, nil
	}
	return time.Time{}, domain.ErrInvalidInput
}
