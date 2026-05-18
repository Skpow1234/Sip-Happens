package http

import (
	"fmt"
	"sort"
	"strings"

	"go.yaml.in/yaml/v2"
)

// RouteOp is a single HTTP operation on core-api.
type RouteOp struct {
	Method string
	Path   string
}

// ImplementedRoutes is the authoritative route list for contract drift checks.
// docs/openapi.yaml must describe exactly these operations (methods + paths).
var ImplementedRoutes = []RouteOp{
	{Method: "GET", Path: "/healthz"},
	{Method: "GET", Path: "/readyz"},
	{Method: "GET", Path: "/metrics"},
	{Method: "GET", Path: "/catalog"},
	{Method: "GET", Path: "/catalog/{id}"},
	{Method: "GET", Path: "/customers/me"},
	{Method: "PATCH", Path: "/customers/me"},
	{Method: "GET", Path: "/profile"},
	{Method: "PUT", Path: "/profile"},
	{Method: "PATCH", Path: "/profile"},
	{Method: "GET", Path: "/addresses"},
	{Method: "POST", Path: "/addresses"},
	{Method: "GET", Path: "/addresses/{id}"},
	{Method: "DELETE", Path: "/addresses/{id}"},
	{Method: "GET", Path: "/cart"},
	{Method: "PUT", Path: "/cart"},
	{Method: "GET", Path: "/shipping/estimate"},
	{Method: "GET", Path: "/orders"},
	{Method: "GET", Path: "/orders/{id}"},
	{Method: "POST", Path: "/checkout"},
	{Method: "GET", Path: "/admin/products"},
	{Method: "POST", Path: "/admin/products"},
	{Method: "PUT", Path: "/admin/products/{id}"},
	{Method: "PATCH", Path: "/admin/products/{id}"},
	{Method: "DELETE", Path: "/admin/products/{id}"},
	{Method: "POST", Path: "/admin/prices"},
	{Method: "PUT", Path: "/admin/prices"},
	{Method: "DELETE", Path: "/admin/prices"},
	{Method: "GET", Path: "/admin/inventory"},
	{Method: "POST", Path: "/admin/inventory"},
	{Method: "PUT", Path: "/admin/inventory"},
	{Method: "GET", Path: "/admin/inventory/reservations"},
	{Method: "GET", Path: "/admin/orders"},
	{Method: "GET", Path: "/admin/orders/export"},
	{Method: "GET", Path: "/admin/orders/{id}"},
	{Method: "POST", Path: "/admin/orders/{id}/cancel"},
	{Method: "POST", Path: "/admin/orders/{id}/reconcile-stripe"},
	{Method: "POST", Path: "/admin/orders/{id}/invoice"},
	{Method: "PATCH", Path: "/admin/orders/{id}/shipment"},
	{Method: "GET", Path: "/admin/audit-events"},
	{Method: "GET", Path: "/admin/stripe/dead-letters"},
	{Method: "POST", Path: "/admin/stripe/dead-letters/replay"},
	{Method: "POST", Path: "/webhooks/stripe"},
}

var openAPIHTTPMethods = map[string]struct{}{
	"get": {}, "post": {}, "put": {}, "patch": {}, "delete": {},
	"head": {}, "options": {}, "trace": {},
}

type openAPIDoc struct {
	Paths map[string]map[string]any `yaml:"paths"`
}

// ParseOpenAPIOperations extracts (METHOD, path) pairs from an OpenAPI 3 document.
func ParseOpenAPIOperations(specYAML []byte) ([]RouteOp, error) {
	var doc openAPIDoc
	if err := yaml.Unmarshal(specYAML, &doc); err != nil {
		return nil, fmt.Errorf("parse openapi yaml: %w", err)
	}
	if doc.Paths == nil {
		return nil, fmt.Errorf("openapi: missing paths")
	}
	var out []RouteOp
	for path, item := range doc.Paths {
		for key := range item {
			if _, ok := openAPIHTTPMethods[strings.ToLower(key)]; !ok {
				continue
			}
			out = append(out, RouteOp{
				Method: strings.ToUpper(key),
				Path:   path,
			})
		}
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Path != out[j].Path {
			return out[i].Path < out[j].Path
		}
		return out[i].Method < out[j].Method
	})
	return out, nil
}

func routeOpKey(op RouteOp) string {
	return op.Method + " " + op.Path
}

// DiffRouteOps returns operations in a but not in b (as "METHOD path" strings).
func DiffRouteOps(a, b []RouteOp) []string {
	setB := make(map[string]struct{}, len(b))
	for _, op := range b {
		setB[routeOpKey(op)] = struct{}{}
	}
	var missing []string
	for _, op := range a {
		k := routeOpKey(op)
		if _, ok := setB[k]; !ok {
			missing = append(missing, k)
		}
	}
	sort.Strings(missing)
	return missing
}
