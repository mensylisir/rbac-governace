package app

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
)

func TestServerRoutes(t *testing.T) {
	dist := t.TempDir()
	t.Setenv("FRONTEND_DIST", dist)
	if err := os.MkdirAll(filepath.Join(dist, "assets"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dist, "index.html"), []byte("<html></html>"), 0o644); err != nil {
		t.Fatal(err)
	}
	server := NewServer()
	routes := server.Routes()

	for _, tc := range []struct {
		path string
		want int
	}{
		{path: "/api/health", want: http.StatusOK},
		{path: "/api/templates", want: http.StatusOK},
		{path: "/", want: http.StatusOK},
	} {
		req := httptest.NewRequest(http.MethodGet, tc.path, nil)
		rec := httptest.NewRecorder()
		routes.ServeHTTP(rec, req)
		if rec.Code != tc.want {
			t.Fatalf("%s returned %d, want %d", tc.path, rec.Code, tc.want)
		}
	}
}
