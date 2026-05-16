package consumers

import "testing"

func TestJetStreamRuntime_validateNamespaceSubjects(t *testing.T) {
	r := &JetStreamRuntime{
		Namespace:     "dev",
		FilterSubject: "sip_happens.dev.events.>",
		DLQSubject:    "sip_happens.dev.dlq",
	}
	if err := r.validateNamespaceSubjects(); err != nil {
		t.Fatal(err)
	}

	r.FilterSubject = "sip_happens.staging.events.>"
	if err := r.validateNamespaceSubjects(); err == nil {
		t.Fatal("expected filter namespace error")
	}
}
