package http

import (
	"net/http/httptest"
	"testing"
)

func TestParseCatalogLimitQuery_Default(t *testing.T) {
	r := httptest.NewRequest("GET", "/catalog?currency=COP", nil)
	n, err := parseCatalogLimitQuery(r)
	if err != nil {
		t.Fatal(err)
	}
	if n != catalogDefaultLimit {
		t.Fatalf("got %d want %d", n, catalogDefaultLimit)
	}
}

func TestParseCatalogLimitQuery_MaxEnforced(t *testing.T) {
	r := httptest.NewRequest("GET", "/catalog?limit=500", nil)
	_, err := parseCatalogLimitQuery(r)
	if err == nil {
		t.Fatal("expected error for limit > max")
	}
}
