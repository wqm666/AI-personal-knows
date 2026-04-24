package mcp

import (
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
	"github.com/personal-know/internal/port"
	"github.com/personal-know/internal/service"
)

func NewServer(svc *service.Service, identity port.IdentityProvider) *server.MCPServer {
	s := server.NewMCPServer(
		"personal-know",
		"1.0.0",
		server.WithResourceCapabilities(true, true),
	)

	registerTools(s, svc, identity)
	return s
}

func registerTools(s *server.MCPServer, svc *service.Service, identity port.IdentityProvider) {
	s.AddTool(mcp.NewTool("note_save",
		mcp.WithDescription(`Save a single, well-structured knowledge note. Use this for manually crafted knowledge with a clear title and tags. If the content contradicts existing knowledge, the old item is automatically superseded (newest-wins). For automatic extraction from conversations, use note_auto_capture instead.`),
		mcp.WithString("content", mcp.Required(), mcp.Description("Knowledge content")),
		mcp.WithString("title", mcp.Description("Title for the knowledge")),
		mcp.WithString("tags", mcp.Description("Comma-separated tags")),
		mcp.WithString("source", mcp.Description("Source: conversation / document / manual")),
		mcp.WithString("source_ref", mcp.Description("Source reference")),
		mcp.WithString("knowledge_type", mcp.Description("Knowledge type: pitfall / decision / faq / general")),
	), newSaveHandler(svc, identity))

	s.AddTool(mcp.NewTool("note_search",
		mcp.WithDescription(`Search the user's personal knowledge base. Use this BEFORE answering questions to check if the user has existing knowledge, preferences, or past decisions on the topic. Results include freshness annotation (latest/outdated) and conflict_group for contradictory items. Superseded items are shown for context but newest knowledge is prioritized.`),
		mcp.WithString("query", mcp.Required(), mcp.Description("Search query text")),
		mcp.WithNumber("limit", mcp.Description("Max results to return, default 5")),
	), newSearchHandler(svc, identity))

	s.AddTool(mcp.NewTool("note_import",
		mcp.WithDescription("Import a document into knowledge base"),
		mcp.WithString("file_content", mcp.Required(), mcp.Description("Document text content")),
		mcp.WithString("file_name", mcp.Required(), mcp.Description("File name")),
		mcp.WithString("chunk_mode", mcp.Description("single or auto (default: auto)")),
	), newImportHandler(svc, identity))

	s.AddTool(mcp.NewTool("note_feedback",
		mcp.WithDescription("Mark a knowledge item as useful"),
		mcp.WithString("item_id", mcp.Required(), mcp.Description("Knowledge item ID")),
	), newFeedbackHandler(svc, identity))

	s.AddTool(mcp.NewTool("note_update",
		mcp.WithDescription("Update an existing knowledge note. Only provided fields will be updated."),
		mcp.WithString("id", mcp.Required(), mcp.Description("Knowledge item ID to update")),
		mcp.WithString("title", mcp.Description("New title")),
		mcp.WithString("content", mcp.Description("New content")),
		mcp.WithString("tags", mcp.Description("New comma-separated tags (replaces existing)")),
		mcp.WithString("knowledge_type", mcp.Description("Knowledge type: pitfall / decision / faq / general")),
	), newUpdateHandler(svc, identity))

	s.AddTool(mcp.NewTool("note_auto_capture",
		mcp.WithDescription(`Capture knowledge from the conversation into the user's digital twin. Call this tool proactively — the user expects all valuable knowledge to be remembered automatically without being asked.

Call when the conversation contains: decisions with reasoning, business rules, lessons learned, architecture discussions, personal preferences, corrections, workflow descriptions, or any domain knowledge worth remembering.

Lightweight: runs in background, auto-filters low-value content, auto-deduplicates. Call per topic — multiple calls per conversation is normal and expected.`),
		mcp.WithString("conversation", mcp.Required(), mcp.Description("The conversation fragment to extract knowledge from. Include enough context to be understood independently.")),
		mcp.WithString("project_context", mcp.Description("Current project or business context for better extraction")),
	), newAutoCaptureHandler(svc, identity))

	s.AddTool(mcp.NewTool("note_maintain",
		mcp.WithDescription("Trigger background knowledge maintenance tasks: link_discovery, consolidation, decay, tag_cluster. Run all if no tasks specified."),
		mcp.WithString("tasks", mcp.Description("Comma-separated task names to run. Empty = run all.")),
	), newMaintainHandler(svc, identity))
}
