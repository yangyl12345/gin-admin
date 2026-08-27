package shop

import (
	"context"
	"fmt"
	"sync"

	"github.com/LyricTian/gin-admin/v10/internal/config"
	"github.com/LyricTian/gin-admin/v10/internal/mods/shop/biz"
	"github.com/LyricTian/gin-admin/v10/internal/mods/shop/schema"
	"github.com/LyricTian/gin-admin/v10/pkg/logging"
	"github.com/robfig/cron/v3"
	"go.uber.org/zap"
)

type Scheduler struct {
	Service *biz.Service

	mu     sync.Mutex
	cron   *cron.Cron
	cancel context.CancelFunc
}

func NewScheduler(service *biz.Service) *Scheduler { return &Scheduler{Service: service} }

func (a *Scheduler) Start(ctx context.Context) error {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.cron != nil {
		return nil
	}
	runtimeCtx, cancel := context.WithCancel(ctx)
	c := cron.New()
	entries := []struct {
		spec    string
		jobType string
	}{
		{config.C.Shop.Scheduler.DiscoverSpec, schema.JobTypeDiscover},
		{config.C.Shop.Scheduler.PublicScanSpec, schema.JobTypePublicScan},
		{config.C.Shop.Scheduler.CheckoutSpec, schema.JobTypeCheckoutSample},
		{config.C.Shop.Scheduler.CleanupSpec, schema.JobTypeCleanup},
		{config.C.Shop.Scheduler.AlertDeliverySpec, schema.JobTypeAlertDelivery},
	}
	for _, entry := range entries {
		entry := entry
		if entry.spec == "" {
			cancel()
			return fmt.Errorf("empty schedule for shop job %s", entry.jobType)
		}
		if _, err := c.AddFunc(entry.spec, func() {
			if _, err := a.Service.TriggerJob(runtimeCtx, entry.jobType, schema.JobTriggerSchedule); err != nil {
				logging.Context(runtimeCtx).Error("failed to enqueue shop job", zap.String("job_type", entry.jobType), zap.Error(err))
			}
		}); err != nil {
			cancel()
			return fmt.Errorf("register shop schedule %s: %w", entry.jobType, err)
		}
	}
	a.Service.SetRuntimeContext(runtimeCtx)
	a.cancel = cancel
	a.cron = c
	c.Start()
	return nil
}

func (a *Scheduler) Stop(ctx context.Context) error {
	a.mu.Lock()
	c := a.cron
	cancel := a.cancel
	a.cron = nil
	a.cancel = nil
	a.mu.Unlock()
	a.Service.StopAcceptingJobs()
	if cancel != nil {
		cancel()
	}
	if c != nil {
		stopped := c.Stop()
		select {
		case <-stopped.Done():
		case <-ctx.Done():
			return ctx.Err()
		}
	}
	if err := a.Service.Wait(ctx); err != nil {
		return err
	}
	return a.Service.JDClient.Close()
}
