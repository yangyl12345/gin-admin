package shop

import (
	"context"

	"github.com/LyricTian/gin-admin/v10/internal/mods/shop/api"
	"github.com/gin-gonic/gin"
)

// Shop is the isolated entry point for the shop domain.
//
// Keep shop-owned APIs, business logic, persistence, and schemas under this
// package instead of adding them to the RBAC module.
type Shop struct {
	StatusAPI *api.Status
}

func (a *Shop) Init(_ context.Context) error {
	return nil
}

func (a *Shop) RegisterV1Routers(_ context.Context, v1 *gin.RouterGroup) error {
	shopGroup := v1.Group("shop")
	shopGroup.GET("status", a.StatusAPI.Get)
	return nil
}

func (a *Shop) Release(_ context.Context) error {
	return nil
}
