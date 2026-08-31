package llm

import (
	"context"
	"encoding/json"

	"github.com/LyricTian/gin-admin/v10/internal/mods/agent/schema"
)

type Usage struct {
	InputTokens  int64
	OutputTokens int64
	TotalTokens  int64
}

func (a *Usage) Add(other Usage) {
	a.InputTokens += other.InputTokens
	a.OutputTokens += other.OutputTokens
	a.TotalTokens += other.TotalTokens
}

type SearchFunc func(context.Context, string, int) ([]schema.RetrievalHit, error)

type Gateway interface {
	Structured(context.Context, string, string, string, string, map[string]any) (json.RawMessage, Usage, error)
	Retrieve(context.Context, string, string, SearchFunc) ([]schema.RetrievalHit, Usage, error)
}
