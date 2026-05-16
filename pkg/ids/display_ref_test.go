package ids

import (
	"strings"
	"testing"
)

func TestNewOrderDisplayRef_Format(t *testing.T) {
	ref, err := NewOrderDisplayRef()
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(ref, "SH-") {
		t.Fatalf("expected SH- prefix, got %q", ref)
	}
	body := strings.TrimPrefix(ref, "SH-")
	if len(body) != 8 {
		t.Fatalf("expected 8 char body, got %q", body)
	}
	for _, r := range body {
		if !strings.ContainsRune(crockfordBase32, r) {
			t.Fatalf("invalid char %q in %q", r, ref)
		}
	}
}
