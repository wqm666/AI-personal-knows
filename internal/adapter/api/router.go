package api

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	pdflib "github.com/ledongthuc/pdf"
	"github.com/nguyenthenguyen/docx"
	"github.com/personal-know/internal/domain"
	"github.com/personal-know/internal/port"
	"github.com/personal-know/internal/service"
)

type Router struct {
	svc      *service.Service
	identity port.IdentityProvider
}

const (
	defaultListLimit  = 20
	maxUploadSize     = 32 << 20 // 32 MB
	maxBulkImportSize = 64 << 20 // 64 MB
)

var xmlTagRe = regexp.MustCompile(`<[^>]*>`)

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
	mux.HandleFunc("/api/review", r.withIdentity(r.handleReview))
	mux.HandleFunc("/api/export", r.withIdentity(r.handleExport))
	mux.HandleFunc("/api/import/bulk", r.withIdentity(r.handleBulkImport))
	mux.HandleFunc("/api/stats", r.withIdentity(r.handleStats))
	mux.HandleFunc("/api/search_log", r.withIdentity(r.handleSearchLog))
	mux.HandleFunc("/api/monitor/ranking", r.withIdentity(r.handleHitRanking))
	mux.HandleFunc("/api/monitor/logs", r.withIdentity(r.handleMonitorLogs))
	mux.HandleFunc("/api/monitor/bad_recall", r.withIdentity(r.handleBadRecall))
	mux.HandleFunc("/api/worklog/", r.withIdentity(r.handleWorkLogItem))
	mux.HandleFunc("/api/worklog", r.withIdentity(r.handleWorkLog))

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

	result, err := r.svc.Search(req.Context(), query, limit, domain.SearchSourceWeb)
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

		var fileContent string
		ext := strings.ToLower(filepath.Ext(header.Filename))
		switch ext {
		case ".pdf":
			text, err := extractPDFText(data)
			if err != nil {
				writeError(w, http.StatusBadRequest, "parse PDF failed: "+err.Error())
				return
			}
			fileContent = text
		case ".docx":
			text, err := extractDocxText(data)
			if err != nil {
				writeError(w, http.StatusBadRequest, "parse DOCX failed: "+err.Error())
				return
			}
			fileContent = text
		default:
			fileContent = string(data)
		}

		chunkMode := req.FormValue("chunk_mode")
		result, err := r.svc.Import(req.Context(), fileContent, header.Filename, chunkMode)
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

func (r *Router) handleReview(w http.ResponseWriter, req *http.Request) {
	switch req.Method {
	case http.MethodGet:
		limit, _ := strconv.Atoi(req.URL.Query().Get("limit"))
		result, err := r.svc.ListPending(req.Context(), limit)
		if err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		writeJSON(w, result)

	case http.MethodPost:
		var body struct {
			Action string `json:"action"`
			ID     string `json:"id"`
			Reason string `json:"reason"`
		}
		if err := json.NewDecoder(req.Body).Decode(&body); err != nil {
			writeError(w, http.StatusBadRequest, "invalid JSON")
			return
		}

		switch body.Action {
		case "approve":
			if body.ID == "" {
				writeError(w, http.StatusBadRequest, "id required")
				return
			}
			if err := r.svc.ApproveKnowledge(req.Context(), body.ID, body.Reason); err != nil {
				writeError(w, http.StatusInternalServerError, err.Error())
				return
			}
			writeJSON(w, map[string]any{"approved": true, "id": body.ID})

		case "reject":
			if body.ID == "" {
				writeError(w, http.StatusBadRequest, "id required")
				return
			}
			if err := r.svc.RejectKnowledge(req.Context(), body.ID, body.Reason); err != nil {
				writeError(w, http.StatusInternalServerError, err.Error())
				return
			}
			writeJSON(w, map[string]any{"rejected": true, "id": body.ID})

		case "revision":
			if body.ID == "" {
				writeError(w, http.StatusBadRequest, "id required")
				return
			}
			if err := r.svc.RequestRevision(req.Context(), body.ID, body.Reason); err != nil {
				writeError(w, http.StatusInternalServerError, err.Error())
				return
			}
			writeJSON(w, map[string]any{"revision_requested": true, "id": body.ID})

		case "suggest":
			if body.ID == "" {
				writeError(w, http.StatusBadRequest, "id required")
				return
			}
			suggestion, err := r.svc.SuggestReview(req.Context(), body.ID)
			if err != nil {
				writeError(w, http.StatusInternalServerError, err.Error())
				return
			}
			writeJSON(w, suggestion)

		case "pending":
			if body.ID == "" {
				writeError(w, http.StatusBadRequest, "id required")
				return
			}
			if err := r.svc.SetReviewStatus(req.Context(), body.ID, domain.ReviewPending, body.Reason); err != nil {
				writeError(w, http.StatusInternalServerError, err.Error())
				return
			}
			writeJSON(w, map[string]any{"set_pending": true, "id": body.ID})

		default:
			writeError(w, http.StatusBadRequest, "action must be approve/reject/revision/pending/suggest")
		}

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

func (r *Router) handleHitRanking(w http.ResponseWriter, req *http.Request) {
	if req.Method != http.MethodGet {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	limit, _ := strconv.Atoi(req.URL.Query().Get("limit"))
	ranking, err := r.svc.KnowledgeHitRanking(req.Context(), limit)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, map[string]any{"items": ranking})
}

func (r *Router) handleMonitorLogs(w http.ResponseWriter, req *http.Request) {
	if req.Method != http.MethodGet {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	offset, _ := strconv.Atoi(req.URL.Query().Get("offset"))
	limit, _ := strconv.Atoi(req.URL.Query().Get("limit"))
	source := req.URL.Query().Get("source")
	logs, total, err := r.svc.ListSearchLogsDetailed(req.Context(), offset, limit, source)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, map[string]any{"items": logs, "total": total})
}

func (r *Router) handleBadRecall(w http.ResponseWriter, req *http.Request) {
	if req.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	var body struct {
		SearchLogID string `json:"search_log_id"`
		BadItemID   string `json:"bad_item_id"`
	}
	if err := json.NewDecoder(req.Body).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON")
		return
	}
	if body.SearchLogID == "" || body.BadItemID == "" {
		writeError(w, http.StatusBadRequest, "search_log_id and bad_item_id required")
		return
	}
	if err := r.svc.MarkBadRecall(req.Context(), body.SearchLogID, body.BadItemID); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, map[string]any{"marked": true})
}

func writeJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(v)
}

func (r *Router) handleExport(w http.ResponseWriter, req *http.Request) {
	if req.Method != http.MethodGet {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

	data, err := r.svc.Export(req.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Content-Disposition", "attachment; filename=personal-know-export.json")
	json.NewEncoder(w).Encode(data)
}

func (r *Router) handleBulkImport(w http.ResponseWriter, req *http.Request) {
	if req.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

	req.Body = http.MaxBytesReader(w, req.Body, maxBulkImportSize)

	contentType := req.Header.Get("Content-Type")

	var exportData service.ExportData

	if strings.HasPrefix(contentType, "multipart/form-data") {
		if err := req.ParseMultipartForm(maxBulkImportSize); err != nil {
			writeError(w, http.StatusBadRequest, "parse form: "+err.Error())
			return
		}
		file, _, err := req.FormFile("file")
		if err != nil {
			writeError(w, http.StatusBadRequest, "file required")
			return
		}
		defer file.Close()
		if err := json.NewDecoder(file).Decode(&exportData); err != nil {
			writeError(w, http.StatusBadRequest, "invalid export JSON: "+err.Error())
			return
		}
	} else {
		if err := json.NewDecoder(req.Body).Decode(&exportData); err != nil {
			writeError(w, http.StatusBadRequest, "invalid JSON: "+err.Error())
			return
		}
	}

	if len(exportData.Items) == 0 {
		writeError(w, http.StatusBadRequest, "no items to import")
		return
	}

	result, err := r.svc.BulkImport(req.Context(), &exportData)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, result)
}

func (r *Router) handleWorkLog(w http.ResponseWriter, req *http.Request) {
	switch req.Method {
	case http.MethodGet:
		dateFrom := req.URL.Query().Get("from")
		dateTo := req.URL.Query().Get("to")
		offset, _ := strconv.Atoi(req.URL.Query().Get("offset"))
		limit, _ := strconv.Atoi(req.URL.Query().Get("limit"))
		items, total, err := r.svc.ListWorkLogs(req.Context(), dateFrom, dateTo, offset, limit)
		if err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		writeJSON(w, map[string]any{"items": items, "total": total})

	case http.MethodPost:
		var body struct {
			Date     string   `json:"date"`
			Content  string   `json:"content"`
			Project  string   `json:"project"`
			Tags     []string `json:"tags"`
			Duration int      `json:"duration"`
		}
		if err := json.NewDecoder(req.Body).Decode(&body); err != nil {
			writeError(w, http.StatusBadRequest, "invalid JSON")
			return
		}
		item, err := r.svc.AddWorkLog(req.Context(), body.Date, body.Content, body.Project, body.Tags, body.Duration)
		if err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		writeJSON(w, item)

	default:
		w.WriteHeader(http.StatusMethodNotAllowed)
	}
}

func (r *Router) handleWorkLogItem(w http.ResponseWriter, req *http.Request) {
	id := strings.TrimPrefix(req.URL.Path, "/api/worklog/")
	if id == "" {
		writeError(w, http.StatusBadRequest, "id required")
		return
	}

	switch req.Method {
	case http.MethodGet:
		item, err := r.svc.GetWorkLog(req.Context(), id)
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
			Date     string   `json:"date"`
			Content  string   `json:"content"`
			Project  string   `json:"project"`
			Tags     []string `json:"tags"`
			Duration int      `json:"duration"`
		}
		if err := json.NewDecoder(req.Body).Decode(&body); err != nil {
			writeError(w, http.StatusBadRequest, "invalid JSON")
			return
		}
		item, err := r.svc.UpdateWorkLog(req.Context(), id, body.Date, body.Content, body.Project, body.Tags, body.Duration)
		if err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		writeJSON(w, item)

	case http.MethodDelete:
		if err := r.svc.DeleteWorkLog(req.Context(), id); err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		writeJSON(w, map[string]bool{"deleted": true})

	default:
		w.WriteHeader(http.StatusMethodNotAllowed)
	}
}

func extractDocxText(data []byte) (string, error) {
	tmpFile, err := os.CreateTemp("", "know-upload-*.docx")
	if err != nil {
		return "", err
	}
	defer os.Remove(tmpFile.Name())

	if _, err := tmpFile.Write(data); err != nil {
		tmpFile.Close()
		return "", err
	}
	tmpFile.Close()

	r, err := docx.ReadDocxFile(tmpFile.Name())
	if err != nil {
		return "", err
	}
	defer r.Close()

	doc := r.Editable()
	text := doc.GetContent()
	text = strings.ReplaceAll(text, "</w:t></w:r></w:p>", "\n")
	text = strings.ReplaceAll(text, "</w:t>", " ")

	cleaned := xmlTagRe.ReplaceAllString(text, "")

	result := strings.TrimSpace(cleaned)
	if result == "" {
		return "", fmt.Errorf("no text content found in DOCX")
	}
	return result, nil
}

func extractPDFText(data []byte) (string, error) {
	tmpFile, err := os.CreateTemp("", "know-upload-*.pdf")
	if err != nil {
		return "", err
	}
	defer os.Remove(tmpFile.Name())

	if _, err := tmpFile.Write(data); err != nil {
		tmpFile.Close()
		return "", err
	}
	tmpFile.Close()

	f, reader, err := pdflib.Open(tmpFile.Name())
	if err != nil {
		return "", err
	}
	defer f.Close()

	var buf bytes.Buffer
	for i := 1; i <= reader.NumPage(); i++ {
		page := reader.Page(i)
		if page.V.IsNull() {
			continue
		}
		text, err := page.GetPlainText(nil)
		if err != nil {
			continue
		}
		if buf.Len() > 0 {
			buf.WriteString("\n\n")
		}
		buf.WriteString(text)
	}

	result := strings.TrimSpace(buf.String())
	if result == "" {
		return "", fmt.Errorf("no text content found in PDF")
	}
	return result, nil
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
