package http

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"sip-happens/services/core-api/internal/admin"
	"sip-happens/services/core-api/internal/testutil"
)

func TestAdminAuditEvents_ByOrderID(t *testing.T) {
	ctx := context.Background()
	tdb, err := testutil.OpenMigrated(ctx)
	if err != nil {
		t.Skipf("db not available: %v", err)
	}
	defer func() { _ = tdb.CloseFn() }()

	orderID := "order-audit-filter-test"
	now := time.Now()
	adm := &admin.SQLService{DB: tdb.DB}
	if err := adm.RecordAudit(ctx, "support", "order", orderID, "order.cancelled", `{"reason":"test"}`); err != nil {
		t.Fatal(err)
	}
	if err := adm.RecordAudit(ctx, "ops", "product", "prod-x", "product.updated", `{"order_id":"`+orderID+`"}`); err != nil {
		t.Fatal(err)
	}
	_, _ = tdb.DB.ExecContext(ctx, `
insert into audit_events (aggregate_type, aggregate_id, actor, event_type, payload_json, created_at)
values ('inventory','inv-1','ops','inventory.upserted','{"note":"other"}',$1)
`, now)

	srv := &Server{DB: tdb.DB, Admin: adm, AdminToken: "test-admin"}
	ts := httptest.NewServer(srv.Handler())
	defer ts.Close()

	req, _ := http.NewRequest(http.MethodGet, ts.URL+"/admin/audit-events?order_id="+orderID, nil)
	req.Header.Set("Authorization", "Bearer test-admin")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status %d", resp.StatusCode)
	}

	var body struct {
		Items []struct {
			AggregateType string `json:"AggregateType"`
			EventType     string `json:"EventType"`
		} `json:"items"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		t.Fatal(err)
	}
	if len(body.Items) < 2 {
		t.Fatalf("expected at least 2 events for order, got %d", len(body.Items))
	}
}
