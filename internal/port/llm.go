package port

import "context"

type LLMClient interface {
	Chat(ctx context.Context, messages []LLMMessage) (string, error)
	ChatJSON(ctx context.Context, messages []LLMMessage, schema any) (string, error)
}

type LLMMessage struct {
	Role    string // system / user / assistant
	Content string
}
