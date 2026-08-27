package mods

import (
	"context"

	"github.com/LyricTian/gin-admin/v10/internal/mods/shop"
	"github.com/gin-gonic/gin"
	"github.com/google/wire"
)

const (
	apiPrefix = "/api/"
)

// Collection of wire providers
var Set = wire.NewSet(
	wire.Struct(new(Mods), "*"),
	shop.Set,
)

type Mods struct {
	Shop *shop.Shop
}

func (a *Mods) Init(ctx context.Context) error {
	return a.Shop.Init(ctx)
}

func (a *Mods) RouterPrefixes() []string {
	return []string{
		apiPrefix,
	}
}

func (a *Mods) RegisterRouters(ctx context.Context, e *gin.Engine) error {
	gAPI := e.Group(apiPrefix)
	v1 := gAPI.Group("v1")

	return a.Shop.RegisterV1Routers(ctx, v1)
}

func (a *Mods) Release(ctx context.Context) error {
	return a.Shop.Release(ctx)
}
