<p align="center">
  <img src="https://img.shields.io/badge/Go-1.23-00ADD8?style=flat-square&logo=go" alt="Go">
  <img src="https://img.shields.io/badge/PostgreSQL-16+pgvector-4169E1?style=flat-square&logo=postgresql" alt="PostgreSQL">
  <img src="https://img.shields.io/badge/MCP-StreamableHTTP-8A2BE2?style=flat-square" alt="MCP">
  <img src="https://img.shields.io/badge/License-MIT-green?style=flat-square" alt="License">
  <a href="https://github.com/wqm666/AI-personal-knows/stargazers"><img src="https://img.shields.io/github/stars/wqm666/AI-personal-knows?style=flat-square" alt="Stars"></a>
</p>

<h1 align="center">Personal Know</h1>

<p align="center">
  <strong>构建你的 AI 数字分身 —— 收集每一片知识碎片，让 AI 真正「懂」你</strong>
</p>

<p align="center">
  <a href="README.md">English</a> · <a href="https://github.com/wqm666/AI-personal-knows/issues">报告 Bug</a> · <a href="https://github.com/wqm666/AI-personal-knows/issues">功能建议</a>
</p>

---

## 为什么需要个人知识库？

我们每天都在产生有价值的知识 —— 调试过程中的踩坑经验、架构决策的思考、技术方案的选型理由、和同事讨论时的灵感火花 —— 但**99% 的知识都蒸发了**。它们散落在聊天记录、临时笔记和我们不可靠的记忆中。等到真正需要时，早已找不到了。

传统笔记工具要求你手动整理知识。但最有价值的知识 —— 凌晨两点调试时的领悟、和资深工程师交流时的经验、反复试错后才发现的坑 —— 往往是**隐性的、非结构化的**，根本塞不进整齐的文件夹。

> 当 AI 成为你最大的「助手」，所有隐性知识都必须变成显性资产。AI 只能消费被明确外化的知识 —— 你脑子里知道但没写下来的东西，对 AI 来说等于不存在。

## 愿景：你的数字分身

**Personal Know** 是一个 AI 原生的个人知识库，它的目标是帮你构建一个**数字分身** —— 一个永远在线、记住你所有知识的「另一个你」。

```
你凌晨 2 点调试一个 bug     ──→  AI 自动捕获解决方案
你读完一篇技术文档          ──→  AI 提取关键知识点
你在对话中分享了一个经验     ──→  AI 记住并归类

    三个月后，你（或你的 AI 助手）再次遇到同样的问题……
    → Personal Know 瞬间召回当时的解决方案 ✨
```

它不只是一个笔记应用，而是一个会**自我进化的第二大脑**：

- **收集** —— 从 AI 对话、文档、手动输入中捕获知识碎片
- **连接** —— 通过语义理解自动发现知识之间的关联
- **进化** —— 将碎片知识聚合为结构化的洞察
- **服务** —— 让你的 AI 助手（Claude、Cursor、GPT）真正「懂」你

## 核心能力

### 知识收集

- **对话自动提取** —— LLM 检测有价值的信号（报错、踩坑、决策），从对话中自动提取结构化知识
- **文档导入** —— 支持 Markdown/文本文件导入，按标题自动分块
- **手动录入** —— 通过 Web UI 或 API 快速保存知识
- **MCP 原生集成** —— AI 智能体通过 `note_save` 和 `note_capture` 直接保存学到的知识

### 知识检索

- **三路并行搜索** —— 向量相似度 + 全文搜索 + 关键词匹配，同时执行，取最优结果
- **知识图谱扩展** —— 沿 `related_ids` 进行 BFS 遍历，发现你不知道自己知道的知识
- **反馈驱动排序** —— 被标记「有用」的知识排名更高，形成正向飞轮

### 知识进化

- **智能去重** —— 基于向量相似度的三级策略：强化已有 / 关联相似 / 创建新条目
- **自动关联发现** —— 后台任务扫描全库，发现并连接语义相关的知识
- **知识聚合** —— LLM 将 3+ 个相关碎片合并为综合索引节点
- **标签归一化** —— LLM 统一同义标签（如 "golang" → "Go"）
- **自然衰减** —— 90 天无人问津的知识自动降权，保持知识库新鲜

### 无缝集成

- **7 个 MCP 工具** —— 完整对接 Claude Code、Cursor 及所有 MCP 兼容客户端
- **REST API** —— 完整的 CRUD + 搜索 + 分析接口
- **内置 Web UI** —— 浏览、搜索、管理知识，无需离开浏览器
- **Docker 一键部署** —— 2 分钟内完成部署

## 架构概览

```
┌─────────────────────────────────────────────────┐
│                   AI 客户端                       │
│  Claude Code / Cursor / ChatGPT / 任何 MCP 客户端 │
└──────────────────────┬──────────────────────────┘
                       │ MCP (StreamableHTTP)
┌──────────────────────▼──────────────────────────┐
│                                                  │
│   ┌──────────┐  ┌───────────┐  ┌─────────────┐  │
│   │ MCP      │  │ REST API  │  │ Web UI      │  │
│   │ Server   │  │ /api/*    │  │ (嵌入式)     │  │
│   └────┬─────┘  └─────┬─────┘  └──────┬──────┘  │
│        └──────────────┼───────────────┘          │
│                       ▼                          │
│            ┌─────────────────────┐               │
│            │       服务层         │               │
│            │     (业务逻辑)       │               │
│            └──────┬──────────────┘               │
│                   │                              │
│    ┌──────────────┼──────────────┐               │
│    ▼              ▼              ▼               │
│ ┌────────┐ ┌───────────────┐ ┌────────┐         │
│ │  存储   │ │   检索编排器    │ │  LLM   │         │
│ │ pg +   │ │ 向量 + 全文    │ │  客户端  │         │
│ │pgvector│ │ + 关键词       │ │(OpenAI)│         │
│ └────────┘ └───────────────┘ └────────┘         │
│                                                  │
│              Personal Know Server                │
└──────────────────────────────────────────────────┘
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
git clone https://github.com/wqm666/AI-personal-knows.git
cd AI-personal-knows

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

**Claude Code / Cursor / 任何 MCP 客户端：**

```json
{
  "mcpServers": {
    "personal-know": {
      "url": "http://localhost:8081/mcp"
    }
  }
}
```

连接后，你的 AI 助手就能：
- `note_save` —— 在对话中随时保存知识
- `note_search` —— 在写代码前自动召回相关知识
- `note_capture` —— 从调试会话中提取踩坑经验

## 工作原理

### 知识生命周期

```
 ┌──────────┐    ┌──────────┐    ┌──────────┐    ┌──────────┐
 │   收集    │───▶│  向量化   │───▶│   去重    │───▶│   存储    │
 │          │    │          │    │          │    │          │
 │ • 对话   │    │  OpenAI  │    │ ≥0.95 ↻  │    │ pgvector │
 │ • 文档   │    │ embedding│    │ 0.75 🔗  │    │ + 元数据  │
 │ • 手动   │    │          │    │ <0.75 ✚  │    │          │
 └──────────┘    └──────────┘    └──────────┘    └──────────┘
                                                       │
                        ┌──────────────────────────────┘
                        ▼
 ┌──────────┐    ┌──────────┐    ┌──────────┐
 │   检索    │───▶│   扩展    │───▶│   维护    │
 │          │    │          │    │          │
 │  三路     │    │ BFS 图   │    │ • 关联   │
 │  并行     │    │  遍历    │    │ • 聚合   │
 │          │    │          │    │ • 衰减   │
 └──────────┘    └──────────┘    └──────────┘
```

### 搜索流水线

1. **三路并行检索** —— 向量（余弦 ≥ 0.7）、全文搜索（`tsvector`）、关键词（词频）同时执行
2. **合并去重** —— 多路命中同一条目时保留最高分
3. **BFS 图扩展** —— 沿 `related_ids` 扩展最多 2 层（上限 100 条）
4. **综合节点展开** —— 展开 synthesis 节点的 `consolidated_from` 源条目

### 智能去重策略

| 相似度 | 动作 | 行为 |
|-------|------|------|
| ≥ 0.95 | **强化** | 不保存，对已有条目 `hit_count++` |
| 0.75–0.95 | **关联** | 保存新条目 + 事务内建立双向关联 |
| < 0.75 | **新建** | 保存为独立条目 |

### 后台维护任务

| 任务 | 说明 |
|------|------|
| `link_discovery` | 扫描全库，发现相似度 0.75–0.95 的条目并建立关联 |
| `consolidation` | 将 3+ 个相关条目通过 LLM 聚合为综合索引节点 |
| `tag_cluster` | 通过 LLM 归一化同义标签（如 "golang" → "Go"） |
| `decay` | 90 天无访问且无反馈的条目标记为衰减 |

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

### 环境变量

所有配置均可通过环境变量覆盖：

| 变量 | 说明 |
|------|------|
| `DATABASE_URL` | PostgreSQL 连接字符串 |
| `LLM_BASE_URL` | LLM API 基础地址 |
| `LLM_API_KEY` | LLM API 密钥 |
| `LLM_CHAT_MODEL` | 对话模型名称 |
| `LLM_EMBEDDING_MODEL` | 向量模型名称 |
| `SERVER_ADDR` | 监听地址（默认 `:8081`） |
| `SERVER_API_KEY` | 可选的 API Key 认证 |
| `CORS_ORIGINS` | 逗号分隔的允许跨域来源 |

## REST API

| 方法 | 端点 | 说明 |
|------|------|------|
| GET | `/api/knowledge` | 知识列表（分页） |
| POST | `/api/knowledge` | 创建知识条目 |
| GET | `/api/knowledge/:id` | 获取单个条目 |
| PUT | `/api/knowledge/:id` | 更新条目 |
| DELETE | `/api/knowledge/:id` | 删除条目 |
| GET/POST | `/api/search` | 搜索知识库 |
| POST | `/api/import` | 导入文档 |
| POST | `/api/capture` | 捕获会话知识 |
| POST | `/api/feedback` | 记录有用反馈 |
| GET/POST | `/api/maintain` | 查看或执行维护任务 |
| GET | `/api/stats` | 知识库统计 |
| GET | `/api/search_log` | 搜索日志分析 |

## 技术栈

| 组件 | 技术 |
|------|------|
| 语言 | Go 1.23 |
| 数据库 | PostgreSQL 16 + pgvector（HNSW 索引，余弦距离） |
| MCP | [mcp-go](https://github.com/mark3labs/mcp-go)（StreamableHTTP 传输） |
| LLM | OpenAI 兼容 API（Embedding + Chat） |
| 前端 | 嵌入式原生 JS 单页应用 |
| 部署 | Docker Compose |

## 项目结构

```
.
├── cmd/server/            # 入口 & 依赖组装
├── internal/
│   ├── domain/            # 领域模型（Knowledge、SearchHit 等）
│   ├── port/              # 接口定义（Store、Retriever、Embedder 等）
│   ├── service/           # 业务逻辑编排
│   └── adapter/
│       ├── api/           # REST API 路由
│       ├── transport/     # MCP 服务器 & 工具处理器
│       ├── store/         # PostgreSQL + pgvector 实现
│       ├── retriever/     # 向量、全文、关键词检索器 + 编排器
│       ├── embedder/      # OpenAI 兼容向量编码客户端
│       ├── llm/           # OpenAI 兼容对话客户端
│       ├── dedup/         # 向量相似度去重
│       ├── maintain/      # 关联发现、聚合、衰减、标签归一化
│       └── identity/      # 身份提供者
├── web/                   # 嵌入式静态 Web UI
├── config.json.example
├── docker-compose.yml
├── Dockerfile
└── deploy.sh
```

## 路线图

- [x] 多策略检索（向量 + 全文 + 关键词）
- [x] 智能去重（三级策略）
- [x] 对话知识自动提取
- [x] 知识图谱 + BFS 扩展
- [x] 后台维护（关联发现、聚合、衰减、标签归一化）
- [x] 内置 Web UI
- [ ] 多用户支持 + 身份认证
- [ ] 浏览器插件：一键捕获网页知识
- [ ] 移动端 App（iOS / Android）
- [ ] 导入 Notion / Obsidian / Logseq
- [ ] 定时自动维护
- [ ] 个人知识库与团队知识库互通共享
- [ ] 自定义知识源插件系统

## 参与贡献

欢迎各种形式的贡献！无论是 Bug 报告、功能建议还是 Pull Request。

1. Fork 本仓库
2. 创建特性分支 (`git checkout -b feature/amazing-feature`)
3. 提交更改 (`git commit -m 'feat: add amazing feature'`)
4. 推送分支 (`git push origin feature/amazing-feature`)
5. 提交 Pull Request

## 相关项目

- [AI-team-know](https://github.com/wqm666/AI-team-know) —— 团队知识库，沉淀团队共同智慧
- [mcp-go](https://github.com/mark3labs/mcp-go) —— MCP 协议 Go 实现

## Star History

<p align="center">
  <a href="https://github.com/wqm666/AI-personal-knows/stargazers">
    <img src="https://starchart.cc/wqm666/AI-personal-knows.svg?variant=adaptive" alt="Star History Chart" width="600">
  </a>
</p>

## 许可证

[MIT](LICENSE) —— 自由使用、Fork、二次开发。
