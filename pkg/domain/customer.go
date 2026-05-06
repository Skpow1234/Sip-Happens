package domain

import "time"

type CustomerID string

type Customer struct {
	ID             CustomerID
	SSHFingerprint string

	Email   string
	Phone   string
	LegalID string

	FullName     string
	BirthDate    time.Time
	AddressLine1 string
	Neighborhood string
	City         string

	CreatedAt time.Time
	UpdatedAt time.Time
}

type CustomerConsent struct {
	CustomerID CustomerID
	Purpose    string
	GrantedAt  time.Time
	RevokedAt  *time.Time
}

type AgeVerification struct {
	CustomerID CustomerID
	BirthDate  time.Time
	VerifiedAt time.Time
	Source     string
}

