package stripe

import (
	"encoding/json"
	"testing"
)

func TestReplayRaw_parsesEventID(t *testing.T) {
	raw := `{"id":"evt_test_1","type":"checkout.session.completed","data":{"object":{"id":"cs_test","metadata":{"order_id":"ord-1"}}}}`
	var evt stripeEvent
	if err := json.Unmarshal([]byte(raw), &evt); err != nil {
		t.Fatal(err)
	}
	if evt.ID != "evt_test_1" {
		t.Fatalf("id=%q", evt.ID)
	}
}
