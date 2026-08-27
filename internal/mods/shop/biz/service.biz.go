package biz

import (
	"context"
	"fmt"
	"math/bits"
	"strings"
	"sync"
	"time"

	"github.com/LyricTian/gin-admin/v10/internal/config"
	"github.com/LyricTian/gin-admin/v10/internal/mods/shop/dal"
	"github.com/LyricTian/gin-admin/v10/internal/mods/shop/jd"
	"github.com/LyricTian/gin-admin/v10/internal/mods/shop/notify"
	"github.com/LyricTian/gin-admin/v10/internal/mods/shop/schema"
	projecterrors "github.com/LyricTian/gin-admin/v10/pkg/errors"
	"github.com/LyricTian/gin-admin/v10/pkg/util"
)

type Service struct {
	Store    *dal.Store
	JDClient jd.Client
	Notifier notify.Notifier
	Trans    *util.Trans

	runtimeMu       sync.RWMutex
	runtimeCtx      context.Context
	runningMu       sync.Mutex
	running         map[string]bool
	closing         bool
	jobWG           sync.WaitGroup
	operationMu     sync.Mutex
	lastOperational map[string]time.Time
}

func NewService(store *dal.Store, client jd.Client, notifier notify.Notifier, trans *util.Trans) *Service {
	return &Service{
		Store: store, JDClient: client, Notifier: notifier, Trans: trans,
		running: make(map[string]bool), lastOperational: make(map[string]time.Time),
	}
}

func (a *Service) SetRuntimeContext(ctx context.Context) {
	a.runtimeMu.Lock()
	a.runtimeCtx = ctx
	a.runtimeMu.Unlock()
	a.runningMu.Lock()
	a.closing = false
	a.runningMu.Unlock()
}

func (a *Service) StopAcceptingJobs() {
	a.runningMu.Lock()
	a.closing = true
	a.runningMu.Unlock()
}

func (a *Service) runtimeContext() context.Context {
	a.runtimeMu.RLock()
	ctx := a.runtimeCtx
	a.runtimeMu.RUnlock()
	if ctx == nil {
		return context.Background()
	}
	return ctx
}

func (a *Service) Wait(ctx context.Context) error {
	done := make(chan struct{})
	go func() {
		a.jobWG.Wait()
		close(done)
	}()
	select {
	case <-done:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (a *Service) EnsureDefaults(ctx context.Context) error {
	_, ok, err := a.Store.GetSetting(ctx)
	if err != nil {
		return err
	}
	if ok {
		return nil
	}
	now := time.Now()
	return a.Store.CreateSetting(ctx, &schema.ShopSetting{
		ID: schema.DefaultSettingID, CandidateDropPercent: 15, AlertDropPercent: 20,
		RecoveryDropPercent: 15, CreatedAt: now, UpdatedAt: now,
	})
}

func (a *Service) GetSetting(ctx context.Context) (*schema.ShopSetting, error) {
	item, ok, err := a.Store.GetSetting(ctx)
	if err != nil {
		return nil, err
	}
	if !ok {
		if err := a.EnsureDefaults(ctx); err != nil {
			return nil, err
		}
		item, _, err = a.Store.GetSetting(ctx)
	}
	return item, err
}

func (a *Service) UpdateSetting(ctx context.Context, form *schema.ShopSettingForm) error {
	if err := form.Validate(); err != nil {
		return err
	}
	item, err := a.GetSetting(ctx)
	if err != nil {
		return err
	}
	item.CandidateDropPercent = form.CandidateDropPercent
	item.AlertDropPercent = form.AlertDropPercent
	item.RecoveryDropPercent = form.RecoveryDropPercent
	item.UpdatedAt = time.Now()
	return a.Store.UpdateSetting(ctx, item)
}

func (a *Service) QueryCategories(ctx context.Context, params schema.JDCategoryQueryParam) (*schema.JDCategoryQueryResult, error) {
	params.Pagination = true
	return a.Store.QueryCategories(ctx, params)
}

func (a *Service) GetCategory(ctx context.Context, id string) (*schema.JDCategory, error) {
	item, ok, err := a.Store.GetCategory(ctx, id)
	if err != nil {
		return nil, err
	}
	if !ok {
		return nil, projecterrors.NotFound("shop_category_not_found", "JD category not found")
	}
	return item, nil
}

func (a *Service) CreateCategory(ctx context.Context, form *schema.JDCategoryForm) (*schema.JDCategory, error) {
	if err := form.Validate(); err != nil {
		return nil, err
	}
	exists, err := a.Store.CategoryURLExists(ctx, form.SourceURL, "")
	if err != nil {
		return nil, err
	}
	if exists {
		return nil, projecterrors.Conflict("shop_category_url_exists", "JD category source_url already exists")
	}
	now := time.Now()
	item := &schema.JDCategory{
		ID: util.NewXID(), Name: form.Name, SourceURL: form.SourceURL, Status: form.Status,
		MaxPages: form.MaxPages, CreatedAt: now, UpdatedAt: now,
	}
	if err := a.Store.CreateCategory(ctx, item); err != nil {
		return nil, err
	}
	return item, nil
}

func (a *Service) UpdateCategory(ctx context.Context, id string, form *schema.JDCategoryForm) error {
	if err := form.Validate(); err != nil {
		return err
	}
	item, err := a.GetCategory(ctx, id)
	if err != nil {
		return err
	}
	exists, err := a.Store.CategoryURLExists(ctx, form.SourceURL, id)
	if err != nil {
		return err
	}
	if exists {
		return projecterrors.Conflict("shop_category_url_exists", "JD category source_url already exists")
	}
	oldStatus := item.Status
	item.Name, item.SourceURL, item.Status, item.MaxPages = form.Name, form.SourceURL, form.Status, form.MaxPages
	item.UpdatedAt = time.Now()
	return a.Trans.Exec(ctx, func(ctx context.Context) error {
		ids, err := a.Store.ProductIDsForCategory(ctx, id)
		if err != nil {
			return err
		}
		if err := a.Store.UpdateCategory(ctx, item); err != nil {
			return err
		}
		if oldStatus == schema.StatusEnabled && form.Status == schema.StatusDisabled {
			for _, productID := range ids {
				fresh, err := a.Store.HasFreshCategoryRelation(ctx, productID)
				if err != nil {
					return err
				}
				if !fresh {
					if err := a.Store.UpdateProductColumns(ctx, productID, map[string]interface{}{
						"discovery_status": schema.DiscoveryStatusStale, "updated_at": time.Now(),
					}); err != nil {
						return err
					}
				}
			}
		}
		return nil
	})
}

func (a *Service) DeleteCategory(ctx context.Context, id string) error {
	if _, err := a.GetCategory(ctx, id); err != nil {
		return err
	}
	return a.Trans.Exec(ctx, func(ctx context.Context) error {
		ids, err := a.Store.ProductIDsForCategory(ctx, id)
		if err != nil {
			return err
		}
		if err := a.Store.DeleteCategoryProducts(ctx, id); err != nil {
			return err
		}
		if err := a.Store.DeleteCategory(ctx, id); err != nil {
			return err
		}
		for _, productID := range ids {
			fresh, err := a.Store.HasFreshCategoryRelation(ctx, productID)
			if err != nil {
				return err
			}
			if !fresh {
				if err := a.Store.UpdateProductColumns(ctx, productID, map[string]interface{}{
					"discovery_status": schema.DiscoveryStatusStale, "updated_at": time.Now(),
				}); err != nil {
					return err
				}
			}
		}
		return nil
	})
}

func (a *Service) QueryProducts(ctx context.Context, params schema.JDProductQueryParam) (*schema.JDProductQueryResult, error) {
	params.Pagination = true
	return a.Store.QueryProducts(ctx, params)
}

func (a *Service) GetProduct(ctx context.Context, id string) (*schema.ProductDetail, error) {
	item, ok, err := a.Store.GetProduct(ctx, id)
	if err != nil {
		return nil, err
	}
	if !ok {
		return nil, projecterrors.NotFound("shop_product_not_found", "JD product not found")
	}
	categories, err := a.Store.ListProductCategories(ctx, id)
	if err != nil {
		return nil, err
	}
	publicPrice, publicOK, err := a.Store.LatestPriceSample(ctx, id, schema.SampleTypePublic)
	if err != nil {
		return nil, err
	}
	if publicOK {
		publicPrice.FillDisplay()
	} else {
		publicPrice = nil
	}
	checkoutPrice, checkoutOK, err := a.Store.LatestPriceSample(ctx, id, schema.SampleTypeCheckout)
	if err != nil {
		return nil, err
	}
	if checkoutOK {
		checkoutPrice.FillDisplay()
	} else {
		checkoutPrice = nil
	}
	now := time.Now()
	stats, err := a.Store.PriceStatsBefore(ctx, id, schema.SampleTypeCheckout, now.Add(time.Nanosecond), now.AddDate(0, 0, -30))
	if err != nil {
		return nil, err
	}
	return &schema.ProductDetail{
		JDProduct: item, Categories: categories, LatestPublicPrice: publicPrice,
		LatestCheckoutPrice: checkoutPrice, CheckoutAverageFen: stats.AverageFen,
		CheckoutAverageYuan: schema.FormatPriceYuan(stats.AverageFen),
	}, nil
}

func (a *Service) UpdateProduct(ctx context.Context, id string, form *schema.JDProductForm) error {
	item, ok, err := a.Store.GetProduct(ctx, id)
	if err != nil {
		return err
	}
	if !ok {
		return projecterrors.NotFound("shop_product_not_found", "JD product not found")
	}
	if item.MonitorStatus == schema.MonitorStatusPaused && form.MonitorStatus == schema.MonitorStatusActive && item.DiscoveryStatus == schema.DiscoveryStatusActive {
		count, err := a.Store.CountActiveProducts(ctx)
		if err != nil {
			return err
		}
		maxProducts := config.C.Shop.JD.MaxProducts
		if maxProducts <= 0 {
			maxProducts = 1000
		}
		if count >= int64(maxProducts) {
			return projecterrors.Conflict("shop_product_limit_reached", "active JD product limit has been reached")
		}
	}
	item.MonitorStatus = form.MonitorStatus
	item.UpdatedAt = time.Now()
	return a.Store.UpdateProduct(ctx, item, "monitor_status", "updated_at")
}

func (a *Service) QueryPriceSamples(ctx context.Context, productID string, params schema.PriceSampleQueryParam) (*schema.PriceSampleQueryResult, error) {
	if _, ok, err := a.Store.GetProduct(ctx, productID); err != nil {
		return nil, err
	} else if !ok {
		return nil, projecterrors.NotFound("shop_product_not_found", "JD product not found")
	}
	params.ProductID = productID
	params.Pagination = true
	result, err := a.Store.QueryPriceSamples(ctx, params)
	if err != nil {
		return nil, err
	}
	for _, item := range result.Data {
		item.FillDisplay()
	}
	return result, nil
}

func (a *Service) QueryAlerts(ctx context.Context, params schema.PriceAlertQueryParam) (*schema.PriceAlertQueryResult, error) {
	params.Pagination = true
	result, err := a.Store.QueryAlerts(ctx, params)
	if err != nil {
		return nil, err
	}
	for _, item := range result.Data {
		item.FillDisplay()
		if product, ok, err := a.Store.GetProduct(ctx, item.ProductID); err == nil && ok {
			item.Product = product
		}
	}
	return result, nil
}

func (a *Service) QueryJobs(ctx context.Context, params schema.JobRunQueryParam) (*schema.JobRunQueryResult, error) {
	params.Pagination = true
	return a.Store.QueryJobs(ctx, params)
}

func (a *Service) SessionStatus(ctx context.Context) (*schema.SessionStatus, error) {
	status, err := a.JDClient.SessionStatus(ctx)
	if err != nil && status != nil {
		status.ErrorSummary = truncate(err.Error(), 1024)
		return status, nil
	}
	return status, err
}

func (a *Service) TestNotification(ctx context.Context) (*schema.NotificationTestResult, error) {
	if !a.Notifier.Available() {
		return nil, projecterrors.InternalServerError("serverchan_unavailable", "SERVERCHAN_SEND_KEY is not configured")
	}
	response, err := a.Notifier.Send(ctx, "京东价格监控测试", "Server酱通知通道已连接。此消息不包含任何京东账号或会话信息。")
	if err != nil {
		return nil, projecterrors.Wrap(err, "send ServerChan test notification")
	}
	return &schema.NotificationTestResult{Provider: "serverchan", Response: response}, nil
}

func DropBasisPoints(baselineFen, currentFen int64) int {
	if baselineFen <= 0 || currentFen >= baselineFen {
		return 0
	}
	if currentFen < 0 {
		currentFen = 0
	}
	hi, lo := bits.Mul64(uint64(baselineFen-currentFen), 10000)
	quotient, _ := bits.Div64(hi, lo, uint64(baselineFen))
	return int(quotient)
}

func dropAtLeastPercent(baselineFen, currentFen int64, percent int) bool {
	return compareDropPercent(baselineFen, currentFen, percent) >= 0
}

func dropExceedsPercent(baselineFen, currentFen int64, percent int) bool {
	return compareDropPercent(baselineFen, currentFen, percent) > 0
}

func compareDropPercent(baselineFen, currentFen int64, percent int) int {
	if baselineFen <= 0 || percent < 0 {
		return -1
	}
	if currentFen < 0 {
		currentFen = 0
	}
	dropFen := int64(0)
	if currentFen < baselineFen {
		dropFen = baselineFen - currentFen
	}
	leftHi, leftLo := bits.Mul64(uint64(dropFen), 100)
	rightHi, rightLo := bits.Mul64(uint64(baselineFen), uint64(percent))
	if leftHi < rightHi || (leftHi == rightHi && leftLo < rightLo) {
		return -1
	}
	if leftHi == rightHi && leftLo == rightLo {
		return 0
	}
	return 1
}

func (a *Service) notifyOperational(ctx context.Context, key, title, message string) {
	if !a.Notifier.Available() {
		return
	}
	a.operationMu.Lock()
	if a.lastOperational == nil {
		a.lastOperational = make(map[string]time.Time)
	}
	if last := a.lastOperational[key]; !last.IsZero() && time.Since(last) < 12*time.Hour {
		a.operationMu.Unlock()
		return
	}
	a.lastOperational[key] = time.Now()
	a.operationMu.Unlock()
	_, _ = a.Notifier.Send(ctx, title, message)
}

func truncate(value string, max int) string {
	value = strings.TrimSpace(value)
	if len(value) <= max {
		return value
	}
	return fmt.Sprintf("%s...", value[:max-3])
}
