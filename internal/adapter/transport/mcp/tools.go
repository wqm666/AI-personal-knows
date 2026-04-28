package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/personal-know/internal/domain"
	"github.com/personal-know/internal/port"
	"github.com/personal-know/internal/service"
)

func mustIdentity(ctx context.Context, identity port.IdentityProvider) (context.Context, *mcp.CallToolResult) {
	ctx, err := injectIdentity(ctx, identity)
	if err != nil {
		return ctx, errorResult(err.Error())
	}
	return ctx, nil
}

func injectIdentity(ctx context.Context, identity port.IdentityProvider) (context.Context, error) {
	id, err := identity.Resolve(ctx)
	if err != nil {
		return ctx, fmt.Errorf("identity resolution failed: %w", err)
	}
	return port.ContextWithIdentity(ctx, id), nil
}

func newSaveHandler(svc *service.Service, identity port.IdentityProvider) func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		ctx, errResult := mustIdentity(ctx, identity)
		if errResult != nil {
			return errResult, nil
		}
		args := req.GetArguments()

		content, _ := args["content"].(string)
		title, _ := args["title"].(string)
		tagsStr, _ := args["tags"].(string)
		source, _ := args["source"].(string)
		sourceRef, _ := args["source_ref"].(string)
		knowledgeType, _ := args["knowledge_type"].(string)

		if content == "" {
			return errorResult("content is required"), nil
		}

		tags := splitComma(tagsStr)

		result, err := svc.Save(ctx, title, content, source, sourceRef, tags)
		if err != nil {
			return errorResult(err.Error()), nil
		}

		if knowledgeType != "" && result.Saved {
			if _, err := svc.UpdateKnowledge(ctx, result.ID, "", "", nil, knowledgeType); err != nil {
				return errorResult("saved but failed to set knowledge_type: " + err.Error()), nil
			}
		}

		return jsonResultWithHint(result, "Tip: For conversations, use note_auto_capture instead — it automatically extracts and categorizes knowledge from raw dialogue.")
	}
}

func newUpdateHandler(svc *service.Service, identity port.IdentityProvider) func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		ctx, errResult := mustIdentity(ctx, identity)
		if errResult != nil {
			return errResult, nil
		}
		args := req.GetArguments()

		id, _ := args["id"].(string)
		title, _ := args["title"].(string)
		content, _ := args["content"].(string)
		tagsStr, _ := args["tags"].(string)
		knowledgeType, _ := args["knowledge_type"].(string)

		if id == "" {
			return errorResult("id is required"), nil
		}

		var tags []string
		if tagsStr != "" {
			tags = splitComma(tagsStr)
		}

		result, err := svc.UpdateKnowledge(ctx, id, title, content, tags, knowledgeType)
		if err != nil {
			return errorResult(err.Error()), nil
		}

		return jsonResult(result)
	}
}

func splitComma(s string) []string {
	if s == "" {
		return nil
	}
	var result []string
	for _, t := range strings.Split(s, ",") {
		t = strings.TrimSpace(t)
		if t != "" {
			result = append(result, t)
		}
	}
	return result
}

func newSearchHandler(svc *service.Service, identity port.IdentityProvider) func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		ctx, errResult := mustIdentity(ctx, identity)
		if errResult != nil {
			return errResult, nil
		}
		args := req.GetArguments()

		query, _ := args["query"].(string)
		limitF, _ := args["limit"].(float64)
		limit := int(limitF)
		if limit < 0 || limit > 100 {
			limit = 0
		}

		if query == "" {
			return errorResult("query is required"), nil
		}

		result, err := svc.Search(ctx, query, limit, domain.SearchSourceMCP)
		if err != nil {
			return errorResult(err.Error()), nil
		}

		return jsonResultWithHint(result, "Reminder: If this conversation produces new knowledge (decisions, business rules, lessons learned), call note_auto_capture to save it.")
	}
}

func newImportHandler(svc *service.Service, identity port.IdentityProvider) func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		ctx, errResult := mustIdentity(ctx, identity)
		if errResult != nil {
			return errResult, nil
		}
		args := req.GetArguments()

		fileContent, _ := args["file_content"].(string)
		fileName, _ := args["file_name"].(string)
		chunkMode, _ := args["chunk_mode"].(string)

		if fileContent == "" || fileName == "" {
			return errorResult("file_content and file_name are required"), nil
		}

		result, err := svc.Import(ctx, fileContent, fileName, chunkMode)
		if err != nil {
			return errorResult(err.Error()), nil
		}

		return jsonResult(result)
	}
}

func newFeedbackHandler(svc *service.Service, identity port.IdentityProvider) func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		ctx, errResult := mustIdentity(ctx, identity)
		if errResult != nil {
			return errResult, nil
		}
		args := req.GetArguments()

		itemID, _ := args["item_id"].(string)

		if itemID == "" {
			return errorResult("item_id is required"), nil
		}

		if err := svc.Feedback(ctx, itemID); err != nil {
			return errorResult(err.Error()), nil
		}

		return jsonResult(map[string]bool{"recorded": true})
	}
}

func newMaintainHandler(svc *service.Service, identity port.IdentityProvider) func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		ctx, errResult := mustIdentity(ctx, identity)
		if errResult != nil {
			return errResult, nil
		}
		args := req.GetArguments()

		tasksStr, _ := args["tasks"].(string)
		taskNames := splitComma(tasksStr)

		results, err := svc.Maintain(ctx, taskNames...)
		if err != nil {
			return errorResult(err.Error()), nil
		}

		return jsonResult(map[string]any{
			"results":         results,
			"available_tasks": svc.ListMaintainTasks(),
		})
	}
}

func newReviewHandler(svc *service.Service, identity port.IdentityProvider) func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		ctx, errResult := mustIdentity(ctx, identity)
		if errResult != nil {
			return errResult, nil
		}
		args := req.GetArguments()

		action, _ := args["action"].(string)
		id, _ := args["id"].(string)
		reason, _ := args["reason"].(string)
		limitF, _ := args["limit"].(float64)
		limit := int(limitF)

		switch action {
		case "approve":
			if id == "" {
				return errorResult("id is required for approve"), nil
			}
			if err := svc.ApproveKnowledge(ctx, id, reason); err != nil {
				return errorResult(err.Error()), nil
			}
			return jsonResult(map[string]any{"approved": true, "id": id})

		case "reject":
			if id == "" {
				return errorResult("id is required for reject"), nil
			}
			if err := svc.RejectKnowledge(ctx, id, reason); err != nil {
				return errorResult(err.Error()), nil
			}
			return jsonResult(map[string]any{"rejected": true, "id": id})

		case "revision":
			if id == "" {
				return errorResult("id is required for revision"), nil
			}
			if err := svc.RequestRevision(ctx, id, reason); err != nil {
				return errorResult(err.Error()), nil
			}
			return jsonResult(map[string]any{"revision_requested": true, "id": id})

		default:
			result, err := svc.ListPending(ctx, limit)
			if err != nil {
				return errorResult(err.Error()), nil
			}
			return jsonResultWithHint(result, "Review each item and use note_review with action=approve/reject/revision to make a decision. Only approved items will be searchable.")
		}
	}
}

func newAutoCaptureHandler(svc *service.Service, identity port.IdentityProvider) func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		ctx, errResult := mustIdentity(ctx, identity)
		if errResult != nil {
			return errResult, nil
		}
		args := req.GetArguments()

		conversation, _ := args["conversation"].(string)
		projectCtx, _ := args["project_context"].(string)

		if conversation == "" {
			return errorResult("conversation is required"), nil
		}

		result, err := svc.AutoCapture(ctx, conversation, projectCtx)
		if err != nil {
			return errorResult(err.Error()), nil
		}

		return jsonResultWithHint(result, "Keep capturing: call note_auto_capture again when new topics or decisions appear later in this conversation.")
	}
}

func newWorkLogAddHandler(svc *service.Service, identity port.IdentityProvider) func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		ctx, errResult := mustIdentity(ctx, identity)
		if errResult != nil {
			return errResult, nil
		}
		args := req.GetArguments()
		content, _ := args["content"].(string)
		date, _ := args["date"].(string)
		project, _ := args["project"].(string)
		tagsStr, _ := args["tags"].(string)
		durationF, _ := args["duration"].(float64)

		if content == "" {
			return errorResult("content is required"), nil
		}

		tags := splitComma(tagsStr)
		w, err := svc.AddWorkLog(ctx, date, content, project, tags, int(durationF))
		if err != nil {
			return errorResult(err.Error()), nil
		}
		return jsonResult(w)
	}
}

func newWorkLogListHandler(svc *service.Service, identity port.IdentityProvider) func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		ctx, errResult := mustIdentity(ctx, identity)
		if errResult != nil {
			return errResult, nil
		}
		args := req.GetArguments()
		dateFrom, _ := args["date_from"].(string)
		dateTo, _ := args["date_to"].(string)
		limitF, _ := args["limit"].(float64)
		limit := int(limitF)

		items, total, err := svc.ListWorkLogs(ctx, dateFrom, dateTo, 0, limit)
		if err != nil {
			return errorResult(err.Error()), nil
		}
		return jsonResult(map[string]any{"items": items, "total": total})
	}
}

func jsonResult(v any) (*mcp.CallToolResult, error) {
	data, err := json.Marshal(v)
	if err != nil {
		return errorResult(fmt.Sprintf("marshal result: %v", err)), nil
	}
	return mcp.NewToolResultText(string(data)), nil
}

func jsonResultWithHint(v any, hint string) (*mcp.CallToolResult, error) {
	data, err := json.Marshal(v)
	if err != nil {
		return errorResult(fmt.Sprintf("marshal result: %v", err)), nil
	}
	return mcp.NewToolResultText(string(data) + "\n\n[SYSTEM] " + hint), nil
}

func errorResult(msg string) *mcp.CallToolResult {
	return mcp.NewToolResultError(msg)
}
