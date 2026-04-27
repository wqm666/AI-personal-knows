<p align="center">
  <img src="https://img.shields.io/badge/Go-1.23-00ADD8?style=flat-square&logo=go" alt="Go">
  <img src="https://img.shields.io/badge/PostgreSQL-16+pgvector-4169E1?style=flat-square&logo=postgresql" alt="PostgreSQL">
  <img src="https://img.shields.io/badge/MCP-StreamableHTTP-8A2BE2?style=flat-square" alt="MCP">
  <img src="https://img.shields.io/badge/License-MIT-green?style=flat-square" alt="License">
  <a href="https://github.com/wqm666/AI-personal-knows/stargazers"><img src="https://img.shields.io/github/stars/wqm666/AI-personal-knows?style=flat-square" alt="Stars"></a>
</p>

<h1 align="center">Personal Know</h1>

<p align="center">
  <strong>Build Your AI-Powered Digital Twin — Capture Every Knowledge Fragment, Never Forget Again</strong>
</p>

<p align="center">
  <a href="README_CN.md">中文文档</a> · <a href="https://github.com/wqm666/AI-personal-knows/issues">Report Bug</a> · <a href="https://github.com/wqm666/AI-personal-knows/issues">Request Feature</a>
</p>

---

## The Problem

We generate valuable knowledge every day — debugging sessions, architectural decisions, meeting insights, code patterns — but **99% of it evaporates**. It lives in chat histories, scattered notes, and our imperfect memory. When we need it most, it's gone.

Traditional note-taking tools expect you to organize knowledge manually. But the most valuable knowledge — the kind that comes from trial and error, from debugging at 2 AM, from that conversation with a senior engineer — is **implicit and unstructured**. It doesn't fit neatly into folders.

## The Vision: Your Digital Twin

**Personal Know** is an AI-native personal knowledge base that turns you into a digital twin — an always-online version of yourself that remembers everything you've learned.

```
You debugging at 2 AM    ──→  AI captures the insight
You reading a tech doc   ──→  AI extracts key knowledge
You solving a tricky bug ──→  AI remembers the solution

    3 months later, you (or your AI) face the same problem...
    → Personal Know instantly recalls the solution ✨
```

It's not just a note app. It's a **second brain** that:

- **Captures** knowledge fragments from AI conversations, documents, and manual input
- **Connects** related knowledge automatically through semantic understanding
- **Evolves** by consolidating fragments into structured insights over time
- **Serves** your AI assistant (Claude, Cursor, GPT) so it truly "knows" you

## Key Features

### Knowledge Quality System (Git-like Review)

Knowledge follows a **git-like branch model** — new items enter as `pending` (staging area) and must pass human review to become `approved` (main branch). Only approved knowledge is returned by search.

- **Human Review** — Every knowledge item requires manual approval before it becomes searchable
- **LLM-Assisted Suggestions** — LLM provides quality recommendations, but humans make the final decision
- **Confidence Scoring** — Each approved item carries a 0-100 confidence score
- **Web UI Review Panel** — Table view with status tabs (All / Pending / Approved / Rejected), inline approve/reject/edit

### Claude Code Skill System

4 built-in skills that teach AI agents how to use the knowledge base effectively:

| Skill | Trigger | What It Does |
|-------|---------|-------------|
| `/know-save` | "remember this", "save to knowledge base" | Extract and save knowledge from conversation |
| `/know-search` | **Auto-triggers** on decision/rule/architecture questions | Search knowledge base before answering |
| `/know-capture` | "extract knowledge from this conversation" | Batch-extract multiple knowledge items |
| `/know-review` | "review pending knowledge" | Interactive review queue with approve/reject |

**`/know-search` proactively triggers** — when users ask about past decisions, business rules, or architecture choices, the AI automatically searches the knowledge base before answering.

### MCP Observability

- **MCP Call Logs** — Track every `note_search` call from AI agents, with source filtering (MCP vs Web)
- **Knowledge Hit Ranking** — See which knowledge items are most frequently recalled
- **Bad Recall Marking** — Flag incorrect search results to improve knowledge quality over time
- **Accuracy Metrics** — Per-item accuracy rate based on feedback

### Knowledge Capture
- **Auto-Extraction from Conversations** — LLM detects valuable signals (errors, pitfalls, decisions) and extracts structured knowledge
- **Document Import** — Import Markdown/text files with auto-chunking by headings
- **Manual Input** — Quick-save knowledge via Web UI or API
- **MCP Integration** — AI agents directly save learnings through `note_save` and `note_capture`

### Knowledge Retrieval
- **Multi-Strategy Search** — Vector similarity + Full-text search + Keyword matching, running in parallel
- **Knowledge Graph Expansion** — BFS traversal along `related_ids` surfaces connected knowledge you didn't know you had
- **Smart Ranking** — Feedback-driven scoring: knowledge marked "useful" ranks higher

### Knowledge Evolution
- **Smart Deduplication** — Vector-based 3-tier strategy: reinforce existing / relate similar / create new
- **Auto Link Discovery** — Background task finds and connects semantically related knowledge
- **Knowledge Consolidation** — LLM merges 3+ related fragments into synthesis nodes
- **Tag Normalization** — LLM unifies synonymous tags (e.g., "golang" → "Go")
- **Natural Decay** — Unused knowledge gracefully fades after 90 days

### Integration
- **8 MCP Tools** — Full integration with Claude Code, Cursor, and any MCP-compatible AI
- **4 Claude Code Skills** — Pre-built skills that boost MCP adoption
- **REST API** — Complete CRUD + search + review + monitoring endpoints
- **Built-in Web UI** — Knowledge review, monitoring dashboard, tag cloud, maintenance panel
- **Docker One-Click Deploy** — Up and running in under 2 minutes

## Architecture

```
┌─────────────────────────────────────────────────┐
│                  AI Clients                      │
│  Claude Code / Cursor / ChatGPT / Any MCP Client│
└──────────────────────┬──────────────────────────┘
                       │ MCP (StreamableHTTP)
┌──────────────────────▼──────────────────────────┐
│                                                  │
│   ┌──────────┐  ┌───────────┐  ┌─────────────┐  │
│   │ MCP      │  │ REST API  │  │ Web UI      │  │
│   │ Server   │  │ /api/*    │  │ (embedded)  │  │
│   └────┬─────┘  └─────┬─────┘  └──────┬──────┘  │
│        └──────────────┼───────────────┘          │
│                       ▼                          │
│            ┌─────────────────────┐               │
│            │   Service Layer     │               │
│            │   (Business Logic)  │               │
│            └──────┬──────────────┘               │
│                   │                              │
│    ┌──────────────┼──────────────┐               │
│    ▼              ▼              ▼               │
│ ┌────────┐ ┌───────────────┐ ┌────────┐         │
│ │ Store  │ │ Orchestrator  │ │  LLM   │         │
│ │ pg +   │ │ Vector + FTS  │ │ Client │         │
│ │pgvector│ │ + Keyword     │ │(OpenAI)│         │
│ └────────┘ └───────────────┘ └────────┘         │
│                                                  │
│               Personal Know Server               │
└──────────────────────────────────────────────────┘
```

## MCP Tools

| Tool | Description |
|------|-------------|
| `note_search` | Semantic search across the knowledge base (only returns approved items) |
| `note_save` | Save knowledge with auto-dedup and linking (enters as pending) |
| `note_auto_capture` | Extract knowledge from conversation sessions via LLM |
| `note_import` | Import documents with auto-chunking |
| `note_update` | Update existing knowledge items |
| `note_review` | Review pending items: list / approve / reject / revision |
| `note_feedback` | Mark knowledge as useful (affects ranking and decay) |
| `note_maintain` | Trigger maintenance: link discovery, consolidation, decay, tag normalization |

## Quick Start

### Prerequisites

- Docker & Docker Compose
- An OpenAI-compatible API key (for embeddings and LLM)

### 1. Clone and configure

```bash
git clone https://github.com/wqm666/AI-personal-knows.git
cd AI-personal-knows

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

**Claude Code / Cursor / Any MCP Client:**

```json
{
  "mcpServers": {
    "personal-know": {
      "url": "http://localhost:8081/mcp"
    }
  }
}
```

Once connected, your AI assistant can:
- `note_save` — Save knowledge during conversations
- `note_search` — Recall relevant knowledge before writing code
- `note_capture` — Extract learnings from debugging sessions

## How It Works

### Knowledge Lifecycle

```
 ┌──────────┐    ┌──────────┐    ┌──────────┐    ┌──────────┐    ┌──────────┐
 │ Capture  │───▶│  Embed   │───▶│  Dedup   │───▶│ Pending  │───▶│ Approved │
 │          │    │          │    │          │    │ (staging) │    │  (main)  │
 │ • chat   │    │ OpenAI   │    │ ≥0.95 ↻  │    │          │    │          │
 │ • doc    │    │ embedding│    │ 0.75 🔗  │    │ Human    │    │ Searchable
 │ • manual │    │          │    │ <0.75 ✚  │    │ Review   │    │ by MCP   │
 └──────────┘    └──────────┘    └──────────┘    └──────────┘    └──────────┘
                                                                       │
                        ┌──────────────────────────────────────────────┘
                        ▼
 ┌──────────┐    ┌──────────┐    ┌──────────┐    ┌──────────┐
 │  Search  │───▶│  Expand  │───▶│ Monitor  │───▶│ Maintain │
 │          │    │          │    │          │    │          │
 │ 3-way    │    │ BFS graph│    │ MCP logs │    │ • links  │
 │ parallel │    │ traversal│    │ accuracy │    │ • merge  │
 │          │    │          │    │ feedback │    │ • decay  │
 └──────────┘    └──────────┘    └──────────┘    └──────────┘
```

### Multi-Strategy Search

1. **Parallel Retrieval** — Vector (cosine ≥ 0.7), Full-text (`tsvector`), Keyword (word frequency) run simultaneously
2. **Merge & Dedup** — Same item from multiple retrievers keeps the highest score
3. **BFS Graph Expansion** — Follow `related_ids` up to 2 layers deep (max 100 items)
4. **Synthesis Resolve** — Expand `consolidated_from` for synthesis nodes

### Smart Deduplication

| Similarity | Action | Behavior |
|-----------|--------|----------|
| ≥ 0.95 | **Reinforce** | Skip save, increment `hit_count` on existing |
| 0.75–0.95 | **Relate** | Save new + bidirectional link in transaction |
| < 0.75 | **New** | Save as independent item |

### Background Maintenance

| Task | What It Does |
|------|-------------|
| `link_discovery` | Scan all items, connect those with similarity 0.75–0.95 |
| `consolidation` | Merge 3+ related items into a synthesis node via LLM |
| `tag_cluster` | Normalize synonymous tags via LLM (e.g., "golang" → "Go") |
| `decay` | Mark items as decayed after 90 days with no hits |

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

### Environment Variables

All fields can be overridden:

| Variable | Description |
|---------|-------------|
| `DATABASE_URL` | PostgreSQL connection string |
| `LLM_BASE_URL` | LLM API base URL |
| `LLM_API_KEY` | LLM API key |
| `LLM_CHAT_MODEL` | Chat model name |
| `LLM_EMBEDDING_MODEL` | Embedding model name |
| `SERVER_ADDR` | Listen address (default `:8081`) |
| `SERVER_API_KEY` | Optional API key for authentication |
| `CORS_ORIGINS` | Comma-separated allowed origins |

## REST API

| Method | Endpoint | Description |
|--------|----------|-------------|
| GET | `/api/knowledge` | List knowledge (paginated) |
| POST | `/api/knowledge` | Create knowledge item (enters as pending) |
| GET | `/api/knowledge/:id` | Get single item |
| PUT | `/api/knowledge/:id` | Update item |
| DELETE | `/api/knowledge/:id` | Delete item |
| GET/POST | `/api/search` | Search knowledge base (approved items only) |
| GET/POST | `/api/review` | List pending / approve / reject / revision |
| POST | `/api/import` | Import document |
| POST | `/api/capture` | Capture session knowledge |
| POST | `/api/feedback` | Record useful feedback |
| GET/POST | `/api/maintain` | List or run maintenance tasks |
| GET | `/api/stats` | Knowledge base statistics (incl. review counts) |
| GET | `/api/search_log` | Search query analytics |
| GET | `/api/monitor/ranking` | Knowledge hit ranking with accuracy |
| GET | `/api/monitor/logs` | MCP call logs (filterable by source) |
| POST | `/api/monitor/bad_recall` | Mark a search result as incorrect |

## Tech Stack

| Component | Technology |
|-----------|-----------|
| Language | Go 1.23 |
| Database | PostgreSQL 16 + pgvector (HNSW, cosine) |
| MCP | [mcp-go](https://github.com/mark3labs/mcp-go) (StreamableHTTP) |
| LLM | OpenAI-compatible API |
| Frontend | Embedded vanilla JS SPA |
| Deploy | Docker Compose |

## Project Structure

```
.
├── cmd/server/            # Entry point & dependency wiring
├── internal/
│   ├── domain/            # Domain models (Knowledge, SearchHit, ReviewResult, etc.)
│   ├── port/              # Interface definitions (Store, Retriever, Embedder, etc.)
│   ├── service/           # Business logic orchestration
│   └── adapter/
│       ├── api/           # REST API router (incl. review & monitoring)
│       ├── transport/     # MCP server & tool handlers
│       ├── store/         # PostgreSQL + pgvector implementation
│       ├── retriever/     # Vector, FTS, Keyword + orchestrator
│       ├── reviewer/      # LLM-assisted review suggestions
│       ├── embedder/      # OpenAI-compatible embedding client
│       ├── llm/           # OpenAI-compatible chat client
│       ├── dedup/         # Vector similarity deduplication
│       ├── maintain/      # Link discovery, consolidation, decay, tags
│       └── identity/      # Owner identity provider
├── skills/                # Claude Code skill definitions
│   ├── know-save/         # Save knowledge from conversation
│   ├── know-search/       # Search knowledge base (auto-triggers)
│   ├── know-capture/      # Batch extract knowledge from dialogue
│   └── know-review/       # Interactive review queue
├── web/                   # Embedded static web UI
├── config.json.example
├── docker-compose.yml
├── Dockerfile
└── deploy.sh
```

## Roadmap

- [x] Multi-strategy retrieval (Vector + FTS + Keyword)
- [x] Smart deduplication with 3-tier strategy
- [x] Auto knowledge extraction from conversations
- [x] Knowledge graph with BFS expansion
- [x] Background maintenance (link discovery, consolidation, decay, tag clustering)
- [x] Built-in Web UI
- [x] **Knowledge Quality System** — Git-like review flow (pending → approved → searchable)
- [x] **Claude Code Skills** — 4 skills to boost MCP adoption (`/know-save`, `/know-search`, `/know-capture`, `/know-review`)
- [x] **MCP Observability** — Call logs, hit ranking, bad recall marking, accuracy metrics
- [ ] Multi-user support with authentication
- [ ] Browser extension for one-click knowledge capture
- [ ] Mobile app (iOS / Android)
- [ ] Import from Notion / Obsidian / Logseq
- [ ] Scheduled auto-maintenance
- [ ] Knowledge sharing between personal and team knowledge bases
- [ ] Plugin system for custom knowledge sources

## Contributing

Contributions are welcome! Whether it's bug reports, feature requests, or pull requests.

1. Fork the repository
2. Create your feature branch (`git checkout -b feature/amazing-feature`)
3. Commit your changes (`git commit -m 'feat: add amazing feature'`)
4. Push to the branch (`git push origin feature/amazing-feature`)
5. Open a Pull Request

## Related Projects

- [AI-team-know](https://github.com/wqm666/AI-team-know) — Team knowledge base for shared team intelligence
- [mcp-go](https://github.com/mark3labs/mcp-go) — Go implementation of MCP protocol

## Star History

<p align="center">
  <a href="https://github.com/wqm666/AI-personal-knows/stargazers">
    <img src="https://starchart.cc/wqm666/AI-personal-knows.svg?variant=adaptive" alt="Star History Chart" width="600">
  </a>
</p>

## License

[MIT](LICENSE) — Use it, fork it, build on it.
