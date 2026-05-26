package http

import (
	"sip-happens/services/core-api/internal/addresses"
	"sip-happens/services/core-api/internal/admin"
	"sip-happens/services/core-api/internal/catalog"
	"sip-happens/services/core-api/internal/customers"
	"sip-happens/services/core-api/internal/db"
	"sip-happens/services/core-api/internal/inventory"
	"sip-happens/services/core-api/internal/orders"
	"sip-happens/services/core-api/internal/payments"
	stripepayments "sip-happens/services/core-api/internal/payments/stripe"
	"sip-happens/services/core-api/internal/oauthapps"
	"sip-happens/services/core-api/internal/subscriptions"
	"sip-happens/services/core-api/internal/testutil"
	"sip-happens/services/core-api/internal/tokens"
)

func testServer(tdb *testutil.TestDB, stripe stripepayments.Service) *Server {
	if stripe == nil {
		stripe = stripepayments.MockClient{}
	}
	paymentsRepo := payments.NewSQLRepository()
	ordersSvc := orders.NewService(
		db.New(tdb.DB),
		orders.NewSQLRepository(),
		inventory.NewSQLRepository(),
		catalog.NewSQLRepository(),
		paymentsRepo,
	)
	return &Server{
		DB:              tdb.DB,
		Catalog:         catalog.NewSQLRepository(),
		Orders:          ordersSvc,
		Payments:        paymentsRepo,
		Admin:           &admin.SQLService{DB: tdb.DB},
		AdminToken:      "test-admin",
		Customers:       customers.NewService(tdb.DB, customers.NewSQLRepository()),
		Addresses:       addresses.NewSQLRepository(),
		Subscriptions:   subscriptions.NewSQLRepository(),
		Tokens:          tokens.NewSQLRepository(),
		OAuthClients:    oauthapps.NewSQLClientRepository(),
		OAuthCodes:      oauthapps.NewSQLCodeRepository(),
		Stripe:          stripe,
		CoreAPIHMACKeys: map[string]string{"dev": "test-secret"},
	}
}
