package events

import (
	"fmt"
	"strings"
)

// JetStream subject layout (ADR 0016): sip_happens.{namespace}.events.{event_type}
// DLQ: sip_happens.{namespace}.dlq
const DomainPrefix = "sip_happens"

func normalizeNamespace(namespace string) string {
	ns := strings.TrimSpace(namespace)
	if ns == "" {
		return "dev"
	}
	return ns
}

// EventsSubjectPrefix returns sip_happens.{namespace}.events (no trailing dot).
func EventsSubjectPrefix(namespace string) string {
	return fmt.Sprintf("%s.%s.events", DomainPrefix, normalizeNamespace(namespace))
}

// EventsFilter returns sip_happens.{namespace}.events.>
func EventsFilter(namespace string) string {
	return EventsSubjectPrefix(namespace) + ".>"
}

// DLQSubject returns sip_happens.{namespace}.dlq
func DLQSubject(namespace string) string {
	return fmt.Sprintf("%s.%s.dlq", DomainPrefix, normalizeNamespace(namespace))
}

// SubjectForEventType returns the full JetStream subject for an event type.
func SubjectForEventType(namespace, eventType string) (string, error) {
	if err := ValidateEventType(eventType); err != nil {
		return "", err
	}
	return EventsSubjectPrefix(namespace) + "." + eventType, nil
}

// ValidateEventType enforces ADR event type naming (dot-separated segments).
func ValidateEventType(eventType string) error {
	eventType = strings.TrimSpace(eventType)
	if eventType == "" {
		return fmt.Errorf("empty event type")
	}
	if strings.Contains(eventType, " ") || strings.Contains(eventType, "/") {
		return fmt.Errorf("invalid event type %q", eventType)
	}
	for _, part := range strings.Split(eventType, ".") {
		if part == "" {
			return fmt.Errorf("invalid event type %q", eventType)
		}
	}
	return nil
}

// ParseEventTypeFromSubject extracts the event type suffix from a subject.
func ParseEventTypeFromSubject(namespace, subject string) (string, error) {
	prefix := EventsSubjectPrefix(namespace) + "."
	if !strings.HasPrefix(subject, prefix) {
		return "", fmt.Errorf("subject %q does not match prefix %q", subject, prefix)
	}
	typ := strings.TrimPrefix(subject, prefix)
	if err := ValidateEventType(typ); err != nil {
		return "", err
	}
	return typ, nil
}

// ValidateEventSubject ensures the subject matches namespace + event type.
func ValidateEventSubject(namespace, subject, eventType string) error {
	want, err := SubjectForEventType(namespace, eventType)
	if err != nil {
		return err
	}
	if subject != want {
		return fmt.Errorf("subject mismatch: got %q want %q", subject, want)
	}
	return nil
}
