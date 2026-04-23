package main

import (
	"context"
	"io/fs"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/spf13/cast"

	"github.com/personal-know/internal/adapter/api"
	"github.com/personal-know/internal/adapter/config/jsonfile"
	"github.com/personal-know/internal/adapter/dedup/vector_dedup"
	openaiembed "github.com/personal-know/internal/adapter/embedder/openai"
	uuidgen "github.com/personal-know/internal/adapter/id/uuid"
	defaultident "github.com/personal-know/internal/adapter/identity/default_provider"
	openaillm "github.com/personal-know/internal/adapter/llm/openai"
	"github.com/personal-know/internal/adapter/maintain"
	"github.com/personal-know/internal/adapter/retriever/fts"
	"github.com/personal-know/internal/adapter/retriever/keyword"
	"github.com/personal-know/internal/adapter/retriever/merger"
	"github.com/personal-know/internal/adapter/retriever/orchestrator"
	"github.com/personal-know/internal/adapter/retriever/vector"
	"github.com/personal-know/internal/adapter/store/pgstore"
	mcptransport "github.com/personal-know/internal/adapter/transport/mcp"
	"github.com/personal-know/internal/port"
	"github.com/personal-know/internal/service"
	"github.com/personal-know/web"

	mcpserver "github.com/mark3labs/mcp-go/server"
)

func main() {
	slog.Info("personal-know starting")

	cfg := loadConfig()

	// --- Store ---
	pg, err := pgstore.New(cfg.Store.DSN, cfg.LLM.EmbeddingDimension)
	if err != nil {
		slog.Error("failed to init store", "error", err)
		os.Exit(1)
	}
	defer pg.Close()
	slog.Info("store initialized", "type", cfg.Store.Type)

	// --- ID Generator ---
	idGen := uuidgen.New()

	// --- Identity Provider ---
	identityProvider := defaultident.New("default")
	slog.Info("identity provider initialized", "type", "default")

	// --- Embedder ---
	var embedder port.Embedder
	if cfg.LLM.BaseURL != "" && cfg.LLM.EmbeddingModel != "" {
		embedder = openaiembed.New(cfg.LLM.BaseURL, cfg.LLM.APIKey, cfg.LLM.EmbeddingModel)
		slog.Info("embedder initialized", "model", cfg.LLM.EmbeddingModel)
	}

	// --- LLM Client ---
	var llm port.LLMClient
	if cfg.LLM.BaseURL != "" && cfg.LLM.ChatModel != "" {
		llm = openaillm.New(cfg.LLM.BaseURL, cfg.LLM.APIKey, cfg.LLM.ChatModel)
		slog.Info("llm client initialized", "model", cfg.LLM.ChatModel)
	}

	// --- Retrievers ---
	m := merger.New()
	orch := orchestrator.New(m)

	for _, rc := range cfg.Retrievers {
		if !rc.Enabled {
			continue
		}
		switch rc.Type {
		case "keyword":
			fetchLimit := cast.ToInt(rc.Params["fetch_limit"])
			orch.Register(keyword.New(pg, fetchLimit))
			slog.Info("retriever registered", "type", "keyword", "fetch_limit", fetchLimit)
		case "fts":
			orch.Register(fts.New(pg))
			slog.Info("retriever registered", "type", "fts")
		case "vector":
			if embedder != nil {
				scoreThreshold := cast.ToFloat64(rc.Params["score_threshold"])
				orch.Register(vector.New(pg, embedder, scoreThreshold))
				slog.Info("retriever registered", "type", "vector", "score_threshold", scoreThreshold)
			}
		}
	}

	// --- Deduplicator ---
	dedup := vector_dedup.New(pg)
	if llm != nil {
		dedup.SetLLM(llm)
	}

	// --- Maintainer ---
	runner := maintain.NewRunner()
	if embedder != nil {
		runner.Register(maintain.NewLinkDiscovery(pg, embedder))
	}
	if llm != nil {
		runner.Register(maintain.NewConsolidation(pg, llm, idGen))
		runner.Register(maintain.NewTagCluster(pg, llm))
	}
	runner.Register(maintain.NewDecay(pg))
	slog.Info("maintainer initialized", "tasks", runner.ListTasks())

	// --- Service ---
	svc := service.New(pg, orch, embedder, dedup, runner, idGen)
	if llm != nil {
		svc.SetLLM(llm)
	}

	// --- Backfill embeddings on startup ---
	backfillCtx, backfillCancel := context.WithCancel(context.Background())
	defer backfillCancel()
	go svc.BackfillEmbeddings(backfillCtx)

	// --- MCP Server ---
	mcpSrv := mcptransport.NewServer(svc, identityProvider)

	// --- REST API ---
	apiRouter := api.NewRouter(svc, identityProvider)

	// --- HTTP Mux ---
	mux := http.NewServeMux()
	httpServer := mcpserver.NewStreamableHTTPServer(mcpSrv)
	mux.Handle("/mcp", httpServer)
	mux.Handle("/api/", apiRouter.Handler())
	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"status":"ok"}`))
	})

	// --- Static files (embedded web UI) ---
	staticFS, err := fs.Sub(web.StaticFS, "static")
	if err != nil {
		slog.Warn("static files not available", "error", err)
	} else {
		mux.Handle("/", http.FileServer(http.FS(staticFS)))
	}

	handler := corsMiddleware(mux, cfg.Server.CORSOrigins)
	if cfg.Server.APIKey != "" {
		handler = apiKeyMiddleware(handler, cfg.Server.APIKey)
	}

	server := &http.Server{
		Addr:    cfg.Server.Addr,
		Handler: handler,
	}

	// --- Graceful shutdown ---
	go func() {
		slog.Info("listening", "addr", cfg.Server.Addr)
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			slog.Error("server error", "error", err)
			os.Exit(1)
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	slog.Info("shutting down")
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	server.Shutdown(ctx)
}

func loadConfig() *port.Config {
	configPath := "config.json"
	if envPath := os.Getenv("CONFIG_PATH"); envPath != "" {
		configPath = envPath
	}

	loader := jsonfile.New()
	cfg, err := loader.Load(configPath)
	if err != nil {
		slog.Warn("config not found, using defaults", "path", configPath, "error", err)
		cfg = jsonfile.DefaultConfig()
	}

	if dsn := os.Getenv("DATABASE_URL"); dsn != "" {
		cfg.Store.DSN = dsn
	}
	if baseURL := os.Getenv("LLM_BASE_URL"); baseURL != "" {
		cfg.LLM.BaseURL = baseURL
	}
	if apiKey := os.Getenv("LLM_API_KEY"); apiKey != "" {
		cfg.LLM.APIKey = apiKey
	}
	if model := os.Getenv("LLM_CHAT_MODEL"); model != "" {
		cfg.LLM.ChatModel = model
	}
	if model := os.Getenv("LLM_EMBEDDING_MODEL"); model != "" {
		cfg.LLM.EmbeddingModel = model
	}
	if addr := os.Getenv("SERVER_ADDR"); addr != "" {
		cfg.Server.Addr = addr
	}
	if key := os.Getenv("SERVER_API_KEY"); key != "" {
		cfg.Server.APIKey = key
	}
	if origins := os.Getenv("CORS_ORIGINS"); origins != "" {
		cfg.Server.CORSOrigins = strings.Split(origins, ",")
	}

	return cfg
}

func corsMiddleware(next http.Handler, allowedOrigins []string) http.Handler {
	allowAll := len(allowedOrigins) == 0
	originSet := make(map[string]bool, len(allowedOrigins))
	for _, o := range allowedOrigins {
		originSet[o] = true
	}

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		origin := r.Header.Get("Origin")
		if allowAll {
			w.Header().Set("Access-Control-Allow-Origin", "*")
		} else if originSet[origin] {
			w.Header().Set("Access-Control-Allow-Origin", origin)
			w.Header().Set("Vary", "Origin")
		}
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization, X-API-Key")
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("X-Frame-Options", "DENY")
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusOK)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func apiKeyMiddleware(next http.Handler, apiKey string) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Skip auth for health check and OPTIONS
		if r.URL.Path == "/health" || r.Method == http.MethodOptions {
			next.ServeHTTP(w, r)
			return
		}

		// Check X-API-Key header or query param
		key := r.Header.Get("X-API-Key")
		if key == "" {
			key = r.URL.Query().Get("api_key")
		}
		if key != apiKey {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusUnauthorized)
			w.Write([]byte(`{"error":"invalid or missing API key"}`))
			return
		}
		next.ServeHTTP(w, r)
	})
}
