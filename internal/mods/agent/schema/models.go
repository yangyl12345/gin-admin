package schema

import (
	"time"

	"github.com/LyricTian/gin-admin/v10/internal/config"
	"github.com/LyricTian/gin-admin/v10/pkg/util"
)

const (
	StatusActive   = "active"
	StatusArchived = "archived"

	IndexStatusPending    = "pending"
	IndexStatusProcessing = "processing"
	IndexStatusReady      = "ready"
	IndexStatusFailed     = "failed"

	JobStatusQueued     = "queued"
	JobStatusProcessing = "processing"
	JobStatusCompleted  = "completed"
	JobStatusFailed     = "failed"

	RunStatusQueued       = "queued"
	RunStatusRunning      = "running"
	RunStatusCompleted    = "completed"
	RunStatusFailed       = "failed"
	RunStatusFailedReview = "failed_review"
	RunStatusInterrupted  = "interrupted"

	StepStatusRunning   = "running"
	StepStatusCompleted = "completed"
	StepStatusFailed    = "failed"

	RoleUser      = "user"
	RoleAssistant = "assistant"
)

type KnowledgeBase struct {
	ID          string     `gorm:"size:20;primaryKey" json:"id"`
	Name        string     `gorm:"size:128;not null" json:"name"`
	Description string     `gorm:"size:2048" json:"description,omitempty"`
	Status      string     `gorm:"size:20;index;not null" json:"status"`
	ArchivedAt  *time.Time `gorm:"index" json:"archived_at,omitempty"`
	CreatedAt   time.Time  `gorm:"index;not null" json:"created_at"`
	UpdatedAt   time.Time  `json:"updated_at"`
}

func (*KnowledgeBase) TableName() string {
	return config.C.FormatTableName("agent_knowledge_bases")
}

type Document struct {
	ID              string     `gorm:"size:20;primaryKey" json:"id"`
	KnowledgeBaseID string     `gorm:"size:20;index;not null" json:"knowledge_base_id"`
	OriginalName    string     `gorm:"size:255;not null" json:"original_name"`
	MediaType       string     `gorm:"size:64;not null" json:"media_type"`
	SizeBytes       int64      `gorm:"not null" json:"size_bytes"`
	SHA256          string     `gorm:"size:64;index;not null" json:"sha256"`
	Content         string     `gorm:"not null" json:"-"`
	IndexStatus     string     `gorm:"size:20;index;not null" json:"index_status"`
	ErrorSummary    string     `gorm:"size:1024" json:"error_summary,omitempty"`
	IndexedAt       *time.Time `gorm:"index" json:"indexed_at,omitempty"`
	ArchivedAt      *time.Time `gorm:"index" json:"archived_at,omitempty"`
	CreatedAt       time.Time  `gorm:"index;not null" json:"created_at"`
	UpdatedAt       time.Time  `json:"updated_at"`
}

func (*Document) TableName() string { return config.C.FormatTableName("agent_documents") }

type Chunk struct {
	ID                 string    `gorm:"size:20;primaryKey" json:"id"`
	KnowledgeBaseID    string    `gorm:"size:20;index;not null" json:"knowledge_base_id"`
	DocumentID         string    `gorm:"size:20;index;not null;uniqueIndex:idx_agent_chunk_ordinal,priority:1" json:"document_id"`
	Ordinal            int       `gorm:"not null;uniqueIndex:idx_agent_chunk_ordinal,priority:2" json:"ordinal"`
	Content            string    `gorm:"type:text;not null" json:"content"`
	LineStart          int       `gorm:"not null" json:"line_start"`
	LineEnd            int       `gorm:"not null" json:"line_end"`
	EmbeddingModel     string    `gorm:"size:64;not null" json:"embedding_model"`
	EmbeddingDimension int       `gorm:"not null" json:"embedding_dimension"`
	Embedding          []byte    `gorm:"not null" json:"-"`
	CreatedAt          time.Time `gorm:"index;not null" json:"created_at"`
}

func (*Chunk) TableName() string { return config.C.FormatTableName("agent_chunks") }

type IngestionJob struct {
	ID              string     `gorm:"size:20;primaryKey" json:"id"`
	DocumentID      string     `gorm:"size:20;index;not null" json:"document_id"`
	KnowledgeBaseID string     `gorm:"size:20;index;not null" json:"knowledge_base_id"`
	Status          string     `gorm:"size:20;index;not null" json:"status"`
	Attempts        int        `gorm:"not null" json:"attempts"`
	InputTokens     int64      `gorm:"not null" json:"input_tokens"`
	ErrorSummary    string     `gorm:"size:1024" json:"error_summary,omitempty"`
	StartedAt       *time.Time `gorm:"index" json:"started_at,omitempty"`
	CompletedAt     *time.Time `gorm:"index" json:"completed_at,omitempty"`
	CreatedAt       time.Time  `gorm:"index;not null" json:"created_at"`
	UpdatedAt       time.Time  `json:"updated_at"`
}

func (*IngestionJob) TableName() string {
	return config.C.FormatTableName("agent_ingestion_jobs")
}

type Conversation struct {
	ID              string     `gorm:"size:20;primaryKey" json:"id"`
	KnowledgeBaseID string     `gorm:"size:20;index;not null" json:"knowledge_base_id"`
	Title           string     `gorm:"size:255;not null" json:"title"`
	ArchivedAt      *time.Time `gorm:"index" json:"archived_at,omitempty"`
	CreatedAt       time.Time  `gorm:"index;not null" json:"created_at"`
	UpdatedAt       time.Time  `json:"updated_at"`
}

func (*Conversation) TableName() string {
	return config.C.FormatTableName("agent_conversations")
}

type Message struct {
	ID             string     `gorm:"size:20;primaryKey" json:"id"`
	ConversationID string     `gorm:"size:20;index;not null" json:"conversation_id"`
	RunID          string     `gorm:"size:20;index" json:"run_id,omitempty"`
	Role           string     `gorm:"size:20;index;not null" json:"role"`
	Content        string     `gorm:"not null" json:"content"`
	CreatedAt      time.Time  `gorm:"index;not null" json:"created_at"`
	Citations      []Citation `gorm:"-" json:"citations,omitempty"`
}

func (*Message) TableName() string { return config.C.FormatTableName("agent_messages") }

type Run struct {
	ID              string     `gorm:"size:20;primaryKey" json:"id"`
	ConversationID  string     `gorm:"size:20;index;not null" json:"conversation_id"`
	KnowledgeBaseID string     `gorm:"size:20;index;not null" json:"knowledge_base_id"`
	UserMessageID   string     `gorm:"size:20;index;not null" json:"user_message_id"`
	FinalMessageID  string     `gorm:"size:20;index" json:"final_message_id,omitempty"`
	Status          string     `gorm:"size:24;index;not null" json:"status"`
	RevisionCount   int        `gorm:"not null" json:"revision_count"`
	InputTokens     int64      `gorm:"not null" json:"input_tokens"`
	OutputTokens    int64      `gorm:"not null" json:"output_tokens"`
	TotalTokens     int64      `gorm:"not null" json:"total_tokens"`
	ErrorSummary    string     `gorm:"size:1024" json:"error_summary,omitempty"`
	StartedAt       *time.Time `gorm:"index" json:"started_at,omitempty"`
	CompletedAt     *time.Time `gorm:"index" json:"completed_at,omitempty"`
	CreatedAt       time.Time  `gorm:"index;not null" json:"created_at"`
	UpdatedAt       time.Time  `json:"updated_at"`
	Steps           []RunStep  `gorm:"-" json:"steps,omitempty"`
	FinalMessage    *Message   `gorm:"-" json:"final_message,omitempty"`
}

func (*Run) TableName() string { return config.C.FormatTableName("agent_runs") }

type RunStep struct {
	ID           string     `gorm:"size:20;primaryKey" json:"id"`
	RunID        string     `gorm:"size:20;index;not null;uniqueIndex:idx_agent_run_step,priority:1" json:"run_id"`
	Ordinal      int        `gorm:"not null;uniqueIndex:idx_agent_run_step,priority:2" json:"ordinal"`
	Role         string     `gorm:"size:24;index;not null" json:"role"`
	Model        string     `gorm:"size:64" json:"model,omitempty"`
	Status       string     `gorm:"size:20;index;not null" json:"status"`
	Summary      string     `gorm:"size:2048" json:"summary,omitempty"`
	InputTokens  int64      `gorm:"not null" json:"input_tokens"`
	OutputTokens int64      `gorm:"not null" json:"output_tokens"`
	TotalTokens  int64      `gorm:"not null" json:"total_tokens"`
	DurationMS   int64      `gorm:"not null" json:"duration_ms"`
	StartedAt    *time.Time `gorm:"index" json:"started_at,omitempty"`
	CompletedAt  *time.Time `gorm:"index" json:"completed_at,omitempty"`
	CreatedAt    time.Time  `gorm:"index;not null" json:"created_at"`
	UpdatedAt    time.Time  `json:"updated_at"`
}

func (*RunStep) TableName() string { return config.C.FormatTableName("agent_run_steps") }

type RunEvent struct {
	ID        uint64    `gorm:"primaryKey;autoIncrement" json:"id"`
	RunID     string    `gorm:"size:20;index;not null" json:"run_id"`
	Type      string    `gorm:"size:40;index;not null" json:"type"`
	Payload   string    `gorm:"type:text;not null" json:"-"`
	Data      any       `gorm:"-" json:"data,omitempty"`
	CreatedAt time.Time `gorm:"index;not null" json:"created_at"`
}

func (*RunEvent) TableName() string { return config.C.FormatTableName("agent_run_events") }

type Citation struct {
	ID           string    `gorm:"size:20;primaryKey" json:"id,omitempty"`
	MessageID    string    `gorm:"size:20;index;not null" json:"-"`
	DocumentID   string    `gorm:"size:20;index;not null" json:"document_id"`
	DocumentName string    `gorm:"size:255;not null" json:"document_name"`
	ChunkID      string    `gorm:"size:20;index;not null" json:"chunk_id"`
	LineStart    int       `gorm:"not null" json:"line_start"`
	LineEnd      int       `gorm:"not null" json:"line_end"`
	Quote        string    `gorm:"size:2048;not null" json:"quote"`
	Score        float64   `gorm:"not null" json:"score"`
	CreatedAt    time.Time `gorm:"index;not null" json:"created_at"`
}

func (*Citation) TableName() string { return config.C.FormatTableName("agent_citations") }

type KnowledgeBaseForm struct {
	Name        string `json:"name" binding:"required,max=128"`
	Description string `json:"description" binding:"max=2048"`
}

type KnowledgeBaseQuery struct {
	util.PaginationParam
	Name string `form:"name"`
}

type DocumentQuery struct{ util.PaginationParam }

type ConversationForm struct {
	KnowledgeBaseID string `json:"knowledge_base_id" binding:"required"`
	Title           string `json:"title" binding:"max=255"`
}

type ConversationQuery struct{ util.PaginationParam }

type MessageQuery struct{ util.PaginationParam }

type RunForm struct {
	Content string `json:"content" binding:"required,max=20000"`
}

type RunCreated struct {
	RunID     string `json:"run_id"`
	MessageID string `json:"message_id"`
	Status    string `json:"status"`
}

type Status struct {
	Name           string            `json:"name"`
	Enabled        bool              `json:"enabled"`
	WorkersRunning bool              `json:"workers_running"`
	Models         map[string]string `json:"models"`
	EmbeddingModel string            `json:"embedding_model"`
}

type RetrievalHit struct {
	ChunkID      string  `json:"chunk_id"`
	DocumentID   string  `json:"document_id"`
	DocumentName string  `json:"document_name"`
	Content      string  `json:"content"`
	LineStart    int     `json:"line_start"`
	LineEnd      int     `json:"line_end"`
	Score        float64 `json:"score"`
}
