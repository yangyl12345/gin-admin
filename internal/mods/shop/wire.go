package shop

import (
	"github.com/LyricTian/gin-admin/v10/internal/mods/shop/api"
	"github.com/LyricTian/gin-admin/v10/internal/mods/shop/biz"
	"github.com/google/wire"
)

// Set contains the providers owned by the shop module.
var Set = wire.NewSet(
	wire.Struct(new(Shop), "*"),
	biz.NewStatus,
	wire.Struct(new(api.Status), "*"),
)
