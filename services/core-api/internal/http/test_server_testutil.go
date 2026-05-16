package http

import (
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
	ordersSvc := orders.NewService(
		db.New(tdb.DB),
		orders.NewSQLRepository(),
		inventory.NewSQLRepository(),
		catalog.NewSQLRepository(),
		payments.NewSQLRepository(),
	)
	return &Server{
		DB:              tdb.DB,
		Catalog:         catalog.NewSQLRepository(),
		Orders:          ordersSvc,
		Customers:       customers.NewService(tdb.DB, customers.NewSQLRepository()),
		Stripe:          stripe,
		CoreAPIHMACKeys: map[string]string{"dev": "test-secret"},
	}
}
