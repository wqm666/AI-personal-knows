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
		mcp.WithDescription("Save a knowledge note. AI should provide title and tags. If new content contradicts existing knowledge, the old item is automatically marked as superseded (newest-wins)."),
		mcp.WithString("content", mcp.Required(), mcp.Description("Knowledge content")),
		mcp.WithString("title", mcp.Description("Title for the knowledge")),
		mcp.WithString("tags", mcp.Description("Comma-separated tags")),
		mcp.WithString("source", mcp.Description("Source: conversation / document / manual")),
		mcp.WithString("source_ref", mcp.Description("Source reference")),
		mcp.WithString("knowledge_type", mcp.Description("Knowledge type: pitfall / decision / faq / general")),
	), newSaveHandler(svc, identity))

	s.AddTool(mcp.NewTool("note_search",
		mcp.WithDescription("Search knowledge base by semantic query. Results include freshness annotation (latest/outdated) and conflict_group for contradictory items. Superseded items are shown for context but newest knowledge is prioritized."),
		mcp.WithString("query", mcp.Required(), mcp.Description("Search query text")),
		mcp.WithNumber("limit", mcp.Description("Max results to return, default 5")),
	), newSearchHandler(svc, identity))

	s.AddTool(mcp.NewTool("note_import",
		mcp.WithDescription("Import a document into knowledge base"),
		mcp.WithString("file_content", mcp.Required(), mcp.Description("Document text content")),
		mcp.WithString("file_name", mcp.Required(), mcp.Description("File name")),
		mcp.WithString("chunk_mode", mcp.Description("single or auto (default: auto)")),
	), newImportHandler(svc, identity))

	s.AddTool(mcp.NewTool("note_capture",
		mcp.WithDescription("Capture knowledge from a conversation session"),
		mcp.WithString("session_summary", mcp.Required(), mcp.Description("Summary of the conversation")),
		mcp.WithString("items_json", mcp.Description("JSON array of extracted knowledge items")),
	), newCaptureHandler(svc, identity))

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

	s.AddTool(mcp.NewTool("note_maintain",
		mcp.WithDescription("Trigger background knowledge maintenance tasks: link_discovery, consolidation, decay, tag_cluster. Run all if no tasks specified."),
		mcp.WithString("tasks", mcp.Description("Comma-separated task names to run. Empty = run all.")),
	), newMaintainHandler(svc, identity))
}
