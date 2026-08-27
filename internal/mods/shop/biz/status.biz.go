package biz

import (
	"context"

	"github.com/LyricTian/gin-admin/v10/internal/mods/shop/schema"
)

// Status exposes module-level information for the standalone shop service.
type Status struct{}

func NewStatus() *Status {
	return new(Status)
}

func (a *Status) Get(_ context.Context) (*schema.Status, error) {
	return &schema.Status{
		Name:   "shop",
		Status: "ready",
	}, nil
}
