package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/mark3labs/mcp-go/mcp"
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

		return jsonResult(result)
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

		result, err := svc.Search(ctx, query, limit)
		if err != nil {
			return errorResult(err.Error()), nil
		}

		return jsonResult(result)
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

func newCaptureHandler(svc *service.Service, identity port.IdentityProvider) func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		ctx, errResult := mustIdentity(ctx, identity)
		if errResult != nil {
			return errResult, nil
		}
		args := req.GetArguments()

		sessionSummary, _ := args["session_summary"].(string)
		itemsJSON, _ := args["items_json"].(string)

		if sessionSummary == "" {
			return errorResult("session_summary is required"), nil
		}

		result, err := svc.Capture(ctx, sessionSummary, itemsJSON)
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

func jsonResult(v any) (*mcp.CallToolResult, error) {
	data, err := json.Marshal(v)
	if err != nil {
		return errorResult(fmt.Sprintf("marshal result: %v", err)), nil
	}
	return mcp.NewToolResultText(string(data)), nil
}

func errorResult(msg string) *mcp.CallToolResult {
	return mcp.NewToolResultError(msg)
}
