package dal

import (
	"context"
	"time"

	"github.com/LyricTian/gin-admin/v10/internal/mods/agent/retrieval"
	"github.com/LyricTian/gin-admin/v10/internal/mods/agent/schema"
	"github.com/LyricTian/gin-admin/v10/pkg/errors"
	"github.com/LyricTian/gin-admin/v10/pkg/util"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type Store struct{ DB *gorm.DB }

func (a *Store) db(ctx context.Context) *gorm.DB { return util.GetDB(ctx, a.DB) }

func (a *Store) CreateKnowledgeBase(ctx context.Context, item *schema.KnowledgeBase) error {
	return errors.WithStack(a.db(ctx).Create(item).Error)
}

func (a *Store) QueryKnowledgeBases(ctx context.Context, params schema.KnowledgeBaseQuery) ([]*schema.KnowledgeBase, *util.PaginationResult, error) {
	db := a.db(ctx).Model(new(schema.KnowledgeBase)).Where("archived_at IS NULL")
	if params.Name != "" {
		db = db.Where("name LIKE ?", "%"+params.Name+"%")
	}
	var items []*schema.KnowledgeBase
	params.Pagination = true
	pr, err := util.WrapPageQuery(ctx, db, params.PaginationParam, util.QueryOptions{OrderFields: []util.OrderByParam{{Field: "created_at", Direction: util.DESC}}}, &items)
	return items, pr, errors.WithStack(err)
}

func (a *Store) GetKnowledgeBase(ctx context.Context, id string, includeArchived bool) (*schema.KnowledgeBase, bool, error) {
	db := a.db(ctx).Where("id = ?", id)
	if !includeArchived {
		db = db.Where("archived_at IS NULL")
	}
	item := new(schema.KnowledgeBase)
	ok, err := util.FindOne(ctx, db, util.QueryOptions{}, item)
	return item, ok, errors.WithStack(err)
}

func (a *Store) LockKnowledgeBase(ctx context.Context, id string) (*schema.KnowledgeBase, bool, error) {
	lockedCtx := util.NewRowLock(ctx)
	item := new(schema.KnowledgeBase)
	ok, err := util.FindOne(lockedCtx, a.db(lockedCtx).Where("id = ? AND archived_at IS NULL", id), util.QueryOptions{}, item)
	return item, ok, errors.WithStack(err)
}

func (a *Store) UpdateKnowledgeBase(ctx context.Context, item *schema.KnowledgeBase) error {
	return errors.WithStack(a.db(ctx).Model(item).Select("name", "description", "updated_at").Updates(item).Error)
}

func (a *Store) ArchiveKnowledgeBase(ctx context.Context, id string, now time.Time) error {
	return errors.WithStack(a.db(ctx).Model(new(schema.KnowledgeBase)).Where("id = ? AND archived_at IS NULL", id).Updates(map[string]any{"status": schema.StatusArchived, "archived_at": now, "updated_at": now}).Error)
}

func (a *Store) CreateDocument(ctx context.Context, item *schema.Document) error {
	return errors.WithStack(a.db(ctx).Create(item).Error)
}

func (a *Store) ActiveDocumentSHAExists(ctx context.Context, knowledgeBaseID, sha string) (bool, error) {
	ok, err := util.Exists(ctx, a.db(ctx).Model(new(schema.Document)).Where("knowledge_base_id = ? AND sha256 = ? AND archived_at IS NULL", knowledgeBaseID, sha))
	return ok, errors.WithStack(err)
}

func (a *Store) QueryDocuments(ctx context.Context, knowledgeBaseID string, params schema.DocumentQuery) ([]*schema.Document, *util.PaginationResult, error) {
	var items []*schema.Document
	params.Pagination = true
	pr, err := util.WrapPageQuery(ctx, a.db(ctx).Model(new(schema.Document)).Where("knowledge_base_id = ? AND archived_at IS NULL", knowledgeBaseID), params.PaginationParam, util.QueryOptions{OrderFields: []util.OrderByParam{{Field: "created_at", Direction: util.DESC}}}, &items)
	return items, pr, errors.WithStack(err)
}

func (a *Store) GetDocument(ctx context.Context, id string, includeArchived bool) (*schema.Document, bool, error) {
	db := a.db(ctx).Where("id = ?", id)
	if !includeArchived {
		db = db.Where("archived_at IS NULL")
	}
	item := new(schema.Document)
	ok, err := util.FindOne(ctx, db, util.QueryOptions{}, item)
	return item, ok, errors.WithStack(err)
}

func (a *Store) LockDocument(ctx context.Context, id string) (*schema.Document, bool, error) {
	lockedCtx := util.NewRowLock(ctx)
	item := new(schema.Document)
	ok, err := util.FindOne(lockedCtx, a.db(lockedCtx).Where("id = ? AND archived_at IS NULL", id), util.QueryOptions{}, item)
	return item, ok, errors.WithStack(err)
}

func (a *Store) UpdateDocumentIndex(ctx context.Context, id, status, summary string, indexedAt *time.Time) error {
	return errors.WithStack(a.db(ctx).Model(new(schema.Document)).Where("id = ?", id).Updates(map[string]any{"index_status": status, "error_summary": summary, "indexed_at": indexedAt, "updated_at": time.Now()}).Error)
}

func (a *Store) ArchiveDocument(ctx context.Context, id string, now time.Time) error {
	return errors.WithStack(a.db(ctx).Model(new(schema.Document)).Where("id = ? AND archived_at IS NULL", id).Updates(map[string]any{"archived_at": now, "index_status": schema.StatusArchived, "updated_at": now}).Error)
}

func (a *Store) ActiveChunkCount(ctx context.Context, knowledgeBaseID, excludeDocumentID string) (int64, error) {
	db := a.db(ctx).Model(new(schema.Chunk)).Joins("JOIN "+new(schema.Document).TableName()+" d ON d.id = "+new(schema.Chunk).TableName()+".document_id").Where(new(schema.Chunk).TableName()+".knowledge_base_id = ? AND d.archived_at IS NULL", knowledgeBaseID)
	if excludeDocumentID != "" {
		db = db.Where(new(schema.Chunk).TableName()+".document_id <> ?", excludeDocumentID)
	}
	var count int64
	err := db.Count(&count).Error
	return count, errors.WithStack(err)
}

func (a *Store) ReplaceDocumentChunks(ctx context.Context, documentID string, chunks []*schema.Chunk) error {
	if err := a.db(ctx).Where("document_id = ?", documentID).Delete(new(schema.Chunk)).Error; err != nil {
		return errors.WithStack(err)
	}
	if len(chunks) == 0 {
		return nil
	}
	return errors.WithStack(a.db(ctx).CreateInBatches(chunks, 100).Error)
}

func (a *Store) DeleteDocumentChunks(ctx context.Context, documentID string) error {
	return errors.WithStack(a.db(ctx).Where("document_id = ?", documentID).Delete(new(schema.Chunk)).Error)
}

func (a *Store) ListVectorRecords(ctx context.Context, knowledgeBaseID string) ([]retrieval.VectorRecord, error) {
	type row struct {
		schema.Chunk
		DocumentName string
	}
	var rows []row
	err := a.db(ctx).Table(new(schema.Chunk).TableName()+" c").
		Select("c.*, d.original_name AS document_name").
		Joins("JOIN "+new(schema.Document).TableName()+" d ON d.id = c.document_id").
		Where("c.knowledge_base_id = ? AND d.archived_at IS NULL AND d.index_status = ?", knowledgeBaseID, schema.IndexStatusReady).
		Order("c.document_id ASC, c.ordinal ASC").Scan(&rows).Error
	if err != nil {
		return nil, errors.WithStack(err)
	}
	result := make([]retrieval.VectorRecord, 0, len(rows))
	for _, row := range rows {
		vector, err := retrieval.DecodeFloat32(row.Embedding)
		if err != nil || len(vector) != row.EmbeddingDimension {
			continue
		}
		result = append(result, retrieval.VectorRecord{ChunkID: row.ID, DocumentID: row.DocumentID, DocumentName: row.DocumentName, Content: row.Content, LineStart: row.LineStart, LineEnd: row.LineEnd, Vector: vector})
	}
	return result, nil
}

func (a *Store) CreateIngestionJob(ctx context.Context, item *schema.IngestionJob) error {
	return errors.WithStack(a.db(ctx).Create(item).Error)
}

func (a *Store) GetIngestionJob(ctx context.Context, id string) (*schema.IngestionJob, bool, error) {
	item := new(schema.IngestionJob)
	ok, err := util.FindOne(ctx, a.db(ctx).Where("id = ?", id), util.QueryOptions{}, item)
	return item, ok, errors.WithStack(err)
}

func (a *Store) RequeueInterruptedIngestion(ctx context.Context) error {
	now := time.Now()
	return errors.WithStack(a.db(ctx).Model(new(schema.IngestionJob)).Where("status = ?", schema.JobStatusProcessing).Updates(map[string]any{"status": schema.JobStatusQueued, "started_at": nil, "updated_at": now}).Error)
}

func (a *Store) ClaimIngestionJob(ctx context.Context, maxAttempts int) (*schema.IngestionJob, bool, error) {
	item := new(schema.IngestionJob)
	err := a.db(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("status = ? AND attempts < ?", schema.JobStatusQueued, maxAttempts).Order("created_at ASC").First(item).Error; err != nil {
			return err
		}
		now := time.Now()
		item.Status, item.Attempts, item.StartedAt, item.UpdatedAt = schema.JobStatusProcessing, item.Attempts+1, &now, now
		return tx.Model(item).Select("status", "attempts", "started_at", "updated_at").Updates(item).Error
	})
	if err == gorm.ErrRecordNotFound {
		return nil, false, nil
	}
	return item, err == nil, errors.WithStack(err)
}

func (a *Store) FinishIngestionJob(ctx context.Context, id, status, summary string, inputTokens int64, retry bool) error {
	now := time.Now()
	updates := map[string]any{"status": status, "error_summary": summary, "input_tokens": inputTokens, "updated_at": now}
	if !retry {
		updates["completed_at"] = now
	}
	return errors.WithStack(a.db(ctx).Model(new(schema.IngestionJob)).Where("id = ?", id).Updates(updates).Error)
}

func (a *Store) CreateConversation(ctx context.Context, item *schema.Conversation) error {
	return errors.WithStack(a.db(ctx).Create(item).Error)
}

func (a *Store) QueryConversations(ctx context.Context, params schema.ConversationQuery) ([]*schema.Conversation, *util.PaginationResult, error) {
	var items []*schema.Conversation
	params.Pagination = true
	pr, err := util.WrapPageQuery(ctx, a.db(ctx).Model(new(schema.Conversation)).Where("archived_at IS NULL"), params.PaginationParam, util.QueryOptions{OrderFields: []util.OrderByParam{{Field: "updated_at", Direction: util.DESC}}}, &items)
	return items, pr, errors.WithStack(err)
}

func (a *Store) GetConversation(ctx context.Context, id string) (*schema.Conversation, bool, error) {
	item := new(schema.Conversation)
	ok, err := util.FindOne(ctx, a.db(ctx).Where("id = ? AND archived_at IS NULL", id), util.QueryOptions{}, item)
	return item, ok, errors.WithStack(err)
}

func (a *Store) ArchiveConversation(ctx context.Context, id string, now time.Time) error {
	return errors.WithStack(a.db(ctx).Model(new(schema.Conversation)).Where("id = ? AND archived_at IS NULL", id).Updates(map[string]any{"archived_at": now, "updated_at": now}).Error)
}

func (a *Store) CreateMessage(ctx context.Context, item *schema.Message) error {
	return errors.WithStack(a.db(ctx).Create(item).Error)
}

func (a *Store) GetMessage(ctx context.Context, id string) (*schema.Message, bool, error) {
	item := new(schema.Message)
	ok, err := util.FindOne(ctx, a.db(ctx).Where("id = ?", id), util.QueryOptions{}, item)
	return item, ok, errors.WithStack(err)
}

func (a *Store) QueryMessages(ctx context.Context, conversationID string, params schema.MessageQuery) ([]*schema.Message, *util.PaginationResult, error) {
	var items []*schema.Message
	params.Pagination = true
	pr, err := util.WrapPageQuery(ctx, a.db(ctx).Model(new(schema.Message)).Where("conversation_id = ?", conversationID), params.PaginationParam, util.QueryOptions{OrderFields: []util.OrderByParam{{Field: "created_at", Direction: util.ASC}}}, &items)
	if err != nil {
		return nil, nil, errors.WithStack(err)
	}
	for _, item := range items {
		if item.Role == schema.RoleAssistant {
			if err := a.db(ctx).Where("message_id = ?", item.ID).Order("created_at ASC").Find(&item.Citations).Error; err != nil {
				return nil, nil, errors.WithStack(err)
			}
		}
	}
	return items, pr, nil
}

func (a *Store) RecentMessages(ctx context.Context, conversationID string, limit int) ([]*schema.Message, error) {
	var reverse []*schema.Message
	err := a.db(ctx).Where("conversation_id = ?", conversationID).Order("created_at DESC").Limit(limit).Find(&reverse).Error
	if err != nil {
		return nil, errors.WithStack(err)
	}
	items := make([]*schema.Message, len(reverse))
	for i := range reverse {
		items[len(reverse)-1-i] = reverse[i]
	}
	return items, nil
}

func (a *Store) CreateRun(ctx context.Context, item *schema.Run) error {
	return errors.WithStack(a.db(ctx).Create(item).Error)
}

func (a *Store) GetRun(ctx context.Context, id string) (*schema.Run, bool, error) {
	item := new(schema.Run)
	ok, err := util.FindOne(ctx, a.db(ctx).Where("id = ?", id), util.QueryOptions{}, item)
	if err != nil || !ok {
		return item, ok, errors.WithStack(err)
	}
	if err := a.db(ctx).Where("run_id = ?", id).Order("ordinal ASC").Find(&item.Steps).Error; err != nil {
		return item, false, errors.WithStack(err)
	}
	if item.FinalMessageID != "" {
		msg := new(schema.Message)
		found, findErr := util.FindOne(ctx, a.db(ctx).Where("id = ?", item.FinalMessageID), util.QueryOptions{}, msg)
		if findErr != nil {
			return item, false, errors.WithStack(findErr)
		}
		if found {
			if err := a.db(ctx).Where("message_id = ?", msg.ID).Order("created_at ASC").Find(&msg.Citations).Error; err != nil {
				return item, false, errors.WithStack(err)
			}
			item.FinalMessage = msg
		}
	}
	return item, true, nil
}

func (a *Store) InterruptRunningRuns(ctx context.Context) ([]string, error) {
	now := time.Now()
	var runIDs []string
	err := a.db(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Model(new(schema.Run)).Where("status = ?", schema.RunStatusRunning).Pluck("id", &runIDs).Error; err != nil || len(runIDs) == 0 {
			return err
		}
		if err := tx.Model(new(schema.Run)).Where("id IN ?", runIDs).Updates(map[string]any{"status": schema.RunStatusInterrupted, "error_summary": "run interrupted by service restart", "completed_at": now, "updated_at": now}).Error; err != nil {
			return err
		}
		return tx.Model(new(schema.RunStep)).Where("run_id IN ? AND status = ?", runIDs, schema.StepStatusRunning).Updates(map[string]any{"status": schema.StepStatusFailed, "summary": "step interrupted by service restart", "completed_at": now, "updated_at": now}).Error
	})
	return runIDs, errors.WithStack(err)
}

func (a *Store) ClaimRun(ctx context.Context) (*schema.Run, bool, error) {
	item := new(schema.Run)
	err := a.db(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("status = ?", schema.RunStatusQueued).Order("created_at ASC").First(item).Error; err != nil {
			return err
		}
		now := time.Now()
		item.Status, item.StartedAt, item.UpdatedAt = schema.RunStatusRunning, &now, now
		return tx.Model(item).Select("status", "started_at", "updated_at").Updates(item).Error
	})
	if err == gorm.ErrRecordNotFound {
		return nil, false, nil
	}
	return item, err == nil, errors.WithStack(err)
}

func (a *Store) UpdateRun(ctx context.Context, item *schema.Run) error {
	return errors.WithStack(a.db(ctx).Model(item).Select("status", "revision_count", "final_message_id", "input_tokens", "output_tokens", "total_tokens", "error_summary", "completed_at", "updated_at").Updates(item).Error)
}

func (a *Store) CreateStep(ctx context.Context, item *schema.RunStep) error {
	return errors.WithStack(a.db(ctx).Create(item).Error)
}

func (a *Store) UpdateStep(ctx context.Context, item *schema.RunStep) error {
	return errors.WithStack(a.db(ctx).Model(item).Select("status", "summary", "input_tokens", "output_tokens", "total_tokens", "duration_ms", "completed_at", "updated_at").Updates(item).Error)
}

func (a *Store) CreateEvent(ctx context.Context, item *schema.RunEvent) error {
	return errors.WithStack(a.db(ctx).Create(item).Error)
}

func (a *Store) ListEvents(ctx context.Context, runID string, after uint64) ([]*schema.RunEvent, error) {
	var items []*schema.RunEvent
	err := a.db(ctx).Where("run_id = ? AND id > ?", runID, after).Order("id ASC").Find(&items).Error
	return items, errors.WithStack(err)
}

func (a *Store) CreateCitations(ctx context.Context, items []*schema.Citation) error {
	if len(items) == 0 {
		return nil
	}
	return errors.WithStack(a.db(ctx).Create(items).Error)
}
