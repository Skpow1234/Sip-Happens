package http

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestAdminConsole(t *testing.T) {
	srv := &Server{AdminToken: "test-admin"}
	ts := httptest.NewServer(srv.Handler())
	defer ts.Close()

	t.Run("unauthorized", func(t *testing.T) {
		resp, err := http.Get(ts.URL + "/admin/console")
		if err != nil {
			t.Fatal(err)
		}
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusUnauthorized {
			t.Fatalf("expected 401, got %d", resp.StatusCode)
		}
	})

	t.Run("ok", func(t *testing.T) {
		req, _ := http.NewRequest(http.MethodGet, ts.URL+"/admin/console", nil)
		req.Header.Set("Authorization", "Bearer test-admin")
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("expected 200, got %d", resp.StatusCode)
		}
		if ct := resp.Header.Get("Content-Type"); !strings.Contains(ct, "text/html") {
			t.Fatalf("content-type: %s", ct)
		}
	})
}
