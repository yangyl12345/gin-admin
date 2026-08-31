package llm

import (
	"context"
	"encoding/json"
	"errors"
	"os"

	"github.com/LyricTian/gin-admin/v10/internal/mods/agent/schema"
	openai "github.com/openai/openai-go/v3"
	"github.com/openai/openai-go/v3/option"
	"github.com/openai/openai-go/v3/responses"
)

type OpenAI struct{ client openai.Client }

func NewOpenAI() *OpenAI {
	return &OpenAI{client: openai.NewClient(option.WithAPIKey(os.Getenv("OPENAI_API_KEY")))}
}

func (a *OpenAI) Embed(ctx context.Context, model string, texts []string) ([][]float32, Usage, error) {
	result, err := a.client.Embeddings.New(ctx, openai.EmbeddingNewParams{
		Input:          openai.EmbeddingNewParamsInputUnion{OfArrayOfStrings: texts},
		Model:          model,
		EncodingFormat: openai.EmbeddingNewParamsEncodingFormatFloat,
	})
	if err != nil {
		return nil, Usage{}, errors.New("OpenAI embeddings request failed")
	}
	vectors := make([][]float32, len(result.Data))
	for _, item := range result.Data {
		if item.Index < 0 || int(item.Index) >= len(vectors) {
			return nil, Usage{}, errors.New("OpenAI embeddings response was invalid")
		}
		vector := make([]float32, len(item.Embedding))
		for i, value := range item.Embedding {
			vector[i] = float32(value)
		}
		vectors[item.Index] = vector
	}
	return vectors, Usage{InputTokens: result.Usage.PromptTokens, TotalTokens: result.Usage.TotalTokens}, nil
}

func (a *OpenAI) Structured(ctx context.Context, model, name, instructions, input string, outputSchema map[string]any) (json.RawMessage, Usage, error) {
	result, err := a.client.Responses.New(ctx, responses.ResponseNewParams{
		Model:        model,
		Instructions: openai.String(instructions),
		Input:        responses.ResponseNewParamsInputUnion{OfString: openai.String(input)},
		Store:        openai.Bool(false),
		Text: responses.ResponseTextConfigParam{Format: responses.ResponseFormatTextConfigUnionParam{
			OfJSONSchema: &responses.ResponseFormatTextJSONSchemaConfigParam{Name: name, Schema: outputSchema, Strict: openai.Bool(true)},
		}},
	})
	if err != nil {
		return nil, Usage{}, errors.New("OpenAI Responses request failed")
	}
	raw := json.RawMessage(result.OutputText())
	if !json.Valid(raw) {
		return nil, Usage{}, errors.New("OpenAI structured response was invalid")
	}
	return raw, Usage{InputTokens: result.Usage.InputTokens, OutputTokens: result.Usage.OutputTokens, TotalTokens: result.Usage.TotalTokens}, nil
}

func (a *OpenAI) Retrieve(ctx context.Context, model, input string, search SearchFunc) ([]schema.RetrievalHit, Usage, error) {
	params := responses.ResponseNewParams{
		Model:             model,
		Instructions:      openai.String("You are the Retriever. You may only gather evidence by calling knowledge_search. Call it one to three times with concise search queries, then stop. Never answer the user's question."),
		Input:             responses.ResponseNewParamsInputUnion{OfString: openai.String(input)},
		Store:             openai.Bool(false),
		ParallelToolCalls: openai.Bool(false),
		Tools: []responses.ToolUnionParam{{OfFunction: &responses.FunctionToolParam{
			Name: "knowledge_search", Description: openai.String("Search the selected knowledge base for relevant evidence."), Strict: openai.Bool(true),
			Parameters: map[string]any{"type": "object", "additionalProperties": false, "properties": map[string]any{"query": map[string]any{"type": "string"}, "top_k": map[string]any{"type": "integer", "minimum": 1, "maximum": 20}}, "required": []string{"query", "top_k"}},
		}}},
	}
	var usage Usage
	seen := make(map[string]struct{})
	var hits []schema.RetrievalHit
	toolCalls := 0
	for round := 0; round < 3; round++ {
		result, err := a.client.Responses.New(ctx, params)
		if err != nil {
			return nil, usage, errors.New("OpenAI Retriever request failed")
		}
		usage.Add(Usage{InputTokens: result.Usage.InputTokens, OutputTokens: result.Usage.OutputTokens, TotalTokens: result.Usage.TotalTokens})
		var outputs []responses.ResponseInputItemUnionParam
		calledThisRound := false
		for _, item := range result.Output {
			if item.Type != "function_call" {
				continue
			}
			call := item.AsFunctionCall()
			if call.Name != "knowledge_search" {
				return nil, usage, errors.New("OpenAI Retriever called an unsupported tool")
			}
			if toolCalls >= 3 {
				return nil, usage, errors.New("OpenAI Retriever exceeded the tool-call limit")
			}
			var args struct {
				Query string `json:"query"`
				TopK  int    `json:"top_k"`
			}
			if err := json.Unmarshal([]byte(call.Arguments), &args); err != nil || args.Query == "" || args.TopK < 1 || args.TopK > 20 {
				return nil, usage, errors.New("OpenAI Retriever tool arguments were invalid")
			}
			calledThisRound = true
			toolCalls++
			found, err := search(ctx, args.Query, args.TopK)
			if err != nil {
				return nil, usage, err
			}
			for _, hit := range found {
				if _, ok := seen[hit.ChunkID]; ok {
					continue
				}
				seen[hit.ChunkID] = struct{}{}
				hits = append(hits, hit)
			}
			payload, _ := json.Marshal(found)
			outputs = append(outputs, responses.ResponseInputItemUnionParam{OfFunctionCallOutput: &responses.ResponseInputItemFunctionCallOutputParam{CallID: openai.String(call.CallID), Output: responses.ResponseInputItemFunctionCallOutputOutputUnionParam{OfString: openai.String(string(payload))}}})
		}
		if !calledThisRound {
			if toolCalls == 0 {
				return nil, usage, errors.New("OpenAI Retriever did not call knowledge_search")
			}
			return hits, usage, nil
		}
		if toolCalls >= 3 {
			return hits, usage, nil
		}
		params = responses.ResponseNewParams{Model: model, PreviousResponseID: openai.String(result.ID), Input: responses.ResponseNewParamsInputUnion{OfInputItemList: outputs}, Store: openai.Bool(false), ParallelToolCalls: openai.Bool(false), Tools: params.Tools}
	}
	return hits, usage, nil
}
