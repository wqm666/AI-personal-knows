package api

import (
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"strconv"
	"strings"

	"github.com/personal-know/internal/port"
	"github.com/personal-know/internal/service"
)

type Router struct {
	svc      *service.Service
	identity port.IdentityProvider
}

const defaultListLimit = 20

func NewRouter(svc *service.Service, identity port.IdentityProvider) *Router {
	return &Router{svc: svc, identity: identity}
}

func (r *Router) Handler() http.Handler {
	mux := http.NewServeMux()

	mux.HandleFunc("/api/knowledge/", r.withIdentity(r.handleKnowledge))
	mux.HandleFunc("/api/knowledge", r.withIdentity(r.handleKnowledge))
	mux.HandleFunc("/api/search", r.withIdentity(r.handleSearch))
	mux.HandleFunc("/api/import", r.withIdentity(r.handleImport))
	mux.HandleFunc("/api/capture", r.withIdentity(r.handleCapture))
	mux.HandleFunc("/api/feedback", r.withIdentity(r.handleFeedback))
	mux.HandleFunc("/api/maintain", r.withIdentity(r.handleMaintain))
	mux.HandleFunc("/api/stats", r.withIdentity(r.handleStats))
	mux.HandleFunc("/api/search_log", r.withIdentity(r.handleSearchLog))

	return mux
}

func (r *Router) handleKnowledge(w http.ResponseWriter, req *http.Request) {
	id := strings.TrimPrefix(req.URL.Path, "/api/knowledge/")
	id = strings.TrimPrefix(id, "/api/knowledge")
	if id == "" || id == "/" {
		r.handleKnowledgeList(w, req)
	} else {
		r.handleKnowledgeItem(w, req, id)
	}
}

func (r *Router) withIdentity(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, req *http.Request) {
		identity, err := r.identity.Resolve(req.Context())
		if err != nil {
			writeError(w, http.StatusUnauthorized, "unauthorized")
			return
		}
		ctx := port.ContextWithIdentity(req.Context(), identity)
		next(w, req.WithContext(ctx))
	}
}

func (r *Router) handleKnowledgeList(w http.ResponseWriter, req *http.Request) {
	switch req.Method {
	case http.MethodGet:
		offset, _ := strconv.Atoi(req.URL.Query().Get("offset"))
		limit, _ := strconv.Atoi(req.URL.Query().Get("limit"))
		if offset < 0 {
			offset = 0
		}
		if limit <= 0 || limit > 100 {
			limit = defaultListLimit
		}

		items, total, err := r.svc.ListKnowledge(req.Context(), offset, limit)
		if err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		writeJSON(w, map[string]any{"items": items, "total": total, "offset": offset, "limit": limit})

	case http.MethodPost:
		var body struct {
			Title     string   `json:"title"`
			Content   string   `json:"content"`
			Tags      []string `json:"tags"`
			Source    string   `json:"source"`
			SourceRef string   `json:"source_ref"`
		}
		if err := json.NewDecoder(req.Body).Decode(&body); err != nil {
			writeError(w, http.StatusBadRequest, "invalid JSON")
			return
		}

		result, err := r.svc.Save(req.Context(), body.Title, body.Content, body.Source, body.SourceRef, body.Tags)
		if err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		writeJSON(w, result)

	default:
		w.WriteHeader(http.StatusMethodNotAllowed)
	}
}

func (r *Router) handleKnowledgeItem(w http.ResponseWriter, req *http.Request, id string) {
	switch req.Method {
	case http.MethodGet:
		item, err := r.svc.GetKnowledge(req.Context(), id)
		if err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		if item == nil {
			writeError(w, http.StatusNotFound, "not found")
			return
		}
		writeJSON(w, item)

	case http.MethodPut:
		var body struct {
			Title         string   `json:"title"`
			Content       string   `json:"content"`
			Tags          []string `json:"tags"`
			KnowledgeType string   `json:"knowledge_type"`
		}
		if err := json.NewDecoder(req.Body).Decode(&body); err != nil {
			writeError(w, http.StatusBadRequest, "invalid JSON")
			return
		}
		result, err := r.svc.UpdateKnowledge(req.Context(), id, body.Title, body.Content, body.Tags, body.KnowledgeType)
		if err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		writeJSON(w, result)

	case http.MethodDelete:
		if err := r.svc.DeleteKnowledge(req.Context(), id); err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		writeJSON(w, map[string]bool{"deleted": true})

	default:
		w.WriteHeader(http.StatusMethodNotAllowed)
	}
}

func (r *Router) handleSearch(w http.ResponseWriter, req *http.Request) {
	if req.Method != http.MethodGet && req.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

	var query string
	var limit int

	if req.Method == http.MethodGet {
		query = req.URL.Query().Get("q")
		limit, _ = strconv.Atoi(req.URL.Query().Get("limit"))
	} else {
		var body struct {
			Query string `json:"query"`
			Limit int    `json:"limit"`
		}
		if err := json.NewDecoder(req.Body).Decode(&body); err != nil {
			writeError(w, http.StatusBadRequest, "invalid JSON")
			return
		}
		query = body.Query
		limit = body.Limit
	}

	if query == "" {
		writeError(w, http.StatusBadRequest, "query required")
		return
	}

	result, err := r.svc.Search(req.Context(), query, limit)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, result)
}

func (r *Router) handleImport(w http.ResponseWriter, req *http.Request) {
	if req.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

	contentType := req.Header.Get("Content-Type")

	const maxUploadSize = 32 << 20 // 32 MB
	req.Body = http.MaxBytesReader(w, req.Body, maxUploadSize)
	if strings.HasPrefix(contentType, "multipart/form-data") {
		if err := req.ParseMultipartForm(maxUploadSize); err != nil {
			writeError(w, http.StatusBadRequest, "parse form: "+err.Error())
			return
		}

		file, header, err := req.FormFile("file")
		if err != nil {
			writeError(w, http.StatusBadRequest, "file required")
			return
		}
		defer file.Close()

		data, err := io.ReadAll(file)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "read file: "+err.Error())
			return
		}

		chunkMode := req.FormValue("chunk_mode")
		result, err := r.svc.Import(req.Context(), string(data), header.Filename, chunkMode)
		if err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		writeJSON(w, result)
		return
	}

	var body struct {
		FileContent string `json:"file_content"`
		FileName    string `json:"file_name"`
		ChunkMode   string `json:"chunk_mode"`
	}
	req.Body = http.MaxBytesReader(w, req.Body, maxUploadSize)
	if err := json.NewDecoder(req.Body).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON")
		return
	}

	result, err := r.svc.Import(req.Context(), body.FileContent, body.FileName, body.ChunkMode)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, result)
}

func (r *Router) handleCapture(w http.ResponseWriter, req *http.Request) {
	if req.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

	var body struct {
		SessionSummary string `json:"session_summary"`
		ItemsJSON      string `json:"items_json"`
	}
	if err := json.NewDecoder(req.Body).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON")
		return
	}

	result, err := r.svc.Capture(req.Context(), body.SessionSummary, body.ItemsJSON)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, result)
}

func (r *Router) handleFeedback(w http.ResponseWriter, req *http.Request) {
	if req.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

	var body struct {
		ItemID string `json:"item_id"`
	}
	if err := json.NewDecoder(req.Body).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON")
		return
	}

	if err := r.svc.Feedback(req.Context(), body.ItemID); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, map[string]bool{"recorded": true})
}

func (r *Router) handleMaintain(w http.ResponseWriter, req *http.Request) {
	switch req.Method {
	case http.MethodGet:
		writeJSON(w, map[string]any{"tasks": r.svc.ListMaintainTasks()})

	case http.MethodPost:
		var body struct {
			Tasks []string `json:"tasks"`
		}
		if req.ContentLength > 0 {
			if err := json.NewDecoder(req.Body).Decode(&body); err != nil {
				writeError(w, http.StatusBadRequest, "invalid JSON")
				return
			}
		}

		results, err := r.svc.Maintain(req.Context(), body.Tasks...)
		if err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		writeJSON(w, map[string]any{"results": results})

	default:
		w.WriteHeader(http.StatusMethodNotAllowed)
	}
}

func (r *Router) handleStats(w http.ResponseWriter, req *http.Request) {
	if req.Method != http.MethodGet {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

	stats, err := r.svc.Stats(req.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, stats)
}

func (r *Router) handleSearchLog(w http.ResponseWriter, req *http.Request) {
	if req.Method != http.MethodGet {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

	stats, err := r.svc.SearchLogStats(req.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, stats)
}

func writeJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(v)
}

func writeError(w http.ResponseWriter, code int, msg string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	if code >= 500 {
		slog.Error("internal error", "status", code, "detail", msg)
		json.NewEncoder(w).Encode(map[string]string{"error": "internal server error"})
	} else {
		json.NewEncoder(w).Encode(map[string]string{"error": msg})
	}
}
