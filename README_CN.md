# Personal Know

> AI 原生的个人知识库，集成 MCP 协议 —— 让你的 AI 助手跨对话地记忆、检索和进化知识。

Personal Know 是一个自托管的知识管理服务，同时暴露 **MCP (Model Context Protocol)** 工具和 **REST API**。AI 智能体（Claude Code、Cursor、GPT 等）通过 MCP 存储、搜索和维护知识；人类用户则通过内置 Web UI 管理知识。

## 特性

- **MCP 协议支持** —— 7 个工具，通过 StreamableHTTP 供 AI 智能体调用
- **多策略检索** —— 向量相似度 + 全文搜索 + 关键词匹配，三路并行
- **智能去重** —— 基于向量相似度的三级策略：强化 / 关联 / 新建
- **自动知识提取** —— LLM 检测信号（错误、踩坑），从对话中自动提取结构化知识
- **知识图谱扩展** —— 沿 `related_ids` 进行 BFS 遍历，关联浮现相关知识
- **自维护** —— 后台任务自动执行关联发现、聚合、衰减、标签归一化
- **内置 Web UI** —— 嵌入式单页应用，支持浏览、搜索和管理知识
- **六边形架构** —— 端口与适配器清晰分离，易于扩展

## 架构概览

```
┌─────────────────┐     ┌─────────────────┐
│   AI 智能体      │     │   Web 浏览器     │
│ (Claude/Cursor)  │     │   (内置 UI)      │
└────────┬─────────┘     └────────┬─────────┘
         │ MCP (StreamableHTTP)    │ REST API
         └──────────┬──────────────┘
                    │
         ┌──────────▼──────────┐
         │  HTTP 服务器 :8081    │
         │  CORS + API Key 认证 │
         └──────────┬──────────┘
                    │
         ┌──────────▼──────────┐
         │      服务层          │
         │    (业务逻辑)        │
         └──────────┬──────────┘
                    │
    ┌───────────────┼───────────────┐
    │               │               │
┌───▼───┐   ┌──────▼──────┐   ┌───▼────┐
│ 存储层 │   │  检索编排器   │   │  LLM   │
│(pgvec) │   │ (3路检索器)  │   │  客户端 │
└────────┘   └─────────────┘   └────────┘
```

## MCP 工具

| 工具 | 说明 |
|------|------|
| `note_search` | 语义搜索知识库 |
| `note_save` | 保存知识，自动去重和关联 |
| `note_capture` | 通过 LLM 从对话会话中提取知识 |
| `note_import` | 导入文档，按 Markdown 标题自动分块 |
| `note_update` | 更新已有知识条目 |
| `note_feedback` | 标记知识有用（影响排名和衰减） |
| `note_maintain` | 触发维护任务：关联发现、聚合、衰减、标签归一化 |

## 快速开始

### 前置条件

- Docker & Docker Compose
- 一个 OpenAI 兼容的 API Key（用于 Embedding 和 LLM）

### 1. 克隆并配置

```bash
git clone https://github.com/wqm666/AI-knows.git
cd AI-knows

# 配置密钥
cp .env.example .env
cp config.json.example config.json

# 编辑 .env —— 设置 POSTGRES_PASSWORD
# 编辑 config.json —— 设置你的 LLM API Key 和端点
```

### 2. 部署

```bash
chmod +x deploy.sh
./deploy.sh
```

### 3. 验证

```bash
curl http://localhost:8081/health
# {"status":"ok"}
```

- **Web 界面**：http://localhost:8081
- **MCP 端点**：http://localhost:8081/mcp
- **REST API**：http://localhost:8081/api/*

### 4. 连接你的 AI 智能体

在 MCP 客户端配置中添加：

```json
{
  "mcpServers": {
    "personal-know": {
      "url": "http://localhost:8081/mcp"
    }
  }
}
```

## 配置说明

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

所有字段均可通过环境变量覆盖：

| 环境变量 | 说明 |
|---------|------|
| `DATABASE_URL` | PostgreSQL 连接字符串 |
| `LLM_BASE_URL` | LLM API 基础地址 |
| `LLM_API_KEY` | LLM API 密钥 |
| `LLM_CHAT_MODEL` | 对话模型名称 |
| `LLM_EMBEDDING_MODEL` | 向量模型名称 |
| `SERVER_ADDR` | 监听地址（默认 `:8081`） |
| `SERVER_API_KEY` | 可选的 API Key 认证 |
| `CORS_ORIGINS` | 逗号分隔的允许跨域来源 |

## 工作原理

### 知识生命周期

```
 产生 ──→ 向量化 ──→ 去重 ──→ 存储 ──→ 检索 ──→ 维护
  │                   │                   │        │
  ├─ 对话捕获         ├─ ≥0.95: 强化      │        ├─ 关联发现
  ├─ 文档导入         ├─ 0.75~0.95: 关联  │        ├─ 聚合
  └─ 手动保存         └─ <0.75: 新建      │        ├─ 标签归一化
                                          │        └─ 衰减 (90天)
                                          │
                                     多策略检索
                                     + BFS 图扩展
```

### 搜索流水线

1. **多策略检索** —— 向量（余弦 ≥ 0.7）、全文搜索（`tsvector`）、关键词（词频）三路并行
2. **合并去重** —— 多路检索命中同一条目时保留最高分
3. **BFS 图扩展** —— 沿 `related_ids` 扩展最多 2 层（上限 100 条）
4. **综合节点展开** —— 展开 synthesis 节点的 `consolidated_from` 源条目

### 去重策略（保存时）

| 相似度 | 动作 | 行为 |
|-------|------|------|
| ≥ 0.95 | **强化** | 不保存，对已有条目 `hit_count++` |
| 0.75–0.95 | **关联** | 保存新条目 + 事务内建立双向关联 |
| < 0.75 | **新建** | 保存为独立条目 |

### 维护任务

| 任务 | 说明 |
|------|------|
| `link_discovery` | 发现相似度 0.75–0.95 的条目并建立关联 |
| `consolidation` | 将 3+ 相关条目通过 LLM 聚合为综合索引节点 |
| `tag_cluster` | 通过 LLM 归一化同义标签（如 "golang" → "Go"） |
| `decay` | 90 天无访问且无反馈的条目标记为衰减 |

## REST API

| 方法 | 端点 | 说明 |
|------|------|------|
| GET | `/api/knowledge` | 知识列表（分页） |
| POST | `/api/knowledge` | 创建知识条目 |
| GET | `/api/knowledge/:id` | 获取单个条目 |
| PUT | `/api/knowledge/:id` | 更新条目 |
| DELETE | `/api/knowledge/:id` | 删除条目 |
| GET/POST | `/api/search` | 搜索知识库 |
| POST | `/api/import` | 导入文档（multipart 或 JSON） |
| POST | `/api/capture` | 捕获会话知识 |
| POST | `/api/feedback` | 记录有用反馈 |
| GET/POST | `/api/maintain` | 查看或执行维护任务 |
| GET | `/api/stats` | 知识库统计 |
| GET | `/api/search_log` | 搜索日志分析 |

## 技术栈

- **语言**：Go 1.23
- **数据库**：PostgreSQL 16 + pgvector（HNSW 索引，余弦距离）
- **MCP**：[mcp-go](https://github.com/mark3labs/mcp-go)（StreamableHTTP 传输）
- **LLM**：OpenAI 兼容 API（Embedding + Chat）
- **前端**：嵌入式原生 JS 单页应用
- **部署**：Docker Compose

## 项目结构

```
.
├── cmd/server/          # 入口 & 依赖组装
├── internal/
│   ├── domain/          # 领域模型（Knowledge、SearchHit 等）
│   ├── port/            # 接口定义（Store、Retriever、Embedder 等）
│   ├── service/         # 业务逻辑编排
│   └── adapter/
│       ├── api/         # REST API 路由
│       ├── transport/   # MCP 服务器 & 工具处理器
│       ├── store/       # PostgreSQL + pgvector 实现
│       ├── retriever/   # 向量、全文、关键词检索器 + 编排器
│       ├── embedder/    # OpenAI 兼容向量编码客户端
│       ├── llm/         # OpenAI 兼容对话客户端
│       ├── dedup/       # 向量相似度去重
│       ├── maintain/    # 关联发现、聚合、衰减、标签归一化
│       └── identity/    # 身份提供者
├── web/                 # 嵌入式静态 Web UI
├── config.json.example  # 配置模板
├── docker-compose.yml   # 容器编排
├── Dockerfile           # 多阶段构建
└── deploy.sh            # 一键部署脚本
```

## 许可证

[MIT](LICENSE)
