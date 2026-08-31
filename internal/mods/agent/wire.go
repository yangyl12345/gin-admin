package agent

import (
	"github.com/LyricTian/gin-admin/v10/internal/mods/agent/api"
	"github.com/LyricTian/gin-admin/v10/internal/mods/agent/biz"
	"github.com/LyricTian/gin-admin/v10/internal/mods/agent/dal"
	"github.com/LyricTian/gin-admin/v10/internal/mods/agent/llm"
	"github.com/LyricTian/gin-admin/v10/internal/mods/agent/retrieval"
	"github.com/google/wire"
)

var Set = wire.NewSet(
	wire.Struct(new(Agent), "*"),
	wire.Struct(new(dal.Store), "*"),
	llm.NewOpenAI,
	wire.Bind(new(llm.Gateway), new(*llm.OpenAI)),
	retrieval.NewCache,
	biz.NewEventHub,
	biz.NewService,
	wire.Struct(new(api.API), "*"),
)
