package mods

import (
	"context"
	"errors"

	"github.com/LyricTian/gin-admin/v10/internal/mods/agent"
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
	agent.Set,
	shop.Set,
)

type Mods struct {
	Agent *agent.Agent
	Shop  *shop.Shop
}

func (a *Mods) Init(ctx context.Context) error {
	if err := a.Agent.Init(ctx); err != nil {
		return err
	}
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

	if err := a.Shop.RegisterV1Routers(ctx, v1); err != nil {
		return err
	}
	if err := a.Agent.RegisterV1Routers(ctx, v1); err != nil {
		return err
	}
	a.Agent.RegisterStaticRouters(e)
	return nil
}

func (a *Mods) Release(ctx context.Context) error {
	return errors.Join(a.Shop.Release(ctx), a.Agent.Release(ctx))
}
