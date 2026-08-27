package jd

import (
	"context"

	"github.com/LyricTian/gin-admin/v10/internal/mods/shop/schema"
)

type Client interface {
	SessionStatus(context.Context) (*schema.SessionStatus, error)
	DiscoverCategory(context.Context, *schema.JDCategory) ([]schema.DiscoveredProduct, error)
	FetchPublicPrice(context.Context, *schema.JDProduct) (*schema.PriceObservation, error)
	FetchCheckoutPreview(context.Context, *schema.JDProduct) (*schema.PriceObservation, error)
	Close() error
}
