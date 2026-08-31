package llm

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/LyricTian/gin-admin/v10/internal/mods/agent/schema"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func testKimi(serverURL string) *Kimi {
	return newKimi("test-api-key", serverURL)
}

func TestKimiUsesMoonshotResponsesBaseURL(t *testing.T) {
	assert.Equal(t, "https://api.moonshot.cn/v1", kimiResponsesBaseURL)
}

func TestKimiStructuredContract(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer r.Body.Close()
		assert.Equal(t, "/responses", r.URL.Path)
		assert.Equal(t, "Bearer test-api-key", r.Header.Get("Authorization"))
		var body map[string]any
		require.NoError(t, json.NewDecoder(r.Body).Decode(&body))
		assert.Equal(t, "kimi-k2.7-code", body["model"])
		assert.Equal(t, false, body["store"])
		textConfig := body["text"].(map[string]any)
		format := textConfig["format"].(map[string]any)
		assert.Equal(t, "json_schema", format["type"])
		assert.Equal(t, true, format["strict"])
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, `{"id":"resp_structured","object":"response","created_at":1,"status":"completed","model":"kimi-k2.7-code","output":[{"id":"msg_1","type":"message","status":"completed","role":"assistant","content":[{"type":"output_text","text":"{\"ok\":true}","annotations":[]}]}],"parallel_tool_calls":false,"tools":[],"usage":{"input_tokens":3,"output_tokens":4,"total_tokens":7}}`)
	}))
	defer server.Close()

	raw, usage, err := testKimi(server.URL).Structured(context.Background(), "kimi-k2.7-code", "unit_schema", "instructions", "input", map[string]any{
		"type": "object", "additionalProperties": false,
		"properties": map[string]any{"ok": map[string]any{"type": "boolean"}},
		"required":   []string{"ok"},
	})
	require.NoError(t, err)
	assert.JSONEq(t, `{"ok":true}`, string(raw))
	assert.Equal(t, Usage{InputTokens: 3, OutputTokens: 4, TotalTokens: 7}, usage)
}

func TestKimiRetrieveReplaysToolCallStatelessly(t *testing.T) {
	var calls atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer r.Body.Close()
		var body map[string]any
		require.NoError(t, json.NewDecoder(r.Body).Decode(&body))
		call := calls.Add(1)
		w.Header().Set("Content-Type", "application/json")
		if call == 1 {
			tools := body["tools"].([]any)
			require.Len(t, tools, 1)
			tool := tools[0].(map[string]any)
			assert.Equal(t, "function", tool["type"])
			assert.Equal(t, "knowledge_search", tool["name"])
			assert.Equal(t, false, body["parallel_tool_calls"])
			_, _ = io.WriteString(w, `{"id":"resp_search_1","object":"response","created_at":1,"status":"completed","model":"kimi-k2.7-code","output":[{"id":"reason_1","type":"reasoning","summary":[]},{"id":"fc_1","type":"function_call","status":"completed","call_id":"call_1","name":"knowledge_search","arguments":"{\"query\":\"needle\",\"top_k\":2}"}],"parallel_tool_calls":false,"tools":[],"usage":{"input_tokens":2,"output_tokens":1,"total_tokens":3}}`)
			return
		}
		assert.NotContains(t, body, "previous_response_id")
		input := body["input"].([]any)
		require.Len(t, input, 2)
		assert.Equal(t, "function_call", input[0].(map[string]any)["type"])
		assert.Equal(t, "call_1", input[0].(map[string]any)["call_id"])
		assert.Equal(t, "function_call_output", input[1].(map[string]any)["type"])
		assert.Equal(t, "call_1", input[1].(map[string]any)["call_id"])
		_, _ = io.WriteString(w, `{"id":"resp_search_2","object":"response","created_at":2,"status":"completed","model":"kimi-k2.7-code","output":[{"id":"msg_1","type":"message","status":"completed","role":"assistant","content":[{"type":"output_text","text":"done","annotations":[]}]}],"parallel_tool_calls":false,"tools":[],"usage":{"input_tokens":1,"output_tokens":1,"total_tokens":2}}`)
	}))
	defer server.Close()

	searchCalls := 0
	hits, usage, err := testKimi(server.URL).Retrieve(context.Background(), "kimi-k2.7-code", "find it", func(_ context.Context, query string, topK int) ([]schema.RetrievalHit, error) {
		searchCalls++
		assert.Equal(t, "needle", query)
		assert.Equal(t, 2, topK)
		return []schema.RetrievalHit{{ChunkID: "chunk-1", LineStart: 1, LineEnd: 1}}, nil
	})
	require.NoError(t, err)
	require.Len(t, hits, 1)
	assert.Equal(t, 1, searchCalls)
	assert.Equal(t, int64(5), usage.TotalTokens)
	assert.Equal(t, int32(2), calls.Load())
}

func TestKimiRedactsUpstreamErrors(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		http.Error(w, "private-upstream-body", http.StatusTooManyRequests)
	}))
	defer server.Close()

	_, _, err := testKimi(server.URL).Structured(context.Background(), "kimi-k2.7-code", "schema", "instructions", "input", map[string]any{"type": "object"})
	require.Error(t, err)
	assert.NotContains(t, err.Error(), "private-upstream-body")
	assert.True(t, strings.Contains(err.Error(), "Kimi Responses request failed"))
}
