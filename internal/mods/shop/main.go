package shop

import (
	"context"

	"github.com/LyricTian/gin-admin/v10/internal/config"
	"github.com/LyricTian/gin-admin/v10/internal/mods/shop/api"
	"github.com/LyricTian/gin-admin/v10/internal/mods/shop/schema"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

// Shop is the isolated entry point for the shop domain.
//
// Keep shop-owned APIs, business logic, persistence, and schemas under this
// package.
type Shop struct {
	DB            *gorm.DB
	StatusAPI     *api.Status
	ManagementAPI *api.Management
	Scheduler     *Scheduler
}

func (a *Shop) AutoMigrate(_ context.Context) error {
	return a.DB.AutoMigrate(
		new(schema.ShopSetting), new(schema.JDCategory), new(schema.JDProduct),
		new(schema.JDCategoryProduct), new(schema.PriceSample), new(schema.PriceAlert),
		new(schema.JobRun),
	)
}

func (a *Shop) Init(ctx context.Context) error {
	if config.C.Storage.DB.AutoMigrate {
		if err := a.AutoMigrate(ctx); err != nil {
			return err
		}
	}
	if err := a.ManagementAPI.Service.EnsureDefaults(ctx); err != nil {
		return err
	}
	if config.C.Shop.Enable {
		return a.Scheduler.Start(ctx)
	}
	return nil
}

func (a *Shop) RegisterV1Routers(_ context.Context, v1 *gin.RouterGroup) error {
	shopGroup := v1.Group("shop")
	{
		shopGroup.GET("status", a.StatusAPI.Get)
		shopGroup.GET("settings", a.ManagementAPI.GetSetting)
		shopGroup.PUT("settings", a.ManagementAPI.UpdateSetting)

		shopGroup.GET("categories", a.ManagementAPI.QueryCategories)
		shopGroup.GET("categories/:id", a.ManagementAPI.GetCategory)
		shopGroup.POST("categories", a.ManagementAPI.CreateCategory)
		shopGroup.PUT("categories/:id", a.ManagementAPI.UpdateCategory)
		shopGroup.DELETE("categories/:id", a.ManagementAPI.DeleteCategory)

		shopGroup.GET("products", a.ManagementAPI.QueryProducts)
		shopGroup.GET("products/:id", a.ManagementAPI.GetProduct)
		shopGroup.PUT("products/:id", a.ManagementAPI.UpdateProduct)
		shopGroup.GET("products/:id/prices", a.ManagementAPI.QueryPrices)
		shopGroup.GET("alerts", a.ManagementAPI.QueryAlerts)

		shopGroup.GET("jobs", a.ManagementAPI.QueryJobs)
		shopGroup.POST("jobs/:type/run", a.ManagementAPI.RunJob)
		shopGroup.GET("session", a.ManagementAPI.SessionStatus)
		shopGroup.POST("notifications/test", a.ManagementAPI.TestNotification)
	}
	return nil
}

func (a *Shop) Release(ctx context.Context) error {
	return a.Scheduler.Stop(ctx)
}
