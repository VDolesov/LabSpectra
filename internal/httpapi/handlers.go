package httpapi

import (
	"encoding/json"
	"net"
	"net/http"
	"path/filepath"
	"strings"

	"labspectra/internal/domain"
	"labspectra/internal/service"
)

const maxUploadBytes = 100 << 20

func writeJSON(w http.ResponseWriter, status int, v interface{}) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func writeErr(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, map[string]string{"error": msg})
}

func (s *Server) handleMeta(w http.ResponseWriter, r *http.Request) {
	statuses := make([]string, 0)
	for _, st := range domain.AllStatuses() {
		statuses = append(statuses, string(st))
	}
	local := isLocalRequest(r)
	root := ""
	if local {
		root = s.svc.Root()
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"root":                   root,
		"statuses":               statuses,
		"products":               s.svc.Products(),
		"sources":                domain.Sources(),
		"characteristic_options": s.svc.Characteristics(),
		"characteristics":        s.svc.Characteristics(),
		"origin_acripol":         domain.OriginAcripol,
		"can_open_local":         local,
	})
}

func (s *Server) handleList(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	list := s.svc.List(service.Filter{
		Query:         q.Get("q"),
		Status:        q.Get("status"),
		AnalysisFrom:  q.Get("a_from"),
		AnalysisTo:    q.Get("a_to"),
		SynthesisFrom: q.Get("s_from"),
		SynthesisTo:   q.Get("s_to"),
	})
	writeJSON(w, http.StatusOK, map[string]interface{}{"items": list, "count": len(list)})
}

func (s *Server) handleCreate(w http.ResponseWriter, r *http.Request) {
	var input service.CreateInput
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		writeErr(w, http.StatusBadRequest, "некорректный JSON: "+err.Error())
		return
	}
	a, err := s.svc.Create(input)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusCreated, a)
}

func (s *Server) handleGet(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	a, err := s.svc.Get(id)
	if err != nil {
		writeErr(w, http.StatusNotFound, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, a)
}

func (s *Server) handleUpdate(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	var input service.UpdateInput
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		writeErr(w, http.StatusBadRequest, "некорректный JSON: "+err.Error())
		return
	}
	a, err := s.svc.Update(id, input)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, a)
}

func (s *Server) handleDelete(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if err := s.svc.SoftDelete(id); err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (s *Server) requireAdmin(w http.ResponseWriter, r *http.Request) bool {
	if !s.svc.CheckAdmin(r.Header.Get("X-Admin-Password")) {
		writeErr(w, http.StatusUnauthorized, "требуется пароль администратора")
		return false
	}
	return true
}

func (s *Server) handleAdminVerify(w http.ResponseWriter, r *http.Request) {
	if !s.requireAdmin(w, r) {
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (s *Server) handleAdminDeleted(w http.ResponseWriter, r *http.Request) {
	if !s.requireAdmin(w, r) {
		return
	}
	list := s.svc.ListDeleted()
	writeJSON(w, http.StatusOK, map[string]interface{}{"items": list, "count": len(list)})
}

func (s *Server) handleAdminRestore(w http.ResponseWriter, r *http.Request) {
	if !s.requireAdmin(w, r) {
		return
	}
	if err := s.svc.Restore(r.PathValue("id")); err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (s *Server) handleAdminPurge(w http.ResponseWriter, r *http.Request) {
	if !s.requireAdmin(w, r) {
		return
	}
	if err := s.svc.Purge(r.PathValue("id")); err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (s *Server) handleAdminProducts(w http.ResponseWriter, r *http.Request) {
	if !s.requireAdmin(w, r) {
		return
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{"items": s.svc.Products()})
}

func (s *Server) handleAdminAddProduct(w http.ResponseWriter, r *http.Request) {
	if !s.requireAdmin(w, r) {
		return
	}
	var input struct {
		Product string `json:"product"`
	}
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		writeErr(w, http.StatusBadRequest, "некорректный JSON: "+err.Error())
		return
	}
	products, err := s.svc.AddProduct(input.Product)
	if err != nil {
		writeErr(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{"items": products})
}

func (s *Server) handleAdminDeleteProduct(w http.ResponseWriter, r *http.Request) {
	if !s.requireAdmin(w, r) {
		return
	}
	products, err := s.svc.DeleteProduct(r.PathValue("product"))
	if err != nil {
		writeErr(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{"items": products})
}

func (s *Server) handleAdminCharacteristics(w http.ResponseWriter, r *http.Request) {
	if !s.requireAdmin(w, r) {
		return
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{"items": s.svc.Characteristics()})
}

func (s *Server) handleAdminAddCharacteristic(w http.ResponseWriter, r *http.Request) {
	if !s.requireAdmin(w, r) {
		return
	}
	var input struct {
		Name string `json:"name"`
	}
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		writeErr(w, http.StatusBadRequest, "некорректный JSON: "+err.Error())
		return
	}
	list, err := s.svc.AddCharacteristic(input.Name)
	if err != nil {
		writeErr(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{"items": list})
}

func (s *Server) handleAdminDeleteCharacteristic(w http.ResponseWriter, r *http.Request) {
	if !s.requireAdmin(w, r) {
		return
	}
	name := r.URL.Query().Get("name")
	list, err := s.svc.DeleteCharacteristic(name)
	if err != nil {
		writeErr(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{"items": list})
}

func (s *Server) handleAddAttachment(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	kind := domain.Kind(r.URL.Query().Get("kind"))
	if !kind.Valid() {
		writeErr(w, http.StatusBadRequest, "неизвестный тип вложения (kind): photo|spectrum")
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, maxUploadBytes)
	if err := r.ParseMultipartForm(64 << 20); err != nil {
		writeErr(w, http.StatusBadRequest, "не удалось прочитать загрузку: "+err.Error())
		return
	}
	files := r.MultipartForm.File["files"]
	if len(files) == 0 {
		files = r.MultipartForm.File["file"]
	}
	if len(files) == 0 {
		writeErr(w, http.StatusBadRequest, "не выбраны файлы")
		return
	}
	var a interface{}
	for _, fh := range files {
		f, err := fh.Open()
		if err != nil {
			writeErr(w, http.StatusInternalServerError, err.Error())
			return
		}
		res, err := s.svc.AddAttachment(id, kind, fh.Filename, f)
		f.Close()
		if err != nil {
			writeErr(w, http.StatusInternalServerError, err.Error())
			return
		}
		a = res
	}
	writeJSON(w, http.StatusOK, a)
}

func (s *Server) handleRemoveAttachment(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	kind := domain.Kind(r.URL.Query().Get("kind"))
	rel := r.URL.Query().Get("name")
	if rel == "" {
		writeErr(w, http.StatusBadRequest, "не указан путь вложения (name)")
		return
	}
	a, err := s.svc.RemoveAttachment(id, kind, rel)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, a)
}

func (s *Server) handleOpenFolder(w http.ResponseWriter, r *http.Request) {
	if !isLocalRequest(r) {
		writeErr(w, http.StatusForbidden, "открытие папки доступно только при локальном запуске")
		return
	}
	id := r.PathValue("id")
	if err := s.svc.OpenFolder(id); err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (s *Server) handleBackup(w http.ResponseWriter, r *http.Request) {
	if !s.requireAdmin(w, r) {
		return
	}
	path, err := s.svc.Backup()
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	name := filepath.Base(path)
	w.Header().Set("Content-Type", "application/zip")
	w.Header().Set("Content-Disposition", `attachment; filename="`+name+`"`)
	w.Header().Set("Cache-Control", "no-store")
	http.ServeFile(w, r, path)
}

func (s *Server) handleOpenRegistry(w http.ResponseWriter, r *http.Request) {
	if !isLocalRequest(r) {
		writeErr(w, http.StatusForbidden, "открытие Excel доступно только при локальном запуске")
		return
	}
	if err := s.svc.OpenRegistry(); err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (s *Server) handleRebuild(w http.ResponseWriter, r *http.Request) {
	if !s.requireAdmin(w, r) {
		return
	}
	n, err := s.svc.RebuildRegistry()
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]int{"rebuilt": n})
}

func (s *Server) handleServeFile(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	rel := r.PathValue("path")
	abs, err := s.svc.AttachmentFile(id, rel)
	if err != nil {
		writeErr(w, http.StatusBadRequest, "некорректный путь")
		return
	}
	w.Header().Set("X-Content-Type-Options", "nosniff")
	if activeContentExt(rel) {
		w.Header().Set("Content-Disposition", "attachment")
	}
	http.ServeFile(w, r, abs)
}

func activeContentExt(name string) bool {
	switch strings.ToLower(filepath.Ext(name)) {
	case ".html", ".htm", ".svg", ".xml", ".xhtml", ".js":
		return true
	}
	return false
}

func isLocalRequest(r *http.Request) bool {
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		host = r.RemoteAddr
	}
	ip := net.ParseIP(host)
	return ip != nil && ip.IsLoopback()
}
