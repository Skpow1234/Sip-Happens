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
	if !strings.Contains(out, "Image:") || !strings.Contains(out, "localhost") {
		t.Fatalf("unexpected: %q", out)
	}
	if !strings.Contains(out, "beer") {
		t.Fatalf("expected ascii placeholder: %q", out)
	}
}
