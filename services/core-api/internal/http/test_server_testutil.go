package http

import (
	"sip-happens/services/core-api/internal/admin"
	"sip-happens/services/core-api/internal/catalog"
	"sip-happens/services/core-api/internal/customers"
	"sip-happens/services/core-api/internal/db"
	"sip-happens/services/core-api/internal/inventory"
	"sip-happens/services/core-api/internal/orders"
	"sip-happens/services/core-api/internal/payments"
	stripepayments "sip-happens/services/core-api/internal/payments/stripe"
	"sip-happens/services/core-api/internal/testutil"
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
		Stripe:          stripe,
		CoreAPIHMACKeys: map[string]string{"dev": "test-secret"},
	}
}
