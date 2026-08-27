package shop

import (
	"github.com/LyricTian/gin-admin/v10/internal/mods/shop/api"
	"github.com/LyricTian/gin-admin/v10/internal/mods/shop/biz"
	"github.com/LyricTian/gin-admin/v10/internal/mods/shop/dal"
	"github.com/LyricTian/gin-admin/v10/internal/mods/shop/jd"
	"github.com/LyricTian/gin-admin/v10/internal/mods/shop/notify"
	"github.com/google/wire"
)

// Set contains the providers owned by the shop module.
var Set = wire.NewSet(
	wire.Struct(new(Shop), "*"),
	NewScheduler,
	biz.NewStatus,
	biz.NewService,
	wire.Struct(new(dal.Store), "*"),
	jd.NewChromeClient,
	notify.NewServerChan,
	wire.Struct(new(api.Status), "*"),
	wire.Struct(new(api.Management), "*"),
)
