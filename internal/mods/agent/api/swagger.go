package api

// The declarations in this file keep all Agent HTTP contracts discoverable by
// Swag without mixing documentation-only concerns into the handler logic.

// swaggerGetKnowledgeBase documents GET knowledge base.
// @Tags AgentAPI
// @Security AgentBearer
// @Param id path string true "knowledge base ID"
// @Success 200 {object} util.ResponseResult{data=schema.KnowledgeBase}
// @Router /api/v1/agent/knowledge-bases/{id} [get]
func swaggerGetKnowledgeBase() {}

// swaggerUpdateKnowledgeBase documents PUT knowledge base.
// @Tags AgentAPI
// @Security AgentBearer
// @Param id path string true "knowledge base ID"
// @Param body body schema.KnowledgeBaseForm true "knowledge base"
// @Success 200 {object} util.ResponseResult
// @Router /api/v1/agent/knowledge-bases/{id} [put]
func swaggerUpdateKnowledgeBase() {}

// swaggerArchiveKnowledgeBase documents soft archive.
// @Tags AgentAPI
// @Security AgentBearer
// @Param id path string true "knowledge base ID"
// @Success 200 {object} util.ResponseResult
// @Router /api/v1/agent/knowledge-bases/{id} [delete]
func swaggerArchiveKnowledgeBase() {}

// swaggerQueryDocuments documents document listing.
// @Tags AgentAPI
// @Security AgentBearer
// @Param id path string true "knowledge base ID"
// @Success 200 {object} util.ResponseResult{data=[]schema.Document}
// @Router /api/v1/agent/knowledge-bases/{id}/documents [get]
func swaggerQueryDocuments() {}

// swaggerUploadDocument documents asynchronous upload.
// @Tags AgentAPI
// @Security AgentBearer
// @Accept multipart/form-data
// @Param id path string true "knowledge base ID"
// @Param file formData file true "UTF-8 .txt or .md file"
// @Success 202 {object} util.ResponseResult
// @Router /api/v1/agent/knowledge-bases/{id}/documents [post]
func swaggerUploadDocument() {}

// swaggerGetDocument documents GET document.
// @Tags AgentAPI
// @Security AgentBearer
// @Param id path string true "document ID"
// @Success 200 {object} util.ResponseResult{data=schema.Document}
// @Router /api/v1/agent/documents/{id} [get]
func swaggerGetDocument() {}

// swaggerArchiveDocument documents soft archive.
// @Tags AgentAPI
// @Security AgentBearer
// @Param id path string true "document ID"
// @Success 200 {object} util.ResponseResult
// @Router /api/v1/agent/documents/{id} [delete]
func swaggerArchiveDocument() {}

// swaggerReindexDocument documents reindexing.
// @Tags AgentAPI
// @Security AgentBearer
// @Param id path string true "document ID"
// @Success 202 {object} util.ResponseResult{data=schema.IngestionJob}
// @Router /api/v1/agent/documents/{id}/reindex [post]
func swaggerReindexDocument() {}

// swaggerGetIngestionJob documents job status.
// @Tags AgentAPI
// @Security AgentBearer
// @Param id path string true "ingestion job ID"
// @Success 200 {object} util.ResponseResult{data=schema.IngestionJob}
// @Router /api/v1/agent/ingestion-jobs/{id} [get]
func swaggerGetIngestionJob() {}

// swaggerQueryConversations documents conversation listing.
// @Tags AgentAPI
// @Security AgentBearer
// @Success 200 {object} util.ResponseResult{data=[]schema.Conversation}
// @Router /api/v1/agent/conversations [get]
func swaggerQueryConversations() {}

// swaggerCreateConversation documents conversation creation.
// @Tags AgentAPI
// @Security AgentBearer
// @Param body body schema.ConversationForm true "conversation"
// @Success 200 {object} util.ResponseResult{data=schema.Conversation}
// @Router /api/v1/agent/conversations [post]
func swaggerCreateConversation() {}

// swaggerGetConversation documents conversation detail.
// @Tags AgentAPI
// @Security AgentBearer
// @Param id path string true "conversation ID"
// @Success 200 {object} util.ResponseResult{data=schema.Conversation}
// @Router /api/v1/agent/conversations/{id} [get]
func swaggerGetConversation() {}

// swaggerArchiveConversation documents conversation archive.
// @Tags AgentAPI
// @Security AgentBearer
// @Param id path string true "conversation ID"
// @Success 200 {object} util.ResponseResult
// @Router /api/v1/agent/conversations/{id} [delete]
func swaggerArchiveConversation() {}

// swaggerQueryMessages documents message history.
// @Tags AgentAPI
// @Security AgentBearer
// @Param id path string true "conversation ID"
// @Success 200 {object} util.ResponseResult{data=[]schema.Message}
// @Router /api/v1/agent/conversations/{id}/messages [get]
func swaggerQueryMessages() {}

// swaggerCreateRun documents asynchronous run creation.
// @Tags AgentAPI
// @Security AgentBearer
// @Param id path string true "conversation ID"
// @Param body body schema.RunForm true "question"
// @Success 202 {object} util.ResponseResult{data=schema.RunCreated}
// @Router /api/v1/agent/conversations/{id}/runs [post]
func swaggerCreateRun() {}

// swaggerGetRun documents run state and trace.
// @Tags AgentAPI
// @Security AgentBearer
// @Param id path string true "run ID"
// @Success 200 {object} util.ResponseResult{data=schema.Run}
// @Router /api/v1/agent/runs/{id} [get]
func swaggerGetRun() {}

// swaggerStreamEvents documents replayable SSE.
// @Tags AgentAPI
// @Security AgentBearer
// @Produce text/event-stream
// @Param id path string true "run ID"
// @Param after query int false "last durable event ID"
// @Success 200 {string} string "SSE event stream"
// @Router /api/v1/agent/runs/{id}/events [get]
func swaggerStreamEvents() {}
