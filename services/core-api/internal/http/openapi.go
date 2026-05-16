package http

import (
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"sync"
)

const redocHTML = `<!DOCTYPE html>
<html lang="en">
<head>
  <meta charset="utf-8"/>
  <meta name="viewport" content="width=device-width, initial-scale=1"/>
  <title>Sip-Happens API</title>
  <link href="https://cdn.redoc.ly/redoc/latest/bundle.css" rel="stylesheet"/>
</head>
<body>
  <redoc spec-url="/openapi.yaml"></redoc>
  <script src="https://cdn.redoc.ly/redoc/latest/bundle.js"></script>
</body>
</html>
`

var (
	openAPISpecOnce sync.Once
	openAPISpec     []byte
)

func loadOpenAPISpec() []byte {
	openAPISpecOnce.Do(func() {
		candidates := []string{os.Getenv("OPENAPI_PATH")}
		if _, file, _, ok := runtime.Caller(0); ok {
			root := filepath.Clean(filepath.Join(filepath.Dir(file), "..", "..", "..", ".."))
			candidates = append(candidates, filepath.Join(root, "docs", "openapi.yaml"))
		}
		candidates = append(candidates, "/app/docs/openapi.yaml", "docs/openapi.yaml")
		for _, p := range candidates {
			if p == "" {
				continue
			}
			b, err := os.ReadFile(p)
			if err == nil && len(b) > 0 {
				openAPISpec = b
				return
			}
		}
	})
	return openAPISpec
}

func (s *Server) handleOpenAPISpec(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	spec := loadOpenAPISpec()
	if len(spec) == 0 {
		writeAPIError(w, r, http.StatusNotFound, "not_found", "openapi spec not available")
		return
	}
	w.Header().Set("Content-Type", "application/yaml; charset=utf-8")
	w.Header().Set("Cache-Control", "public, max-age=300")
	_, _ = w.Write(spec)
}

func (s *Server) handleOpenAPIDocs(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	if !s.requireAdminRole(w, r, AdminRoleFinance) {
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	_, _ = w.Write([]byte(redocHTML))
}
