package httpapi

import (
	"archive/zip"
	"bytes"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"labspectra/internal/service"
)

func newTestHandler(t *testing.T) http.Handler {
	t.Helper()
	svc, err := service.New(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { svc.Close() })
	srv, err := New(svc)
	if err != nil {
		t.Fatal(err)
	}
	return srv.Handler()
}

func TestGuardAllowsSameOriginAnyHost(t *testing.T) {
	h := newTestHandler(t)
	req := httptest.NewRequest("GET", "http://my-app.up.railway.app/api/meta", nil)
	req.Host = "my-app.up.railway.app"
	req.Header.Set("Origin", "http://my-app.up.railway.app")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Errorf("свой Origin на публичном хосте: код %d, ожидался 200", rec.Code)
	}
}

func TestGuardBlocksCrossOrigin(t *testing.T) {
	h := newTestHandler(t)
	req := httptest.NewRequest("POST", "http://127.0.0.1:8765/api/backup", nil)
	req.Host = "127.0.0.1:8765"
	req.Header.Set("Origin", "http://evil.example.com")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusForbidden {
		t.Errorf("кросс-ориджин: код %d, ожидался 403", rec.Code)
	}
}

func TestGuardAllowsLocal(t *testing.T) {
	h := newTestHandler(t)
	req := httptest.NewRequest("GET", "http://127.0.0.1:8765/api/meta", nil)
	req.Host = "127.0.0.1:8765"
	req.Header.Set("Origin", "http://127.0.0.1:8765")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Errorf("локальный запрос: код %d, ожидался 200", rec.Code)
	}
	if rec.Header().Get("Content-Security-Policy") == "" {
		t.Error("отсутствует заголовок Content-Security-Policy")
	}
}

func TestMaintenanceEndpointsRequireAdmin(t *testing.T) {
	h := newTestHandler(t)

	for _, tc := range []struct {
		method string
		path   string
		body   string
	}{
		{http.MethodPost, "/api/backup", ""},
		{http.MethodPost, "/api/registry/rebuild", ""},
	} {
		req := httptest.NewRequest(tc.method, "http://127.0.0.1:8765"+tc.path, strings.NewReader(tc.body))
		req.Host = "127.0.0.1:8765"
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)
		if rec.Code != http.StatusUnauthorized {
			t.Errorf("%s без админа: код %d, ожидался 401", tc.path, rec.Code)
		}
	}
}

func TestBackupEndpointDownloadsZip(t *testing.T) {
	root := t.TempDir()
	svc, err := service.New(root)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { svc.Close() })

	a, err := svc.Create(service.CreateInput{Product: "R2531"})
	if err != nil {
		t.Fatal(err)
	}
	photoPath := filepath.Join(root, "samples", a.ID, "photos", "photo_1.jpg")
	if err := os.WriteFile(photoPath, []byte("photo"), 0o644); err != nil {
		t.Fatal(err)
	}
	reportDir := filepath.Join(root, "samples", a.ID, "reports")
	if err := os.MkdirAll(reportDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(reportDir, "report.txt"), []byte("report"), 0o644); err != nil {
		t.Fatal(err)
	}

	srv, err := New(svc)
	if err != nil {
		t.Fatal(err)
	}
	req := httptest.NewRequest(http.MethodPost, "http://127.0.0.1:8765/api/backup", nil)
	req.Host = "127.0.0.1:8765"
	req.Header.Set("X-Admin-Password", "123")
	rec := httptest.NewRecorder()
	srv.Handler().ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("backup status = %d, want 200: %s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Header().Get("Content-Type"), "application/zip") {
		t.Fatalf("content-type = %q, want zip", rec.Header().Get("Content-Type"))
	}
	if !strings.Contains(rec.Header().Get("Content-Disposition"), ".zip") {
		t.Fatalf("content-disposition = %q, want zip filename", rec.Header().Get("Content-Disposition"))
	}

	zr, err := zip.NewReader(bytes.NewReader(rec.Body.Bytes()), int64(rec.Body.Len()))
	if err != nil {
		t.Fatalf("invalid zip response: %v", err)
	}
	got := make(map[string]bool)
	for _, f := range zr.File {
		got[f.Name] = true
	}
	for _, name := range []string{
		"registry.xlsx",
		"samples/" + a.ID + "/card.json",
		"samples/" + a.ID + "/photos/photo_1.jpg",
		"samples/" + a.ID + "/reports/report.txt",
	} {
		if !got[name] {
			t.Errorf("zip missing %s", name)
		}
	}
}
