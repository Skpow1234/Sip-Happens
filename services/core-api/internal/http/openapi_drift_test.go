package http

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// repoOpenAPIPath reads docs/openapi.yaml from the repo root for drift
// comparison. The spec file is excluded by `.gitignore` so it is absent
// on fresh CI checkouts; in that case we skip the test rather than fail,
// matching the convention `scripts/check-migration-linearity.sh` uses
// for the migrations dir. Local devs (who have the file) still get the
// strict comparison.
func repoOpenAPIPath(t *testing.T) []byte {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller failed")
	}
	// services/core-api/internal/http -> repo root (4 levels up)
	root := filepath.Clean(filepath.Join(filepath.Dir(file), "..", "..", "..", ".."))
	p := filepath.Join(root, "docs", "openapi.yaml")
	b, err := os.ReadFile(p)
	if err != nil {
		if os.IsNotExist(err) {
			t.Skipf("docs/openapi.yaml not present (likely a CI checkout where docs/ is gitignored); skipping drift check")
			return nil
		}
		t.Fatalf("read %s: %v", p, err)
	}
	return b
}

func TestOpenAPIDrift_ImplementedRoutesMatchSpec(t *testing.T) {
	spec := repoOpenAPIPath(t)
	if spec == nil {
		return // skipped inside helper
	}
	fromSpec, err := ParseOpenAPIOperations(spec)
	if err != nil {
		t.Fatal(err)
	}

	if missing := DiffRouteOps(ImplementedRoutes, fromSpec); len(missing) > 0 {
		t.Fatalf("handlers implemented but missing from openapi.yaml:\n  %s", strings.Join(missing, "\n  "))
	}
	if extra := DiffRouteOps(fromSpec, ImplementedRoutes); len(extra) > 0 {
		t.Fatalf("openapi.yaml documents routes not in ImplementedRoutes:\n  %s", strings.Join(extra, "\n  "))
	}
}

func TestOpenAPIDrift_LoadedSpecMatchesFile(t *testing.T) {
	spec := repoOpenAPIPath(t)
	if spec == nil {
		return // skipped inside helper
	}
	fromFile, err := ParseOpenAPIOperations(spec)
	if err != nil {
		t.Fatal(err)
	}
	loaded := loadOpenAPISpec()
	if len(loaded) == 0 {
		t.Fatal("loadOpenAPISpec returned empty")
	}
	fromLoad, err := ParseOpenAPIOperations(loaded)
	if err != nil {
		t.Fatal(err)
	}
	if missing := DiffRouteOps(fromFile, fromLoad); len(missing) > 0 {
		t.Fatalf("docs/openapi.yaml differs from loaded spec (missing in load): %v", missing)
	}
	if extra := DiffRouteOps(fromLoad, fromFile); len(extra) > 0 {
		t.Fatalf("loaded spec differs from docs/openapi.yaml (extra in load): %v", extra)
	}
}
