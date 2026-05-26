package tui

import (
	"strings"
	"testing"
)

func TestFormatProductImageBlockEmpty(t *testing.T) {
	if formatProductImageBlock("") != "" {
		t.Fatal("expected empty")
	}
}

func TestFormatProductImageBlockShowsLink(t *testing.T) {
	out := formatProductImageBlock("http://localhost:9000/product-images/products/x/demo.jpg")
	// Should surface the localized "Imagen:" label and the URL itself, but
	// MUST NOT print the old ASCII `+----+ | beer |` placeholder — that
	// was the very "broken placeholder" this polish step removed.
	if !strings.Contains(out, "Imagen:") || !strings.Contains(out, "localhost") {
		t.Fatalf("expected localized label + URL, got %q", out)
	}
	if strings.Contains(out, "beer") || strings.Contains(out, "+------+") {
		t.Fatalf("legacy ASCII placeholder leaked back in: %q", out)
	}
}

func TestFormatProductImageBlockTruncatesLongURL(t *testing.T) {
	long := "https://example.com/" + strings.Repeat("a", 200)
	out := formatProductImageBlock(long)
	if !strings.Contains(out, "...") {
		t.Fatalf("expected truncation marker, got %q", out)
	}
}
