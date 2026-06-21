package httpapi

import (
	"io/fs"
	"net/http"
	"net/url"

	"labspectra/internal/service"
	"labspectra/internal/web"
)

type Server struct {
	svc *service.Service
	mux *http.ServeMux
}

func New(svc *service.Service) (*Server, error) {
	s := &Server{svc: svc, mux: http.NewServeMux()}
	if err := s.routes(); err != nil {
		return nil, err
	}
	return s, nil
}

func (s *Server) Handler() http.Handler { return guard(s.mux) }

func guard(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if o := r.Header.Get("Origin"); o != "" && !sameOrigin(o, r.Host) {
			http.Error(w, "cross-origin forbidden", http.StatusForbidden)
			return
		}
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("X-Frame-Options", "DENY")
		w.Header().Set("Content-Security-Policy",
			"default-src 'self'; img-src 'self' data:; style-src 'self' 'unsafe-inline'; object-src 'none'; base-uri 'none'; frame-ancestors 'none'")
		next.ServeHTTP(w, r)
	})
}

func sameOrigin(origin, host string) bool {
	u, err := url.Parse(origin)
	if err != nil {
		return false
	}
	return u.Host == host
}

func (s *Server) routes() error {

	s.mux.HandleFunc("GET /api/meta", s.handleMeta)
	s.mux.HandleFunc("GET /api/analyses", s.handleList)
	s.mux.HandleFunc("POST /api/analyses", s.handleCreate)
	s.mux.HandleFunc("GET /api/analyses/{id}", s.handleGet)
	s.mux.HandleFunc("PUT /api/analyses/{id}", s.handleUpdate)
	s.mux.HandleFunc("DELETE /api/analyses/{id}", s.handleDelete)
	s.mux.HandleFunc("POST /api/analyses/{id}/attachments", s.handleAddAttachment)
	s.mux.HandleFunc("DELETE /api/analyses/{id}/attachments", s.handleRemoveAttachment)
	s.mux.HandleFunc("POST /api/analyses/{id}/open-folder", s.handleOpenFolder)
	s.mux.HandleFunc("POST /api/registry/rebuild", s.handleRebuild)
	s.mux.HandleFunc("POST /api/registry/open", s.handleOpenRegistry)
	s.mux.HandleFunc("POST /api/backup", s.handleBackup)

	s.mux.HandleFunc("POST /api/admin/verify", s.handleAdminVerify)
	s.mux.HandleFunc("GET /api/admin/deleted", s.handleAdminDeleted)
	s.mux.HandleFunc("POST /api/admin/analyses/{id}/restore", s.handleAdminRestore)
	s.mux.HandleFunc("DELETE /api/admin/analyses/{id}", s.handleAdminPurge)

	s.mux.HandleFunc("GET /files/{id}/{path...}", s.handleServeFile)

	assets, err := fs.Sub(web.FS, "assets")
	if err != nil {
		return err
	}
	s.mux.Handle("/", http.FileServerFS(assets))
	return nil
}
