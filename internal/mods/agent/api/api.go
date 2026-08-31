package api

import (
	"crypto/sha256"
	"crypto/subtle"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/LyricTian/gin-admin/v10/internal/config"
	"github.com/LyricTian/gin-admin/v10/internal/mods/agent/biz"
	"github.com/LyricTian/gin-admin/v10/internal/mods/agent/schema"
	projecterrors "github.com/LyricTian/gin-admin/v10/pkg/errors"
	"github.com/LyricTian/gin-admin/v10/pkg/util"
	"github.com/gin-gonic/gin"
)

type API struct{ Service *biz.Service }

func (a *API) RequireEnabled() gin.HandlerFunc {
	return func(c *gin.Context) {
		if !config.C.Agent.Enable {
			util.ResError(c, projecterrors.New("agent_disabled", "Agent is disabled", http.StatusServiceUnavailable))
			return
		}
		c.Next()
	}
}

func (a *API) Auth() gin.HandlerFunc {
	return func(c *gin.Context) {
		header := c.GetHeader("Authorization")
		parts := strings.SplitN(header, " ", 2)
		provided := ""
		schemeOK := len(parts) == 2 && strings.EqualFold(parts[0], "Bearer")
		if schemeOK {
			provided = parts[1]
		}
		expected := os.Getenv("AGENT_API_KEY")
		providedHash := sha256.Sum256([]byte(provided))
		expectedHash := sha256.Sum256([]byte(expected))
		if !schemeOK || expected == "" || subtle.ConstantTimeCompare(providedHash[:], expectedHash[:]) != 1 {
			util.ResError(c, projecterrors.New("agent_unauthorized", "Unauthorized", http.StatusUnauthorized))
			return
		}
		c.Next()
	}
}

// @Tags AgentAPI
// @Summary Get non-sensitive Agent status
// @Success 200 {object} util.ResponseResult{data=schema.Status}
// @Router /api/v1/agent/status [get]
func (a *API) Status(c *gin.Context) { util.ResSuccess(c, a.Service.Status()) }

// @Tags AgentAPI
// @Summary List knowledge bases
// @Security AgentBearer
// @Param current query int false "pagination index"
// @Param pageSize query int false "pagination size"
// @Success 200 {object} util.ResponseResult{data=[]schema.KnowledgeBase}
// @Router /api/v1/agent/knowledge-bases [get]
func (a *API) QueryKnowledgeBases(c *gin.Context) {
	var params schema.KnowledgeBaseQuery
	if err := util.ParseQuery(c, &params); err != nil {
		util.ResError(c, err)
		return
	}
	items, page, err := a.Service.QueryKnowledgeBases(c.Request.Context(), params)
	if err != nil {
		util.ResError(c, err)
		return
	}
	util.ResPage(c, items, page)
}

// @Tags AgentAPI
// @Summary Create knowledge base
// @Security AgentBearer
// @Param body body schema.KnowledgeBaseForm true "knowledge base"
// @Success 200 {object} util.ResponseResult{data=schema.KnowledgeBase}
// @Router /api/v1/agent/knowledge-bases [post]
func (a *API) CreateKnowledgeBase(c *gin.Context) {
	form := new(schema.KnowledgeBaseForm)
	if err := util.ParseJSON(c, form); err != nil {
		util.ResError(c, err)
		return
	}
	item, err := a.Service.CreateKnowledgeBase(c.Request.Context(), form)
	if err != nil {
		util.ResError(c, err)
		return
	}
	util.ResSuccess(c, item)
}

func (a *API) GetKnowledgeBase(c *gin.Context) {
	item, err := a.Service.GetKnowledgeBase(c.Request.Context(), c.Param("id"))
	if err != nil {
		util.ResError(c, err)
		return
	}
	util.ResSuccess(c, item)
}

func (a *API) UpdateKnowledgeBase(c *gin.Context) {
	form := new(schema.KnowledgeBaseForm)
	if err := util.ParseJSON(c, form); err != nil {
		util.ResError(c, err)
		return
	}
	if err := a.Service.UpdateKnowledgeBase(c.Request.Context(), c.Param("id"), form); err != nil {
		util.ResError(c, err)
		return
	}
	util.ResOK(c)
}

func (a *API) ArchiveKnowledgeBase(c *gin.Context) {
	if err := a.Service.ArchiveKnowledgeBase(c.Request.Context(), c.Param("id")); err != nil {
		util.ResError(c, err)
		return
	}
	util.ResOK(c)
}

func (a *API) QueryDocuments(c *gin.Context) {
	var params schema.DocumentQuery
	if err := util.ParseQuery(c, &params); err != nil {
		util.ResError(c, err)
		return
	}
	items, page, err := a.Service.QueryDocuments(c.Request.Context(), c.Param("id"), params)
	if err != nil {
		util.ResError(c, err)
		return
	}
	util.ResPage(c, items, page)
}

func (a *API) UploadDocument(c *gin.Context) {
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, config.C.Agent.MaxUploadBytes+1024*1024)
	header, err := c.FormFile("file")
	if err != nil {
		util.ResError(c, projecterrors.BadRequest("agent_file_required", "multipart field file is required"))
		return
	}
	file, err := header.Open()
	if err != nil {
		util.ResError(c, projecterrors.BadRequest("agent_file_unreadable", "uploaded file cannot be read"))
		return
	}
	defer file.Close()
	content, err := io.ReadAll(io.LimitReader(file, config.C.Agent.MaxUploadBytes+1))
	if err != nil {
		util.ResError(c, projecterrors.BadRequest("agent_file_unreadable", "uploaded file cannot be read"))
		return
	}
	doc, job, err := a.Service.UploadDocument(c.Request.Context(), c.Param("id"), header.Filename, content)
	if err != nil {
		util.ResError(c, err)
		return
	}
	accepted(c, map[string]any{"document": doc, "ingestion_job": job})
}

func (a *API) GetDocument(c *gin.Context) {
	item, err := a.Service.GetDocument(c.Request.Context(), c.Param("id"))
	if err != nil {
		util.ResError(c, err)
		return
	}
	util.ResSuccess(c, item)
}

func (a *API) ArchiveDocument(c *gin.Context) {
	if err := a.Service.ArchiveDocument(c.Request.Context(), c.Param("id")); err != nil {
		util.ResError(c, err)
		return
	}
	util.ResOK(c)
}

func (a *API) ReindexDocument(c *gin.Context) {
	job, err := a.Service.ReindexDocument(c.Request.Context(), c.Param("id"))
	if err != nil {
		util.ResError(c, err)
		return
	}
	accepted(c, job)
}

func (a *API) GetIngestionJob(c *gin.Context) {
	item, err := a.Service.GetIngestionJob(c.Request.Context(), c.Param("id"))
	if err != nil {
		util.ResError(c, err)
		return
	}
	util.ResSuccess(c, item)
}

func (a *API) QueryConversations(c *gin.Context) {
	var params schema.ConversationQuery
	if err := util.ParseQuery(c, &params); err != nil {
		util.ResError(c, err)
		return
	}
	items, page, err := a.Service.QueryConversations(c.Request.Context(), params)
	if err != nil {
		util.ResError(c, err)
		return
	}
	util.ResPage(c, items, page)
}

func (a *API) CreateConversation(c *gin.Context) {
	form := new(schema.ConversationForm)
	if err := util.ParseJSON(c, form); err != nil {
		util.ResError(c, err)
		return
	}
	item, err := a.Service.CreateConversation(c.Request.Context(), form)
	if err != nil {
		util.ResError(c, err)
		return
	}
	util.ResSuccess(c, item)
}

func (a *API) GetConversation(c *gin.Context) {
	item, err := a.Service.GetConversation(c.Request.Context(), c.Param("id"))
	if err != nil {
		util.ResError(c, err)
		return
	}
	util.ResSuccess(c, item)
}

func (a *API) ArchiveConversation(c *gin.Context) {
	if err := a.Service.ArchiveConversation(c.Request.Context(), c.Param("id")); err != nil {
		util.ResError(c, err)
		return
	}
	util.ResOK(c)
}

func (a *API) QueryMessages(c *gin.Context) {
	var params schema.MessageQuery
	if err := util.ParseQuery(c, &params); err != nil {
		util.ResError(c, err)
		return
	}
	items, page, err := a.Service.QueryMessages(c.Request.Context(), c.Param("id"), params)
	if err != nil {
		util.ResError(c, err)
		return
	}
	util.ResPage(c, items, page)
}

func (a *API) CreateRun(c *gin.Context) {
	form := new(schema.RunForm)
	if err := util.ParseJSON(c, form); err != nil {
		util.ResError(c, err)
		return
	}
	item, err := a.Service.CreateRun(c.Request.Context(), c.Param("id"), form)
	if err != nil {
		util.ResError(c, err)
		return
	}
	accepted(c, item)
}

func (a *API) GetRun(c *gin.Context) {
	item, err := a.Service.GetRun(c.Request.Context(), c.Param("id"))
	if err != nil {
		util.ResError(c, err)
		return
	}
	util.ResSuccess(c, item)
}

func (a *API) StreamEvents(c *gin.Context) {
	after, err := eventCursor(c)
	if err != nil {
		util.ResError(c, projecterrors.BadRequest("agent_invalid_event_cursor", "event cursor must be an unsigned integer"))
		return
	}
	runID := c.Param("id")
	if _, err := a.Service.GetRun(c.Request.Context(), runID); err != nil {
		util.ResError(c, err)
		return
	}
	flusher, ok := c.Writer.(http.Flusher)
	if !ok {
		util.ResError(c, projecterrors.InternalServerError("agent_sse_unavailable", "streaming is unavailable"))
		return
	}
	live, unsubscribe := a.Service.Hub.Subscribe(runID)
	defer unsubscribe()
	c.Header("Content-Type", "text/event-stream")
	c.Header("Cache-Control", "no-cache")
	c.Header("Connection", "keep-alive")
	c.Header("X-Accel-Buffering", "no")
	last := after
	replay, err := a.Service.ListEvents(c.Request.Context(), runID, after)
	if err != nil {
		return
	}
	for _, event := range replay {
		if event.ID <= last {
			continue
		}
		writeSSE(c.Writer, event)
		last = event.ID
		flusher.Flush()
		if terminalEvent(event.Type) {
			return
		}
	}
	heartbeat := time.NewTicker(15 * time.Second)
	defer heartbeat.Stop()
	for {
		select {
		case <-c.Request.Context().Done():
			return
		case <-heartbeat.C:
			fmt.Fprintf(c.Writer, "id: %d\nevent: heartbeat\ndata: {\"id\":%d,\"run_id\":%q,\"time\":%q}\n\n", last, last, runID, time.Now().UTC().Format(time.RFC3339Nano))
			flusher.Flush()
		case event := <-live:
			if event == nil || event.ID <= last {
				continue
			}
			writeSSE(c.Writer, event)
			last = event.ID
			flusher.Flush()
			if terminalEvent(event.Type) {
				return
			}
		}
	}
}

func eventCursor(c *gin.Context) (uint64, error) {
	raw := c.Query("after")
	if raw == "" {
		raw = c.GetHeader("Last-Event-ID")
	}
	if raw == "" {
		return 0, nil
	}
	return strconv.ParseUint(raw, 10, 64)
}

func writeSSE(w io.Writer, event *schema.RunEvent) {
	payloadData := map[string]any{}
	if err := json.Unmarshal([]byte(event.Payload), &payloadData); err != nil {
		payloadData["run_id"] = event.RunID
		payloadData["time"] = event.CreatedAt.UTC().Format(time.RFC3339Nano)
	}
	payloadData["id"] = event.ID
	payload, _ := json.Marshal(payloadData)
	fmt.Fprintf(w, "id: %d\nevent: %s\ndata: %s\n\n", event.ID, event.Type, payload)
}

func terminalEvent(eventType string) bool {
	return eventType == "answer.completed" || eventType == "run.failed"
}

func accepted(c *gin.Context, data any) {
	util.ResJSON(c, http.StatusAccepted, util.ResponseResult{Success: true, Data: data})
}
