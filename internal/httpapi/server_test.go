package httpapi

import (
	"net/http"
	"net/http/httptest"
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
