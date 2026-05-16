package events

import "testing"

func TestSubjectForEventType_ADR0016(t *testing.T) {
	subj, err := SubjectForEventType("dev", TypeOrdersOrderPaid)
	if err != nil {
		t.Fatal(err)
	}
	want := "sip_happens.dev.events.orders.order.paid"
	if subj != want {
		t.Fatalf("got %q want %q", subj, want)
	}
}

func TestParseEventTypeFromSubject(t *testing.T) {
	typ, err := ParseEventTypeFromSubject("dev", "sip_happens.dev.events.orders.order.paid")
	if err != nil {
		t.Fatal(err)
	}
	if typ != TypeOrdersOrderPaid {
		t.Fatalf("got %q", typ)
	}
}

func TestValidateEventType_rejectsInvalid(t *testing.T) {
	if err := ValidateEventType(""); err == nil {
		t.Fatal("expected error")
	}
	if err := ValidateEventType("orders..paid"); err == nil {
		t.Fatal("expected error")
	}
}
