package test

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"sort"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/LyricTian/gin-admin/v10/internal/config"
	"github.com/LyricTian/gin-admin/v10/internal/mods/agent/llm"
	agentschema "github.com/LyricTian/gin-admin/v10/internal/mods/agent/schema"
	"github.com/LyricTian/gin-admin/v10/pkg/util"
	"github.com/gavv/httpexpect/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type fakeAgentGateway struct {
	mu                 sync.Mutex
	stages             []string
	lastChunkID        string
	finalReviewApprove bool
	invalidCitation    bool
	embedFailures      int
}

func (a *fakeAgentGateway) add(stage string) {
	a.mu.Lock()
	a.stages = append(a.stages, stage)
	a.mu.Unlock()
}

func (a *fakeAgentGateway) Embed(_ context.Context, _ string, texts []string) ([][]float32, llm.Usage, error) {
	a.mu.Lock()
	if a.embedFailures > 0 {
		a.embedFailures--
		a.mu.Unlock()
		return nil, llm.Usage{}, errors.New("OpenAI rate limit")
	}
	a.mu.Unlock()
	result := make([][]float32, len(texts))
	for i := range result {
		result[i] = []float32{1, 0}
	}
	return result, llm.Usage{InputTokens: int64(len(texts)), TotalTokens: int64(len(texts))}, nil
}

func (a *fakeAgentGateway) setEmbedFailures(count int) {
	a.mu.Lock()
	a.embedFailures = count
	a.mu.Unlock()
}

func (a *fakeAgentGateway) Structured(_ context.Context, _ string, name, _ string, _ string, _ map[string]any) (json.RawMessage, llm.Usage, error) {
	a.mu.Lock()
	chunkID := a.lastChunkID
	invalidCitation := a.invalidCitation
	approveFinal := a.finalReviewApprove
	a.mu.Unlock()

	switch name {
	case "supervisor_plan":
		a.add("supervisor")
		return json.RawMessage(`{"retrieval_query":"verification phrase","answer_constraints":["cite the source"]}`), llm.Usage{InputTokens: 1, OutputTokens: 1, TotalTokens: 2}, nil
	case "grounded_answer":
		a.add("answerer")
		if invalidCitation {
			chunkID = "invented-chunk"
		}
		return json.RawMessage(fmt.Sprintf(`{"markdown":"The verification phrase is cobalt-orchid.","citations":[%q]}`, chunkID)), llm.Usage{InputTokens: 1, OutputTokens: 1, TotalTokens: 2}, nil
	case "answer_review":
		a.add("reviewer")
		return json.RawMessage(`{"approved":false,"feedback":"Use the source wording exactly."}`), llm.Usage{InputTokens: 1, OutputTokens: 1, TotalTokens: 2}, nil
	case "revised_grounded_answer":
		a.add("answerer.revision")
		return json.RawMessage(fmt.Sprintf(`{"markdown":"The verification phrase is **cobalt-orchid**.","citations":[%q]}`, chunkID)), llm.Usage{InputTokens: 1, OutputTokens: 1, TotalTokens: 2}, nil
	case "final_answer_review":
		a.add("reviewer.final")
		return json.RawMessage(fmt.Sprintf(`{"approved":%t,"feedback":""}`, approveFinal)), llm.Usage{InputTokens: 1, OutputTokens: 1, TotalTokens: 2}, nil
	default:
		return nil, llm.Usage{}, fmt.Errorf("unexpected structured output name %q", name)
	}
}

func (a *fakeAgentGateway) Retrieve(ctx context.Context, _ string, _ string, search llm.SearchFunc) ([]agentschema.RetrievalHit, llm.Usage, error) {
	a.add("retriever")
	hits, err := search(ctx, "verification phrase", 8)
	if err != nil {
		return nil, llm.Usage{}, err
	}
	if len(hits) > 0 {
		a.mu.Lock()
		a.lastChunkID = hits[0].ChunkID
		a.mu.Unlock()
	}
	return hits, llm.Usage{InputTokens: 1, OutputTokens: 1, TotalTokens: 2}, nil
}

func (a *fakeAgentGateway) resetRun(approveFinal, invalidCitation bool) {
	a.mu.Lock()
	a.stages = nil
	a.lastChunkID = ""
	a.finalReviewApprove = approveFinal
	a.invalidCitation = invalidCitation
	a.mu.Unlock()
}

func (a *fakeAgentGateway) stageList() []string {
	a.mu.Lock()
	defer a.mu.Unlock()
	return append([]string(nil), a.stages...)
}

func TestAgentKnowledgeWorkflowAndSSE(t *testing.T) {
	t.Setenv("OPENAI_API_KEY", "test-openai-key")
	t.Setenv("AGENT_API_KEY", "test-agent-key")

	originalConfig := config.C.Agent
	config.C.Agent.Enable = true
	config.C.Agent.WorkerPollSeconds = 1
	config.C.Agent.IndexWorkerConcurrency = 1
	config.C.Agent.MaxIndexAttempts = 3
	defer func() { config.C.Agent = originalConfig }()

	module := appInjector.M.Agent
	service := module.Service
	originalGateway := service.LLM
	fake := &fakeAgentGateway{}
	fake.resetRun(true, false)
	service.LLM = fake
	require.NoError(t, module.AutoMigrate(context.Background()))
	require.NoError(t, service.Start(context.Background()))
	defer func() {
		service.Stop()
		service.LLM = originalGateway
	}()

	e := tester(t)
	e.GET(baseAPI + "/agent/status").Expect().Status(http.StatusOK).
		JSON().Path("$.data.enabled").Boolean().IsTrue()
	e.GET(baseAPI + "/agent/knowledge-bases").Expect().Status(http.StatusUnauthorized)
	e.GET(baseAPI+"/agent/knowledge-bases").WithHeader("Authorization", "Bearer wrong").Expect().Status(http.StatusUnauthorized)

	const auth = "Bearer test-agent-key"
	var kb agentschema.KnowledgeBase
	e.POST(baseAPI+"/agent/knowledge-bases").WithHeader("Authorization", auth).
		WithJSON(agentschema.KnowledgeBaseForm{Name: "Agent integration test", Description: "Fake OpenAI workflow"}).
		Expect().Status(http.StatusOK).JSON().Decode(&util.ResponseResult{Data: &kb})
	require.NotEmpty(t, kb.ID)
	e.POST(baseAPI+"/agent/knowledge-bases/"+kb.ID+"/documents").WithHeader("Authorization", auth).
		WithMultipart().WithFileBytes("file", "not-allowed.pdf", []byte("not a supported document")).
		Expect().Status(http.StatusBadRequest)
	e.POST(baseAPI+"/agent/knowledge-bases/"+kb.ID+"/documents").WithHeader("Authorization", auth).
		WithMultipart().WithFileBytes("file", "invalid.md", []byte{0xff, 0xfe}).
		Expect().Status(http.StatusBadRequest)

	type uploadResult struct {
		Document     agentschema.Document     `json:"document"`
		IngestionJob agentschema.IngestionJob `json:"ingestion_job"`
	}
	var uploaded uploadResult
	e.POST(baseAPI+"/agent/knowledge-bases/"+kb.ID+"/documents").WithHeader("Authorization", auth).
		WithMultipart().WithFileBytes("file", "guide.md", []byte("# Smoke guide\n\nThe verification phrase is cobalt-orchid.\n")).
		Expect().Status(http.StatusAccepted).JSON().Decode(&util.ResponseResult{Data: &uploaded})
	require.NotEmpty(t, uploaded.Document.ID)
	require.NotEmpty(t, uploaded.IngestionJob.ID)

	e.POST(baseAPI+"/agent/knowledge-bases/"+kb.ID+"/documents").WithHeader("Authorization", auth).
		WithMultipart().WithFileBytes("file", "copy.md", []byte("# Smoke guide\n\nThe verification phrase is cobalt-orchid.\n")).
		Expect().Status(http.StatusConflict)

	waitAgentJob(t, uploaded.IngestionJob.ID)
	var indexed agentschema.Document
	e.GET(baseAPI+"/agent/documents/"+uploaded.Document.ID).WithHeader("Authorization", auth).
		Expect().Status(http.StatusOK).JSON().Decode(&util.ResponseResult{Data: &indexed})
	assert.Equal(t, agentschema.IndexStatusReady, indexed.IndexStatus)

	var conversation agentschema.Conversation
	e.POST(baseAPI+"/agent/conversations").WithHeader("Authorization", auth).
		WithJSON(agentschema.ConversationForm{KnowledgeBaseID: kb.ID, Title: "Evidence chat"}).
		Expect().Status(http.StatusOK).JSON().Decode(&util.ResponseResult{Data: &conversation})
	require.Equal(t, kb.ID, conversation.KnowledgeBaseID)

	runID := createAgentRun(t, e, auth, conversation.ID, "What is the verification phrase?")
	completed := waitAgentRun(t, runID)
	assert.Equal(t, agentschema.RunStatusCompleted, completed.Status)
	assert.Equal(t, 1, completed.RevisionCount)
	require.NotNil(t, completed.FinalMessage)
	require.Len(t, completed.FinalMessage.Citations, 1)
	assert.Equal(t, uploaded.Document.ID, completed.FinalMessage.Citations[0].DocumentID)
	assert.Greater(t, completed.TotalTokens, int64(0))
	assert.Equal(t, []string{"supervisor", "retriever", "answerer", "reviewer", "answerer.revision", "reviewer.final"}, fake.stageList())

	sse := e.GET(baseAPI+"/agent/runs/"+runID+"/events").WithHeader("Authorization", auth).
		Expect().Status(http.StatusOK).Body().Raw()
	assert.Contains(t, sse, "event: run.started")
	assert.Contains(t, sse, "event: retrieval.completed")
	assert.Contains(t, sse, "event: answer.delta")
	assert.Contains(t, sse, "event: answer.completed")
	assert.Contains(t, sse, `"run_id":"`+runID+`"`)
	assert.Contains(t, sse, `"time":"`)
	eventIDs := sseIDs(sse)
	require.NotEmpty(t, eventIDs)
	assert.True(t, sort.SliceIsSorted(eventIDs, func(i, j int) bool { return eventIDs[i] < eventIDs[j] }))
	replay := e.GET(fmt.Sprintf("%s/agent/runs/%s/events", baseAPI, runID)).WithQuery("after", eventIDs[0]).WithHeader("Authorization", auth).
		Expect().Status(http.StatusOK).Body().Raw()
	replayIDs := sseIDs(replay)
	require.NotEmpty(t, replayIDs)
	assert.Greater(t, replayIDs[0], eventIDs[0])

	fake.resetRun(false, false)
	failedReviewRunID := createAgentRun(t, e, auth, conversation.ID, "Please repeat the verification phrase.")
	failedReview := waitAgentRun(t, failedReviewRunID)
	assert.Equal(t, agentschema.RunStatusFailedReview, failedReview.Status)
	assert.Nil(t, failedReview.FinalMessage)
	failedReviewEvents, err := service.ListEvents(context.Background(), failedReviewRunID, 0)
	require.NoError(t, err)
	for _, event := range failedReviewEvents {
		assert.NotEqual(t, "answer.delta", event.Type)
	}

	fake.resetRun(true, true)
	invalidCitationRunID := createAgentRun(t, e, auth, conversation.ID, "Cite the verification phrase.")
	invalidCitationRun := waitAgentRun(t, invalidCitationRunID)
	assert.Equal(t, agentschema.RunStatusFailed, invalidCitationRun.Status)
	assert.Nil(t, invalidCitationRun.FinalMessage)

	fake.setEmbedFailures(3)
	var failedJob agentschema.IngestionJob
	e.POST(baseAPI+"/agent/documents/"+uploaded.Document.ID+"/reindex").WithHeader("Authorization", auth).
		Expect().Status(http.StatusAccepted).JSON().Decode(&util.ResponseResult{Data: &failedJob})
	waitAgentJobFailure(t, failedJob.ID)
}

func createAgentRun(t *testing.T, e *httpexpect.Expect, auth, conversationID, content string) string {
	t.Helper()
	var created agentschema.RunCreated
	e.POST(baseAPI+"/agent/conversations/"+conversationID+"/runs").WithHeader("Authorization", auth).
		WithJSON(agentschema.RunForm{Content: content}).Expect().Status(http.StatusAccepted).
		JSON().Decode(&util.ResponseResult{Data: &created})
	require.NotEmpty(t, created.RunID)
	assert.Equal(t, agentschema.RunStatusQueued, created.Status)
	return created.RunID
}

func waitAgentJob(t *testing.T, jobID string) {
	t.Helper()
	require.Eventually(t, func() bool {
		var job agentschema.IngestionJob
		if err := appInjector.DB.Where("id = ?", jobID).First(&job).Error; err != nil {
			return false
		}
		return job.Status == agentschema.JobStatusCompleted || job.Status == agentschema.JobStatusFailed
	}, 10*time.Second, 25*time.Millisecond)
	var job agentschema.IngestionJob
	require.NoError(t, appInjector.DB.Where("id = ?", jobID).First(&job).Error)
	require.Equal(t, agentschema.JobStatusCompleted, job.Status, job.ErrorSummary)
}

func waitAgentRun(t *testing.T, runID string) *agentschema.Run {
	t.Helper()
	require.Eventually(t, func() bool {
		var run agentschema.Run
		if err := appInjector.DB.Where("id = ?", runID).First(&run).Error; err != nil {
			return false
		}
		return run.Status == agentschema.RunStatusCompleted || run.Status == agentschema.RunStatusFailed || run.Status == agentschema.RunStatusFailedReview || run.Status == agentschema.RunStatusInterrupted
	}, 10*time.Second, 25*time.Millisecond)
	result, err := appInjector.M.Agent.Service.GetRun(context.Background(), runID)
	require.NoError(t, err)
	return result
}

func waitAgentJobFailure(t *testing.T, jobID string) {
	t.Helper()
	require.Eventually(t, func() bool {
		var job agentschema.IngestionJob
		if err := appInjector.DB.Where("id = ?", jobID).First(&job).Error; err != nil {
			return false
		}
		return job.Status == agentschema.JobStatusFailed
	}, 10*time.Second, 25*time.Millisecond)
	var job agentschema.IngestionJob
	require.NoError(t, appInjector.DB.Where("id = ?", jobID).First(&job).Error)
	require.Equal(t, agentschema.JobStatusFailed, job.Status)
	require.Equal(t, 3, job.Attempts)
}

func sseIDs(body string) []uint64 {
	var ids []uint64
	for _, line := range strings.Split(body, "\n") {
		var id uint64
		if _, err := fmt.Sscanf(line, "id: %d", &id); err == nil {
			ids = append(ids, id)
		}
	}
	return ids
}
