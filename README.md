# Personal Know

> AI-native personal knowledge base with MCP integration — let your AI assistant remember, search, and evolve knowledge across conversations.

Personal Know is a self-hosted knowledge management server that exposes both **MCP (Model Context Protocol)** tools and a **REST API**. AI agents (Claude Code, Cursor, GPT, etc.) can store, search, and maintain knowledge through MCP, while humans can manage knowledge through the built-in web UI.

## Features

- **MCP Protocol Support** — 7 tools for AI agents via StreamableHTTP
- **Multi-Strategy Retrieval** — Vector similarity + Full-text search + Keyword matching, in parallel
- **Smart Deduplication** — Vector-based dedup with 3 strategies: reinforce / relate / new
- **Auto Knowledge Extraction** — LLM detects signals (errors, pitfalls) and extracts structured knowledge from conversations
- **Knowledge Graph Expansion** — BFS traversal along `related_ids` to surface connected knowledge
- **Self-Maintenance** — Background tasks for link discovery, consolidation, decay, and tag normalization
- **Built-in Web UI** — Embedded SPA for browsing, searching, and managing knowledge
- **Hexagonal Architecture** — Clean separation of ports and adapters for easy extension

## Architecture

```
┌─────────────────┐     ┌─────────────────┐
│   AI Agent       │     │   Web Browser    │
│ (Claude/Cursor)  │     │   (Built-in UI)  │
└────────┬─────────┘     └────────┬─────────┘
         │ MCP (StreamableHTTP)    │ REST API
         └──────────┬──────────────┘
                    │
         ┌──────────▼──────────┐
         │   HTTP Server :8081  │
         │  CORS + API Key Auth │
         └──────────┬──────────┘
                    │
         ┌──────────▼──────────┐
         │    Service Layer     │
         │  (Business Logic)    │
         └──────────┬──────────┘
                    │
    ┌───────────────┼───────────────┐
    │               │               │
┌───▼───┐   ┌──────▼──────┐   ┌───▼────┐
│ Store  │   │ Orchestrator │   │  LLM   │
│(pgvec) │   │ (3 retrievers)│  │ Client │
└────────┘   └─────────────┘   └────────┘
```

## MCP Tools

| Tool | Description |
|------|-------------|
| `note_search` | Semantic search across the knowledge base |
| `note_save` | Save knowledge with auto-dedup and linking |
| `note_capture` | Extract knowledge from conversation sessions via LLM |
| `note_import` | Import documents with auto-chunking by Markdown headings |
| `note_update` | Update existing knowledge items |
| `note_feedback` | Mark knowledge as useful (affects ranking and decay) |
| `note_maintain` | Trigger maintenance: link discovery, consolidation, decay, tag normalization |

## Quick Start

### Prerequisites

- Docker & Docker Compose
- An OpenAI-compatible API key (for embeddings and LLM)

### 1. Clone and configure

```bash
git clone https://github.com/your-username/personal-know.git
cd personal-know

# Set up secrets
cp .env.example .env
cp config.json.example config.json

# Edit .env — set POSTGRES_PASSWORD
# Edit config.json — set your LLM API key and endpoint
```

### 2. Deploy

```bash
chmod +x deploy.sh
./deploy.sh
```

### 3. Verify

```bash
curl http://localhost:8081/health
# {"status":"ok"}
```

- **Web UI**: http://localhost:8081
- **MCP endpoint**: http://localhost:8081/mcp
- **REST API**: http://localhost:8081/api/*

### 4. Connect your AI agent

Add to your MCP client configuration:

```json
{
  "mcpServers": {
    "personal-know": {
      "url": "http://localhost:8081/mcp"
    }
  }
}
```

## Configuration

### config.json

```json
{
  "server": {
    "addr": ":8081",
    "cors_origins": [],
    "api_key": ""
  },
  "store": {
    "type": "postgres",
    "dsn": "postgres://user:pass@host:5432/db?sslmode=disable"
  },
  "retrievers": [
    { "type": "keyword", "enabled": true, "params": { "fetch_limit": 500 } },
    { "type": "fts", "enabled": true },
    { "type": "vector", "enabled": true, "params": { "score_threshold": 0.7 } }
  ],
  "llm": {
    "base_url": "https://api.openai.com",
    "api_key": "your-api-key",
    "chat_model": "gpt-4o-mini",
    "embedding_model": "text-embedding-3-small"
  }
}
```

All fields can be overridden by environment variables:

| Environment Variable | Description |
|---------------------|-------------|
| `DATABASE_URL` | PostgreSQL connection string |
| `LLM_BASE_URL` | LLM API base URL |
| `LLM_API_KEY` | LLM API key |
| `LLM_CHAT_MODEL` | Chat model name |
| `LLM_EMBEDDING_MODEL` | Embedding model name |
| `SERVER_ADDR` | Listen address (default `:8081`) |
| `SERVER_API_KEY` | Optional API key for authentication |
| `CORS_ORIGINS` | Comma-separated allowed origins |

## How It Works

### Knowledge Lifecycle

```
 Create ─→ Embed ─→ Dedup ─→ Store ─→ Search ─→ Maintain
   │                  │                    │          │
   ├─ conversation    ├─ ≥0.95: reinforce  │          ├─ link discovery
   ├─ document        ├─ 0.75~0.95: relate │          ├─ consolidation
   └─ manual          └─ <0.75: new        │          ├─ tag normalization
                                           │          └─ decay (90 days)
                                           │
                                      Multi-strategy
                                      retrieval + BFS
                                      graph expansion
```

### Search Pipeline

1. **Multi-Strategy Retrieval** — Vector (cosine ≥ 0.7), FTS (`tsvector`), Keyword (word frequency) run in parallel
2. **Merge & Dedup** — Same item from multiple retrievers keeps the highest score
3. **BFS Graph Expansion** — Follow `related_ids` up to 2 layers deep (max 100 items)
4. **Synthesis Resolve** — Expand `consolidated_from` for synthesis nodes

### Dedup Strategy (on save)

| Similarity | Action | Behavior |
|-----------|--------|----------|
| ≥ 0.95 | **Reinforce** | Skip save, increment `hit_count` on existing |
| 0.75–0.95 | **Relate** | Save new item + bidirectional link in transaction |
| < 0.75 | **New** | Save as independent item |

### Maintenance Tasks

| Task | Description |
|------|-------------|
| `link_discovery` | Find and link items with similarity 0.75–0.95 |
| `consolidation` | Merge 3+ related items into a synthesis node via LLM |
| `tag_cluster` | Normalize synonymous tags via LLM (e.g., "golang" → "Go") |
| `decay` | Mark items as decayed after 90 days with no hits and no feedback |

## REST API

| Method | Endpoint | Description |
|--------|----------|-------------|
| GET | `/api/knowledge` | List knowledge (paginated) |
| POST | `/api/knowledge` | Create knowledge item |
| GET | `/api/knowledge/:id` | Get single item |
| PUT | `/api/knowledge/:id` | Update item |
| DELETE | `/api/knowledge/:id` | Delete item |
| GET/POST | `/api/search` | Search knowledge base |
| POST | `/api/import` | Import document (multipart or JSON) |
| POST | `/api/capture` | Capture session knowledge |
| POST | `/api/feedback` | Record useful feedback |
| GET/POST | `/api/maintain` | List or run maintenance tasks |
| GET | `/api/stats` | Knowledge base statistics |
| GET | `/api/search_log` | Search query analytics |

## Tech Stack

- **Language**: Go 1.23
- **Database**: PostgreSQL 16 + pgvector (HNSW index, cosine distance)
- **MCP**: [mcp-go](https://github.com/mark3labs/mcp-go) (StreamableHTTP transport)
- **LLM**: OpenAI-compatible API (embeddings + chat)
- **Frontend**: Embedded vanilla JS SPA
- **Deployment**: Docker Compose

## Project Structure

```
.
├── cmd/server/          # Entry point & dependency wiring
├── internal/
│   ├── domain/          # Domain models (Knowledge, SearchHit, etc.)
│   ├── port/            # Interface definitions (Store, Retriever, Embedder, etc.)
│   ├── service/         # Business logic orchestration
│   └── adapter/
│       ├── api/         # REST API router
│       ├── transport/   # MCP server & tool handlers
│       ├── store/       # PostgreSQL + pgvector implementation
│       ├── retriever/   # Vector, FTS, Keyword retrievers + orchestrator
│       ├── embedder/    # OpenAI-compatible embedding client
│       ├── llm/         # OpenAI-compatible chat client
│       ├── dedup/       # Vector similarity deduplication
│       ├── maintain/    # Link discovery, consolidation, decay, tag clustering
│       └── identity/    # Owner identity provider
├── web/                 # Embedded static web UI
├── config.json.example  # Configuration template
├── docker-compose.yml   # Container orchestration
├── Dockerfile           # Multi-stage build
└── deploy.sh            # One-click deployment script
```

## License

[MIT](LICENSE)
