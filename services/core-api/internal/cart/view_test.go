package cart

import (
	"context"
	"testing"

	"sip-happens/services/core-api/internal/customers"
)

func TestBuildView_empty(t *testing.T) {
	v, err := BuildView(context.TODO(), nil, nil, nil, "COP", nil)
	if err != nil {
		t.Fatal(err)
	}
	if v.Currency != "COP" || v.SubtotalMinorUnits != 0 || v.ItemCount != 0 || len(v.Items) != 0 {
		t.Fatalf("empty cart: %+v", v)
	}
}

func TestBuildView_skipsLinesWhenCatalogNil(t *testing.T) {
	v, err := BuildView(context.TODO(), nil, nil, nil, "COP", []customers.CartLine{
		{ProductID: "prd_missing", Qty: 2},
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(v.Items) != 0 || v.SubtotalMinorUnits != 0 {
		t.Fatalf("expected empty view without catalog, got %+v", v)
	}
}
