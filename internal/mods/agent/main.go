package agent

import (
	"context"
	"net/http"
	"path/filepath"

	"github.com/LyricTian/gin-admin/v10/internal/config"
	"github.com/LyricTian/gin-admin/v10/internal/mods/agent/api"
	"github.com/LyricTian/gin-admin/v10/internal/mods/agent/biz"
	"github.com/LyricTian/gin-admin/v10/internal/mods/agent/schema"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

type Agent struct {
	DB      *gorm.DB
	API     *api.API
	Service *biz.Service
}

func (a *Agent) AutoMigrate(_ context.Context) error {
	return a.DB.AutoMigrate(
		new(schema.KnowledgeBase), new(schema.Document), new(schema.Chunk),
		new(schema.IngestionJob), new(schema.Conversation), new(schema.Message),
		new(schema.Run), new(schema.RunStep), new(schema.RunEvent), new(schema.Citation),
	)
}

func (a *Agent) Init(ctx context.Context) error {
	if !config.C.Agent.Enable {
		return nil
	}
	if err := a.Service.ValidateConfig(); err != nil {
		return err
	}
	if config.C.Storage.DB.AutoMigrate {
		if err := a.AutoMigrate(ctx); err != nil {
			return err
		}
	}
	return a.Service.Start(ctx)
}

func (a *Agent) RegisterV1Routers(_ context.Context, v1 *gin.RouterGroup) error {
	group := v1.Group("agent")
	group.GET("status", a.API.Status)
	protected := group.Group("")
	protected.Use(a.API.RequireEnabled(), a.API.Auth())
	{
		protected.GET("knowledge-bases", a.API.QueryKnowledgeBases)
		protected.POST("knowledge-bases", a.API.CreateKnowledgeBase)
		protected.GET("knowledge-bases/:id", a.API.GetKnowledgeBase)
		protected.PUT("knowledge-bases/:id", a.API.UpdateKnowledgeBase)
		protected.DELETE("knowledge-bases/:id", a.API.ArchiveKnowledgeBase)
		protected.GET("knowledge-bases/:id/documents", a.API.QueryDocuments)
		protected.POST("knowledge-bases/:id/documents", a.API.UploadDocument)
		protected.GET("documents/:id", a.API.GetDocument)
		protected.DELETE("documents/:id", a.API.ArchiveDocument)
		protected.POST("documents/:id/reindex", a.API.ReindexDocument)
		protected.GET("ingestion-jobs/:id", a.API.GetIngestionJob)
		protected.GET("conversations", a.API.QueryConversations)
		protected.POST("conversations", a.API.CreateConversation)
		protected.GET("conversations/:id", a.API.GetConversation)
		protected.DELETE("conversations/:id", a.API.ArchiveConversation)
		protected.GET("conversations/:id/messages", a.API.QueryMessages)
		protected.POST("conversations/:id/runs", a.API.CreateRun)
		protected.GET("runs/:id", a.API.GetRun)
		protected.GET("runs/:id/events", a.API.StreamEvents)
	}
	return nil
}

func (a *Agent) RegisterStaticRouters(e *gin.Engine) {
	root := filepath.Join(config.C.Middleware.Static.Dir, "agent")
	e.GET("/agent", func(c *gin.Context) { c.Redirect(http.StatusTemporaryRedirect, "/agent/") })
	e.GET("/agent/", func(c *gin.Context) { c.File(filepath.Join(root, "index.html")) })
	e.StaticFS("/agent/assets", gin.Dir(filepath.Join(root, "assets"), false))
}

func (a *Agent) Release(context.Context) error {
	a.Service.Stop()
	return nil
}
