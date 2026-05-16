package admin

import (
	"bytes"
	"encoding/csv"
	"testing"
	"time"

	"sip-happens/pkg/domain"
)

func TestWriteOrdersCSV_HeaderAndRow(t *testing.T) {
	var buf bytes.Buffer
	created := time.Date(2026, 5, 16, 12, 0, 0, 0, time.UTC)
	orders := []domain.Order{{
		ID:                 "ord-1",
		DisplayRef:         "SH-ABC123",
		Status:             domain.OrderStatusPaid,
		CustomerID:         "cust-1",
		FullName:           "Demo User",
		City:               "Bogotá",
		Currency:           "COP",
		SubtotalMinorUnits: 10000,
		TotalMinorUnits:    12000,
		CreatedAt:          created,
		UpdatedAt:          created,
	}}
	if err := WriteOrdersCSV(&buf, orders); err != nil {
		t.Fatal(err)
	}
	rows, err := csv.NewReader(&buf).ReadAll()
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 2 {
		t.Fatalf("expected header+1 row, got %d", len(rows))
	}
	if rows[0][0] != "order_id" || rows[1][0] != "ord-1" || rows[1][1] != "SH-ABC123" {
		t.Fatalf("unexpected csv: %#v", rows)
	}
}
