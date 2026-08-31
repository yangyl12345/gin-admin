package biz

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"time"
	"unicode/utf8"

	"github.com/LyricTian/gin-admin/v10/internal/config"
	"github.com/LyricTian/gin-admin/v10/internal/mods/agent/dal"
	"github.com/LyricTian/gin-admin/v10/internal/mods/agent/llm"
	"github.com/LyricTian/gin-admin/v10/internal/mods/agent/retrieval"
	"github.com/LyricTian/gin-admin/v10/internal/mods/agent/schema"
	projecterrors "github.com/LyricTian/gin-admin/v10/pkg/errors"
	"github.com/LyricTian/gin-admin/v10/pkg/util"
)

type Service struct {
	Store *dal.Store
	Trans *util.Trans
	LLM   llm.Gateway
	Cache *retrieval.Cache
	Hub   *EventHub

	cancel    context.CancelFunc
	wg        sync.WaitGroup
	indexWake chan struct{}
	runWake   chan struct{}
	running   atomic.Bool
}

func NewService(store *dal.Store, trans *util.Trans, gateway llm.Gateway, cache *retrieval.Cache, hub *EventHub) *Service {
	return &Service{Store: store, Trans: trans, LLM: gateway, Cache: cache, Hub: hub, indexWake: make(chan struct{}, 1), runWake: make(chan struct{}, 1)}
}

func (a *Service) ValidateConfig() error {
	if !config.C.Agent.Enable {
		return nil
	}
	cfg := config.C.Agent
	modelsValid := strings.TrimSpace(cfg.EmbeddingModel) != "" && strings.TrimSpace(cfg.SupervisorModel) != "" && strings.TrimSpace(cfg.RetrieverModel) != "" && strings.TrimSpace(cfg.AnswererModel) != "" && strings.TrimSpace(cfg.ReviewerModel) != ""
	limitsValid := cfg.MaxUploadBytes > 0 && cfg.MaxUploadBytes <= 10*1024*1024 && cfg.ChunkSize > 0 && cfg.ChunkOverlap >= 0 && cfg.ChunkOverlap < cfg.ChunkSize && cfg.MaxChunksPerKnowledgeBase > 0 && cfg.IndexWorkerConcurrency > 0 && cfg.MaxIndexAttempts > 0 && cfg.WorkerPollSeconds > 0 && cfg.RunTimeoutSeconds > 0 && cfg.RetrievalTopK > 0 && cfg.RetrievalTopK <= 20 && cfg.CacheTTLSeconds >= 0 && cfg.EmbeddingBatchSize > 0
	if !modelsValid || !limitsValid {
		return fmt.Errorf("invalid Agent configuration")
	}
	if strings.TrimSpace(getenv("OPENAI_API_KEY")) == "" {
		return fmt.Errorf("OPENAI_API_KEY is required when Agent is enabled")
	}
	if strings.TrimSpace(getenv("AGENT_API_KEY")) == "" {
		return fmt.Errorf("AGENT_API_KEY is required when Agent is enabled")
	}
	return nil
}

func (a *Service) Start(ctx context.Context) error {
	if !config.C.Agent.Enable {
		return nil
	}
	if err := a.ValidateConfig(); err != nil {
		return err
	}
	if err := a.Store.RequeueInterruptedIngestion(ctx); err != nil {
		return err
	}
	interruptedRunIDs, err := a.Store.InterruptRunningRuns(ctx)
	if err != nil {
		return err
	}
	for _, runID := range interruptedRunIDs {
		if err := a.publishEvent(ctx, runID, "run.failed", map[string]any{"status": schema.RunStatusInterrupted, "error": "run interrupted by service restart"}); err != nil {
			return err
		}
	}
	workerCtx, cancel := context.WithCancel(ctx)
	a.cancel = cancel
	a.running.Store(true)
	concurrency := config.C.Agent.IndexWorkerConcurrency
	if concurrency < 1 {
		concurrency = 1
	}
	for i := 0; i < concurrency; i++ {
		a.wg.Add(1)
		go a.indexLoop(workerCtx)
	}
	a.wg.Add(1)
	go a.runLoop(workerCtx)
	a.wakeIndex()
	a.wakeRun()
	return nil
}

var getenv = os.Getenv

func (a *Service) Stop() {
	if a.cancel != nil {
		a.cancel()
	}
	a.wg.Wait()
	a.running.Store(false)
}

func (a *Service) Status() *schema.Status {
	return &schema.Status{Name: "agent", Enabled: config.C.Agent.Enable, WorkersRunning: a.running.Load(), EmbeddingModel: config.C.Agent.EmbeddingModel, Models: map[string]string{"supervisor": config.C.Agent.SupervisorModel, "retriever": config.C.Agent.RetrieverModel, "answerer": config.C.Agent.AnswererModel, "reviewer": config.C.Agent.ReviewerModel}}
}

func (a *Service) CreateKnowledgeBase(ctx context.Context, form *schema.KnowledgeBaseForm) (*schema.KnowledgeBase, error) {
	now := time.Now()
	item := &schema.KnowledgeBase{ID: util.NewXID(), Name: strings.TrimSpace(form.Name), Description: strings.TrimSpace(form.Description), Status: schema.StatusActive, CreatedAt: now, UpdatedAt: now}
	if item.Name == "" {
		return nil, projecterrors.BadRequest("agent_name_required", "name is required")
	}
	return item, a.Store.CreateKnowledgeBase(ctx, item)
}

func (a *Service) QueryKnowledgeBases(ctx context.Context, params schema.KnowledgeBaseQuery) ([]*schema.KnowledgeBase, *util.PaginationResult, error) {
	return a.Store.QueryKnowledgeBases(ctx, params)
}

func (a *Service) GetKnowledgeBase(ctx context.Context, id string) (*schema.KnowledgeBase, error) {
	item, ok, err := a.Store.GetKnowledgeBase(ctx, id, false)
	if err != nil {
		return nil, err
	}
	if !ok {
		return nil, projecterrors.NotFound("agent_knowledge_base_not_found", "knowledge base not found")
	}
	return item, nil
}

func (a *Service) UpdateKnowledgeBase(ctx context.Context, id string, form *schema.KnowledgeBaseForm) error {
	item, err := a.GetKnowledgeBase(ctx, id)
	if err != nil {
		return err
	}
	item.Name, item.Description, item.UpdatedAt = strings.TrimSpace(form.Name), strings.TrimSpace(form.Description), time.Now()
	if item.Name == "" {
		return projecterrors.BadRequest("agent_name_required", "name is required")
	}
	return a.Store.UpdateKnowledgeBase(ctx, item)
}

func (a *Service) ArchiveKnowledgeBase(ctx context.Context, id string) error {
	if _, err := a.GetKnowledgeBase(ctx, id); err != nil {
		return err
	}
	a.Cache.Invalidate(id)
	return a.Store.ArchiveKnowledgeBase(ctx, id, time.Now())
}

func (a *Service) QueryDocuments(ctx context.Context, kbID string, params schema.DocumentQuery) ([]*schema.Document, *util.PaginationResult, error) {
	if _, err := a.GetKnowledgeBase(ctx, kbID); err != nil {
		return nil, nil, err
	}
	return a.Store.QueryDocuments(ctx, kbID, params)
}

func (a *Service) GetDocument(ctx context.Context, id string) (*schema.Document, error) {
	item, ok, err := a.Store.GetDocument(ctx, id, false)
	if err != nil {
		return nil, err
	}
	if !ok {
		return nil, projecterrors.NotFound("agent_document_not_found", "document not found")
	}
	return item, nil
}

func (a *Service) UploadDocument(ctx context.Context, kbID, name string, content []byte) (*schema.Document, *schema.IngestionJob, error) {
	if _, err := a.GetKnowledgeBase(ctx, kbID); err != nil {
		return nil, nil, err
	}
	if name == "" || filepath.Base(name) != name || strings.ContainsAny(name, "/\\") {
		return nil, nil, projecterrors.BadRequest("agent_invalid_filename", "file name must not contain a path")
	}
	ext := strings.ToLower(filepath.Ext(name))
	if ext != ".txt" && ext != ".md" {
		return nil, nil, projecterrors.BadRequest("agent_unsupported_file", "only .txt and .md files are accepted")
	}
	if int64(len(content)) > config.C.Agent.MaxUploadBytes {
		return nil, nil, projecterrors.RequestEntityTooLarge("agent_file_too_large", "file exceeds the configured 10 MiB limit")
	}
	if len(content) == 0 || !utf8.Valid(content) || strings.ContainsRune(string(content), '\x00') {
		return nil, nil, projecterrors.BadRequest("agent_invalid_utf8", "file must be non-empty UTF-8 text")
	}
	normalized := strings.TrimPrefix(strings.ReplaceAll(strings.ReplaceAll(string(content), "\r\n", "\n"), "\r", "\n"), "\ufeff")
	if strings.TrimSpace(normalized) == "" {
		return nil, nil, projecterrors.BadRequest("agent_empty_file", "file must not be empty")
	}
	sum := sha256.Sum256([]byte(normalized))
	sha := hex.EncodeToString(sum[:])
	now := time.Now()
	mediaType := "text/plain"
	if ext == ".md" {
		mediaType = "text/markdown"
	}
	doc := &schema.Document{ID: util.NewXID(), KnowledgeBaseID: kbID, OriginalName: name, MediaType: mediaType, SizeBytes: int64(len(content)), SHA256: sha, Content: normalized, IndexStatus: schema.IndexStatusPending, CreatedAt: now, UpdatedAt: now}
	job := &schema.IngestionJob{ID: util.NewXID(), DocumentID: doc.ID, KnowledgeBaseID: kbID, Status: schema.JobStatusQueued, CreatedAt: now, UpdatedAt: now}
	err := a.Trans.Exec(ctx, func(txCtx context.Context) error {
		if _, ok, err := a.Store.LockKnowledgeBase(txCtx, kbID); err != nil {
			return err
		} else if !ok {
			return projecterrors.NotFound("agent_knowledge_base_not_found", "knowledge base not found")
		}
		exists, err := a.Store.ActiveDocumentSHAExists(txCtx, kbID, sha)
		if err != nil {
			return err
		}
		if exists {
			return projecterrors.Conflict("agent_duplicate_document", "an active document with the same content already exists")
		}
		if err := a.Store.CreateDocument(txCtx, doc); err != nil {
			return err
		}
		return a.Store.CreateIngestionJob(txCtx, job)
	})
	if err != nil {
		return nil, nil, err
	}
	a.wakeIndex()
	return doc, job, nil
}

func (a *Service) ArchiveDocument(ctx context.Context, id string) error {
	doc, err := a.GetDocument(ctx, id)
	if err != nil {
		return err
	}
	if err := a.Trans.Exec(ctx, func(txCtx context.Context) error {
		if err := a.Store.ArchiveDocument(txCtx, id, time.Now()); err != nil {
			return err
		}
		return a.Store.DeleteDocumentChunks(txCtx, id)
	}); err != nil {
		return err
	}
	a.Cache.Invalidate(doc.KnowledgeBaseID)
	return nil
}

func (a *Service) ReindexDocument(ctx context.Context, id string) (*schema.IngestionJob, error) {
	doc, err := a.GetDocument(ctx, id)
	if err != nil {
		return nil, err
	}
	now := time.Now()
	job := &schema.IngestionJob{ID: util.NewXID(), DocumentID: doc.ID, KnowledgeBaseID: doc.KnowledgeBaseID, Status: schema.JobStatusQueued, CreatedAt: now, UpdatedAt: now}
	if err := a.Trans.Exec(ctx, func(txCtx context.Context) error {
		if err := a.Store.UpdateDocumentIndex(txCtx, doc.ID, schema.IndexStatusPending, "", nil); err != nil {
			return err
		}
		return a.Store.CreateIngestionJob(txCtx, job)
	}); err != nil {
		return nil, err
	}
	a.Cache.Invalidate(doc.KnowledgeBaseID)
	a.wakeIndex()
	return job, nil
}

func (a *Service) GetIngestionJob(ctx context.Context, id string) (*schema.IngestionJob, error) {
	item, ok, err := a.Store.GetIngestionJob(ctx, id)
	if err != nil {
		return nil, err
	}
	if !ok {
		return nil, projecterrors.NotFound("agent_ingestion_job_not_found", "ingestion job not found")
	}
	return item, nil
}

func (a *Service) CreateConversation(ctx context.Context, form *schema.ConversationForm) (*schema.Conversation, error) {
	if _, err := a.GetKnowledgeBase(ctx, form.KnowledgeBaseID); err != nil {
		return nil, err
	}
	now := time.Now()
	title := strings.TrimSpace(form.Title)
	if title == "" {
		title = "新对话"
	}
	item := &schema.Conversation{ID: util.NewXID(), KnowledgeBaseID: form.KnowledgeBaseID, Title: title, CreatedAt: now, UpdatedAt: now}
	return item, a.Store.CreateConversation(ctx, item)
}

func (a *Service) QueryConversations(ctx context.Context, params schema.ConversationQuery) ([]*schema.Conversation, *util.PaginationResult, error) {
	return a.Store.QueryConversations(ctx, params)
}

func (a *Service) GetConversation(ctx context.Context, id string) (*schema.Conversation, error) {
	item, ok, err := a.Store.GetConversation(ctx, id)
	if err != nil {
		return nil, err
	}
	if !ok {
		return nil, projecterrors.NotFound("agent_conversation_not_found", "conversation not found")
	}
	return item, nil
}

func (a *Service) ArchiveConversation(ctx context.Context, id string) error {
	if _, err := a.GetConversation(ctx, id); err != nil {
		return err
	}
	return a.Store.ArchiveConversation(ctx, id, time.Now())
}

func (a *Service) QueryMessages(ctx context.Context, conversationID string, params schema.MessageQuery) ([]*schema.Message, *util.PaginationResult, error) {
	if _, err := a.GetConversation(ctx, conversationID); err != nil {
		return nil, nil, err
	}
	return a.Store.QueryMessages(ctx, conversationID, params)
}

func (a *Service) CreateRun(ctx context.Context, conversationID string, form *schema.RunForm) (*schema.RunCreated, error) {
	conversation, err := a.GetConversation(ctx, conversationID)
	if err != nil {
		return nil, err
	}
	if _, err := a.GetKnowledgeBase(ctx, conversation.KnowledgeBaseID); err != nil {
		return nil, err
	}
	content := strings.TrimSpace(form.Content)
	if content == "" {
		return nil, projecterrors.BadRequest("agent_content_required", "content is required")
	}
	now := time.Now()
	message := &schema.Message{ID: util.NewXID(), ConversationID: conversation.ID, Role: schema.RoleUser, Content: content, CreatedAt: now}
	run := &schema.Run{ID: util.NewXID(), ConversationID: conversation.ID, KnowledgeBaseID: conversation.KnowledgeBaseID, UserMessageID: message.ID, Status: schema.RunStatusQueued, CreatedAt: now, UpdatedAt: now}
	message.RunID = run.ID
	if err := a.Trans.Exec(ctx, func(txCtx context.Context) error {
		if err := a.Store.CreateMessage(txCtx, message); err != nil {
			return err
		}
		return a.Store.CreateRun(txCtx, run)
	}); err != nil {
		return nil, err
	}
	a.wakeRun()
	return &schema.RunCreated{RunID: run.ID, MessageID: message.ID, Status: run.Status}, nil
}

func (a *Service) GetRun(ctx context.Context, id string) (*schema.Run, error) {
	item, ok, err := a.Store.GetRun(ctx, id)
	if err != nil {
		return nil, err
	}
	if !ok {
		return nil, projecterrors.NotFound("agent_run_not_found", "run not found")
	}
	return item, nil
}

func (a *Service) ListEvents(ctx context.Context, runID string, after uint64) ([]*schema.RunEvent, error) {
	if _, err := a.GetRun(ctx, runID); err != nil {
		return nil, err
	}
	items, err := a.Store.ListEvents(ctx, runID, after)
	if err != nil {
		return nil, err
	}
	for _, item := range items {
		_ = json.Unmarshal([]byte(item.Payload), &item.Data)
	}
	return items, nil
}

func (a *Service) publishEvent(ctx context.Context, runID, eventType string, data any) error {
	now := time.Now().UTC()
	payloadData := make(map[string]any)
	if fields, ok := data.(map[string]any); ok {
		for key, value := range fields {
			payloadData[key] = value
		}
	} else if data != nil {
		payloadData["payload"] = data
	}
	payloadData["run_id"] = runID
	payloadData["time"] = now.Format(time.RFC3339Nano)
	payload, err := json.Marshal(payloadData)
	if err != nil {
		return err
	}
	event := &schema.RunEvent{RunID: runID, Type: eventType, Payload: string(payload), Data: payloadData, CreatedAt: now}
	if err := a.Store.CreateEvent(ctx, event); err != nil {
		return err
	}
	a.Hub.Publish(event)
	return nil
}

func (a *Service) wakeIndex() {
	select {
	case a.indexWake <- struct{}{}:
	default:
	}
}
func (a *Service) wakeRun() {
	select {
	case a.runWake <- struct{}{}:
	default:
	}
}

func (a *Service) indexLoop(ctx context.Context) {
	defer a.wg.Done()
	poll := time.Duration(max(1, config.C.Agent.WorkerPollSeconds)) * time.Second
	ticker := time.NewTicker(poll)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		case <-a.indexWake:
		}
		for {
			job, ok, err := a.Store.ClaimIngestionJob(ctx, max(1, config.C.Agent.MaxIndexAttempts))
			if err != nil || !ok {
				break
			}
			a.processIngestion(ctx, job)
		}
	}
}

func (a *Service) processIngestion(ctx context.Context, job *schema.IngestionJob) {
	if _, ok, err := a.Store.GetKnowledgeBase(ctx, job.KnowledgeBaseID, false); err != nil {
		a.finishIngestionFailure(ctx, job, "knowledge base lookup failed")
		return
	} else if !ok {
		a.finishIngestionTerminalFailure(ctx, job, "knowledge base is archived or unavailable")
		return
	}
	doc, ok, err := a.Store.GetDocument(ctx, job.DocumentID, false)
	if err != nil || !ok {
		a.finishIngestionTerminalFailure(ctx, job, "document is archived or unavailable")
		return
	}
	_ = a.Store.UpdateDocumentIndex(ctx, doc.ID, schema.IndexStatusProcessing, "", nil)
	parts := retrieval.ChunkText(doc.Content, config.C.Agent.ChunkSize, config.C.Agent.ChunkOverlap)
	existing, err := a.Store.ActiveChunkCount(ctx, doc.KnowledgeBaseID, doc.ID)
	if err != nil || len(parts) == 0 || existing+int64(len(parts)) > int64(config.C.Agent.MaxChunksPerKnowledgeBase) {
		a.finishIngestionFailure(ctx, job, "document cannot be indexed within the knowledge-base chunk limit")
		return
	}
	texts := make([]string, len(parts))
	for i := range parts {
		texts[i] = parts[i].Content
	}
	batchSize := max(1, config.C.Agent.EmbeddingBatchSize)
	var vectors [][]float32
	var usage llm.Usage
	for start := 0; start < len(texts); start += batchSize {
		end := min(len(texts), start+batchSize)
		batch, batchUsage, err := a.LLM.Embed(ctx, config.C.Agent.EmbeddingModel, texts[start:end])
		usage.Add(batchUsage)
		if err != nil || len(batch) != end-start {
			a.finishIngestionFailure(ctx, job, "OpenAI embeddings request failed")
			return
		}
		vectors = append(vectors, batch...)
	}
	now := time.Now()
	chunks := make([]*schema.Chunk, len(parts))
	for i := range parts {
		if len(vectors[i]) == 0 {
			a.finishIngestionFailure(ctx, job, "OpenAI embeddings response was invalid")
			return
		}
		chunks[i] = &schema.Chunk{ID: util.NewXID(), KnowledgeBaseID: doc.KnowledgeBaseID, DocumentID: doc.ID, Ordinal: i, Content: parts[i].Content, LineStart: parts[i].LineStart, LineEnd: parts[i].LineEnd, EmbeddingModel: config.C.Agent.EmbeddingModel, EmbeddingDimension: len(vectors[i]), Embedding: retrieval.EncodeFloat32(vectors[i]), CreatedAt: now}
	}
	err = a.Trans.Exec(ctx, func(txCtx context.Context) error {
		if _, ok, err := a.Store.LockKnowledgeBase(txCtx, doc.KnowledgeBaseID); err != nil {
			return err
		} else if !ok {
			return errIngestionSourceUnavailable
		}
		if _, ok, err := a.Store.LockDocument(txCtx, doc.ID); err != nil {
			return err
		} else if !ok {
			return errIngestionSourceUnavailable
		}
		if err := a.Store.ReplaceDocumentChunks(txCtx, doc.ID, chunks); err != nil {
			return err
		}
		if err := a.Store.UpdateDocumentIndex(txCtx, doc.ID, schema.IndexStatusReady, "", &now); err != nil {
			return err
		}
		return a.Store.FinishIngestionJob(txCtx, job.ID, schema.JobStatusCompleted, "", usage.TotalTokens, false)
	})
	if err != nil {
		if errors.Is(err, errIngestionSourceUnavailable) {
			a.finishIngestionTerminalFailure(ctx, job, "document or knowledge base was archived while indexing")
			return
		}
		a.finishIngestionFailure(ctx, job, "failed to persist the completed index")
		return
	}
	a.Cache.Invalidate(doc.KnowledgeBaseID)
}

var errIngestionSourceUnavailable = errors.New("ingestion source is unavailable")

func (a *Service) finishIngestionFailure(ctx context.Context, job *schema.IngestionJob, summary string) {
	if job.Attempts < max(1, config.C.Agent.MaxIndexAttempts) {
		_ = a.Store.FinishIngestionJob(ctx, job.ID, schema.JobStatusQueued, summary, 0, true)
		a.wakeIndex()
		return
	}
	_ = a.Store.FinishIngestionJob(ctx, job.ID, schema.JobStatusFailed, summary, 0, false)
	_ = a.Store.UpdateDocumentIndex(ctx, job.DocumentID, schema.IndexStatusFailed, summary, nil)
}

func (a *Service) finishIngestionTerminalFailure(ctx context.Context, job *schema.IngestionJob, summary string) {
	_ = a.Store.FinishIngestionJob(ctx, job.ID, schema.JobStatusFailed, summary, 0, false)
	if doc, ok, _ := a.Store.GetDocument(ctx, job.DocumentID, false); ok {
		_ = a.Store.UpdateDocumentIndex(ctx, doc.ID, schema.IndexStatusFailed, summary, nil)
	}
}

func (a *Service) runLoop(ctx context.Context) {
	defer a.wg.Done()
	ticker := time.NewTicker(time.Duration(max(1, config.C.Agent.WorkerPollSeconds)) * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		case <-a.runWake:
		}
		for {
			run, ok, err := a.Store.ClaimRun(ctx)
			if err != nil || !ok {
				break
			}
			timeoutCtx, cancel := context.WithTimeout(ctx, time.Duration(max(1, config.C.Agent.RunTimeoutSeconds))*time.Second)
			a.executeRun(timeoutCtx, run)
			cancel()
		}
	}
}
