package httpapi

import (
	"encoding/json"
	"net/http"
	"path/filepath"
	"strings"

	"labspectra/internal/domain"
	"labspectra/internal/service"
)

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
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"root":           s.svc.Root(),
		"statuses":       statuses,
		"products":       domain.Products(),
		"origin_acripol": domain.OriginAcripol,
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
	if err := s.svc.Delete(id); err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (s *Server) handleAddAttachment(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	kind := domain.Kind(r.URL.Query().Get("kind"))
	if !kind.Valid() {
		writeErr(w, http.StatusBadRequest, "неизвестный тип вложения (kind): photo|spectrum")
		return
	}
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
	id := r.PathValue("id")
	if err := s.svc.OpenFolder(id); err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (s *Server) handleBackup(w http.ResponseWriter, r *http.Request) {
	path, err := s.svc.Backup()
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"path": path})
}

func (s *Server) handleOpenRegistry(w http.ResponseWriter, r *http.Request) {
	if err := s.svc.OpenRegistry(); err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (s *Server) handleRebuild(w http.ResponseWriter, r *http.Request) {
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
