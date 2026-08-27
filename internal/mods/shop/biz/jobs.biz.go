package biz

import (
	"context"
	stderrors "errors"
	"fmt"
	"math/rand"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/LyricTian/gin-admin/v10/internal/config"
	"github.com/LyricTian/gin-admin/v10/internal/mods/shop/jd"
	"github.com/LyricTian/gin-admin/v10/internal/mods/shop/schema"
	projecterrors "github.com/LyricTian/gin-admin/v10/pkg/errors"
	"github.com/LyricTian/gin-admin/v10/pkg/logging"
	"github.com/LyricTian/gin-admin/v10/pkg/util"
	"go.uber.org/zap"
)

func (a *Service) TriggerJob(ctx context.Context, jobType, trigger string) (*schema.JobRun, error) {
	if !schema.ValidJobType(jobType) {
		return nil, projecterrors.BadRequest("invalid_shop_job_type", "unsupported shop job type %q", jobType)
	}
	if trigger != schema.JobTriggerManual && trigger != schema.JobTriggerSchedule {
		return nil, projecterrors.BadRequest("invalid_shop_job_trigger", "unsupported shop job trigger %q", trigger)
	}
	if trigger == schema.JobTriggerManual && !schema.ValidManualJobType(jobType) {
		return nil, projecterrors.BadRequest("invalid_manual_shop_job_type", "job type %q cannot be triggered manually", jobType)
	}
	now := time.Now()
	job := &schema.JobRun{
		ID: util.NewXID(), JobType: jobType, Trigger: trigger, Status: schema.JobStatusQueued,
		CreatedAt: now, UpdatedAt: now,
	}
	a.runningMu.Lock()
	if a.closing {
		a.runningMu.Unlock()
		return nil, projecterrors.Conflict("shop_scheduler_stopping", "shop scheduler is stopping")
	}
	a.jobWG.Add(1)
	a.runningMu.Unlock()
	if err := a.Store.CreateJob(ctx, job); err != nil {
		a.jobWG.Done()
		return nil, err
	}
	runtimeCtx := a.runtimeContext()
	go func() {
		defer a.jobWG.Done()
		a.runJob(runtimeCtx, job.ID, jobType)
	}()
	return job, nil
}

func (a *Service) tryStart(jobType string) bool {
	a.runningMu.Lock()
	defer a.runningMu.Unlock()
	if a.running == nil {
		a.running = make(map[string]bool)
	}
	if a.running[jobType] {
		return false
	}
	a.running[jobType] = true
	return true
}

func (a *Service) finish(jobType string) {
	a.runningMu.Lock()
	delete(a.running, jobType)
	a.runningMu.Unlock()
}

func (a *Service) runJob(ctx context.Context, id, jobType string) {
	if !a.tryStart(jobType) {
		now := time.Now()
		_ = a.Store.UpdateJob(context.Background(), id, map[string]interface{}{
			"status": schema.JobStatusSkipped, "error_summary": "same job type is already running",
			"finished_at": now, "updated_at": now,
		})
		return
	}
	defer a.finish(jobType)
	now := time.Now()
	_ = a.Store.UpdateJob(context.Background(), id, map[string]interface{}{
		"status": schema.JobStatusRunning, "started_at": now, "updated_at": now,
	})

	job := &schema.JobRun{ID: id, JobType: jobType}
	var err error
	switch jobType {
	case schema.JobTypeDiscover:
		err = a.executeDiscover(ctx, job)
	case schema.JobTypePublicScan:
		err = a.executePublicScan(ctx, job)
	case schema.JobTypeCheckoutSample:
		err = a.executeCheckout(ctx, job)
	case schema.JobTypeCleanup:
		err = a.executeCleanup(ctx, job)
	case schema.JobTypeAlertDelivery:
		err = a.DeliverPendingAlerts(ctx, 20)
	}
	finished := time.Now()
	status := schema.JobStatusSucceeded
	errorSummary := ""
	if err != nil {
		status = schema.JobStatusFailed
		errorSummary = truncate(err.Error(), 2048)
		logging.Context(ctx).Error("shop job failed", zap.String("job_type", jobType), zap.Error(err))
	}
	_ = a.Store.UpdateJob(context.Background(), id, map[string]interface{}{
		"status": status, "scanned_count": job.ScannedCount, "success_count": job.SuccessCount,
		"failure_count": job.FailureCount, "cursor": job.Cursor, "error_summary": errorSummary,
		"finished_at": finished, "updated_at": finished,
	})
}

func (a *Service) executeDiscover(ctx context.Context, job *schema.JobRun) error {
	categories, err := a.Store.ListEnabledCategories(ctx)
	if err != nil {
		return err
	}
	var errs []string
	for _, category := range categories {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}
		job.ScannedCount++
		products, discoverErr := a.JDClient.DiscoverCategory(ctx, category)
		now := time.Now()
		if discoverErr != nil {
			job.FailureCount++
			summary := truncate(discoverErr.Error(), 1024)
			_ = a.Store.UpdateCategoryDiscovery(ctx, category.ID, schema.JobStatusFailed, summary, now)
			errs = append(errs, category.Name+": "+summary)
			continue
		}
		if err := a.applyDiscovery(ctx, category, products, now); err != nil {
			job.FailureCount++
			summary := truncate(err.Error(), 1024)
			_ = a.Store.UpdateCategoryDiscovery(ctx, category.ID, schema.JobStatusFailed, summary, now)
			errs = append(errs, category.Name+": "+summary)
			continue
		}
		job.SuccessCount++
		_ = a.Store.UpdateCategoryDiscovery(ctx, category.ID, schema.JobStatusSucceeded, "", now)
	}
	if len(errs) > 0 {
		return fmt.Errorf("category discovery failures: %s", strings.Join(errs, "; "))
	}
	return nil
}

func (a *Service) applyDiscovery(ctx context.Context, category *schema.JDCategory, discovered []schema.DiscoveredProduct, now time.Time) error {
	sort.Slice(discovered, func(i, j int) bool { return discovered[i].SKU < discovered[j].SKU })
	return a.Trans.Exec(ctx, func(ctx context.Context) error {
		affected, err := a.Store.ProductIDsForCategory(ctx, category.ID)
		if err != nil {
			return err
		}
		if err := a.Store.IncrementCategoryMisses(ctx, category.ID); err != nil {
			return err
		}
		activeCount, err := a.Store.CountActiveProducts(ctx)
		if err != nil {
			return err
		}
		for _, found := range discovered {
			if !found.SelfOperated || found.SKU == "" {
				continue
			}
			product, ok, err := a.Store.GetProductBySKU(ctx, found.SKU)
			if err != nil {
				return err
			}
			if !ok {
				discoveryStatus := schema.DiscoveryStatusActive
				maxProducts := config.C.Shop.JD.MaxProducts
				if maxProducts <= 0 {
					maxProducts = 1000
				}
				if activeCount >= int64(maxProducts) {
					discoveryStatus = schema.DiscoveryStatusCapped
				} else {
					activeCount++
				}
				next := now
				product = &schema.JDProduct{
					ID: util.NewXID(), SKU: found.SKU, Name: found.Name, CanonicalURL: found.CanonicalURL,
					ImageURL: found.ImageURL, SelfOperated: true, MonitorStatus: schema.MonitorStatusActive,
					DiscoveryStatus: discoveryStatus, AlertState: schema.AlertStateArmed,
					NextCheckoutAt: &next, FirstSeenAt: now, LastSeenAt: now, CreatedAt: now, UpdatedAt: now,
				}
				if err := a.Store.CreateProduct(ctx, product); err != nil {
					return err
				}
			} else {
				discoveryStatus := product.DiscoveryStatus
				if product.DiscoveryStatus != schema.DiscoveryStatusActive {
					maxProducts := config.C.Shop.JD.MaxProducts
					if maxProducts <= 0 {
						maxProducts = 1000
					}
					if activeCount < int64(maxProducts) {
						discoveryStatus = schema.DiscoveryStatusActive
						if product.MonitorStatus == schema.MonitorStatusActive {
							activeCount++
						}
					} else {
						discoveryStatus = schema.DiscoveryStatusCapped
					}
				}
				fields := map[string]interface{}{
					"name": found.Name, "canonical_url": found.CanonicalURL, "image_url": found.ImageURL,
					"self_operated": true, "discovery_status": discoveryStatus, "last_seen_at": now, "updated_at": now,
				}
				if err := a.Store.UpdateProductColumns(ctx, product.ID, fields); err != nil {
					return err
				}
			}
			relation := &schema.JDCategoryProduct{
				ID: util.NewXID(), CategoryID: category.ID, ProductID: product.ID, MissCount: 0,
				FirstSeenAt: now, LastSeenAt: now, CreatedAt: now, UpdatedAt: now,
			}
			if err := a.Store.UpsertCategoryProduct(ctx, relation); err != nil {
				return err
			}
			affected = append(affected, product.ID)
		}
		seen := make(map[string]struct{})
		for _, productID := range affected {
			if _, ok := seen[productID]; ok {
				continue
			}
			seen[productID] = struct{}{}
			fresh, err := a.Store.HasFreshCategoryRelation(ctx, productID)
			if err != nil {
				return err
			}
			if !fresh {
				if err := a.Store.UpdateProductColumns(ctx, productID, map[string]interface{}{
					"discovery_status": schema.DiscoveryStatusStale, "updated_at": now,
				}); err != nil {
					return err
				}
			}
		}
		return nil
	})
}

func (a *Service) executePublicScan(ctx context.Context, job *schema.JobRun) error {
	limit := config.C.Shop.JD.MaxProducts
	if limit <= 0 {
		limit = 1000
	}
	products, err := a.Store.ListActiveProducts(ctx, limit)
	if err != nil {
		return err
	}
	workers := config.C.Shop.JD.PublicWorkers
	if workers <= 0 {
		workers = 4
	}
	if workers > len(products) && len(products) > 0 {
		workers = len(products)
	}
	jobs := make(chan *schema.JDProduct)
	var scanned, succeeded, failed int64
	var firstErr atomic.Value
	var wg sync.WaitGroup
	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for product := range jobs {
				atomic.AddInt64(&scanned, 1)
				observation, fetchErr := a.JDClient.FetchPublicPrice(ctx, product)
				if fetchErr == nil {
					fetchErr = a.recordPublicPrice(ctx, product, observation)
				}
				if fetchErr != nil {
					atomic.AddInt64(&failed, 1)
					if firstErr.Load() == nil {
						firstErr.Store(fetchErr.Error())
					}
				} else {
					atomic.AddInt64(&succeeded, 1)
				}
				if err := waitRandom(ctx, config.C.Shop.JD.PublicDelayMin, config.C.Shop.JD.PublicDelayMax); err != nil {
					return
				}
			}
		}()
	}
sendLoop:
	for _, product := range products {
		select {
		case <-ctx.Done():
			break sendLoop
		case jobs <- product:
		}
	}
	close(jobs)
	wg.Wait()
	job.ScannedCount = int(scanned)
	job.SuccessCount = int(succeeded)
	job.FailureCount = int(failed)
	if ctx.Err() != nil {
		return ctx.Err()
	}
	if failed > 0 && succeeded == 0 {
		if value := firstErr.Load(); value != nil {
			return fmt.Errorf("all public price checks failed: %s", value.(string))
		}
	}
	return nil
}

func (a *Service) recordPublicPrice(ctx context.Context, product *schema.JDProduct, observation *schema.PriceObservation) error {
	if observation == nil || observation.PriceFen <= 0 || observation.Currency != "CNY" || !observation.SelfOperated {
		return jd.ErrPriceUnavailable
	}
	now := time.Now()
	stats, err := a.Store.PriceStatsBefore(ctx, product.ID, schema.SampleTypePublic, now, now.AddDate(0, 0, -30))
	if err != nil {
		return err
	}
	sample := &schema.PriceSample{
		ID: util.NewXID(), ProductID: product.ID, SampleType: schema.SampleTypePublic,
		PriceFen: observation.PriceFen, Currency: "CNY", Source: "jd-mobile-page", Valid: true,
		CollectedAt: now, CreatedAt: now,
	}
	if err := a.Store.CreatePriceSample(ctx, sample); err != nil {
		return err
	}
	setting, err := a.GetSetting(ctx)
	if err != nil {
		return err
	}
	fields := map[string]interface{}{"updated_at": now}
	if strings.TrimSpace(observation.Name) != "" {
		fields["name"] = strings.TrimSpace(observation.Name)
	}
	if stats.Count > 0 && dropAtLeastPercent(stats.AverageFen, observation.PriceFen, setting.CandidateDropPercent) {
		fields["checkout_pending"] = true
		fields["next_checkout_at"] = now
	}
	return a.Store.UpdateProductColumns(ctx, product.ID, fields)
}

func (a *Service) executeCheckout(ctx context.Context, job *schema.JobRun) error {
	batch := config.C.Shop.JD.CheckoutBatch
	if batch <= 0 {
		batch = 20
	}
	products, err := a.Store.ListCheckoutDueProducts(ctx, time.Now(), batch)
	if err != nil {
		return err
	}
	if len(products) == 0 {
		return nil
	}
	status, err := a.JDClient.SessionStatus(ctx)
	if err != nil {
		return err
	}
	if status.CaptchaBlocked {
		a.notifyOperational(ctx, "jd-captcha", "京东价格监控需要人工验证", "检测到京东安全验证。结算查价已暂停；系统不会绕过验证码，请在专用 Chrome Profile 中人工完成验证。")
		return jd.ErrCaptchaBlocked
	}
	if !status.Authenticated {
		a.notifyOperational(ctx, "jd-login", "京东价格监控登录已失效", "结算查价已暂停。请停止服务，运行 jd-login 人工登录后再启动服务。")
		return jd.ErrLoginRequired
	}
	var errs []string
	for _, product := range products {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}
		job.ScannedCount++
		observation, fetchErr := a.JDClient.FetchCheckoutPreview(ctx, product)
		if fetchErr == nil {
			fetchErr = a.recordCheckoutPrice(ctx, product, observation)
		}
		if fetchErr != nil {
			job.FailureCount++
			errs = append(errs, product.SKU+": "+truncate(fetchErr.Error(), 200))
			if stderrors.Is(fetchErr, jd.ErrCaptchaBlocked) || stderrors.Is(fetchErr, jd.ErrLoginRequired) {
				return fetchErr
			}
			if stderrors.Is(fetchErr, jd.ErrNotSelfOperated) {
				_ = a.Store.UpdateProductColumns(ctx, product.ID, map[string]interface{}{
					"self_operated": false, "discovery_status": schema.DiscoveryStatusStale, "updated_at": time.Now(),
				})
			}
		} else {
			job.SuccessCount++
		}
		if err := waitRandom(ctx, config.C.Shop.JD.CheckoutDelayMin, config.C.Shop.JD.CheckoutDelayMax); err != nil {
			return err
		}
	}
	if err := a.DeliverPendingAlerts(ctx, 20); err != nil {
		errs = append(errs, "deliver alerts: "+err.Error())
	}
	if len(errs) > 0 && job.SuccessCount == 0 {
		return fmt.Errorf("checkout failures: %s", strings.Join(errs, "; "))
	}
	return nil
}

func (a *Service) recordCheckoutPrice(ctx context.Context, product *schema.JDProduct, observation *schema.PriceObservation) error {
	if observation == nil || observation.PriceFen <= 0 || observation.Currency != "CNY" || !observation.SelfOperated {
		return jd.ErrPriceUnavailable
	}
	now := time.Now()
	stats, err := a.Store.PriceStatsBefore(ctx, product.ID, schema.SampleTypeCheckout, now, now.AddDate(0, 0, -30))
	if err != nil {
		return err
	}
	setting, err := a.GetSetting(ctx)
	if err != nil {
		return err
	}
	sample := &schema.PriceSample{
		ID: util.NewXID(), ProductID: product.ID, SampleType: schema.SampleTypeCheckout,
		PriceFen: observation.PriceFen, Currency: "CNY", Source: "jd-checkout-preview", Valid: true,
		CollectedAt: now, CreatedAt: now,
	}
	return a.Trans.Exec(ctx, func(ctx context.Context) error {
		if err := a.Store.CreatePriceSample(ctx, sample); err != nil {
			return err
		}
		nextInterval := 24 * time.Hour
		if stats.Count+1 < 3 {
			nextInterval = 12 * time.Hour
		}
		fields := map[string]interface{}{
			"checkout_pending": false, "next_checkout_at": now.Add(nextInterval), "updated_at": now,
		}
		nextState, triggerAlert, dropBPS := EvaluateAlertState(product.AlertState, product.FirstSeenAt, now, stats, observation.PriceFen, setting)
		if nextState != product.AlertState {
			fields["alert_state"] = nextState
		}
		if triggerAlert {
			alert := &schema.PriceAlert{
				ID: util.NewXID(), ProductID: product.ID, TriggerSampleID: sample.ID,
				BaselinePriceFen: stats.AverageFen, CurrentPriceFen: observation.PriceFen,
				DropBasisPoints: dropBPS, SendStatus: schema.AlertSendPending, CreatedAt: now, UpdatedAt: now,
			}
			if err := a.Store.CreateAlert(ctx, alert); err != nil {
				return err
			}
		}
		return a.Store.UpdateProductColumns(ctx, product.ID, fields)
	})
}

func EvaluateAlertState(currentState string, firstSeenAt, now time.Time, stats *schema.PriceStats, currentPriceFen int64, setting *schema.ShopSetting) (string, bool, int) {
	if stats == nil || setting == nil || stats.AverageFen <= 0 {
		return currentState, false, 0
	}
	dropBPS := DropBasisPoints(stats.AverageFen, currentPriceFen)
	if currentState == schema.AlertStateAlerting {
		if !dropExceedsPercent(stats.AverageFen, currentPriceFen, setting.RecoveryDropPercent) {
			return schema.AlertStateArmed, false, dropBPS
		}
		return currentState, false, dropBPS
	}
	warm := stats.Count >= 3 && now.Sub(firstSeenAt) >= 24*time.Hour
	if currentState == schema.AlertStateArmed && warm && dropExceedsPercent(stats.AverageFen, currentPriceFen, setting.AlertDropPercent) {
		return schema.AlertStateAlerting, true, dropBPS
	}
	return currentState, false, dropBPS
}

func (a *Service) DeliverPendingAlerts(ctx context.Context, limit int) error {
	if !a.Notifier.Available() {
		return nil
	}
	alerts, err := a.Store.ListPendingAlerts(ctx, limit)
	if err != nil {
		return err
	}
	var errs []string
	for _, alert := range alerts {
		product, ok, getErr := a.Store.GetProduct(ctx, alert.ProductID)
		if getErr != nil || !ok {
			if getErr == nil {
				getErr = fmt.Errorf("product not found")
			}
			errs = append(errs, alert.ID+": "+getErr.Error())
			continue
		}
		categories, _ := a.Store.ListProductCategories(ctx, product.ID)
		names := make([]string, 0, len(categories))
		for _, category := range categories {
			names = append(names, category.Name)
		}
		title := fmt.Sprintf("京东自营降价 %.2f%%：%s", float64(alert.DropBasisPoints)/100, product.Name)
		description := fmt.Sprintf(
			"- SKU：%s\n- 分类：%s\n- 30 日结算均价：¥%s\n- 当前结算价：¥%s\n- 降幅：%.2f%%\n- 商品：%s\n- 采样时间：%s\n\n价格口径：数量 1、账号默认地址、自动应用已有优惠；仅进入结算预览，未提交订单。",
			product.SKU, strings.Join(names, "、"), schema.FormatPriceYuan(alert.BaselinePriceFen),
			schema.FormatPriceYuan(alert.CurrentPriceFen), float64(alert.DropBasisPoints)/100,
			product.CanonicalURL, alert.CreatedAt.Format(time.RFC3339),
		)
		response, sendErr := a.Notifier.Send(ctx, title, description)
		attempts := alert.SendAttempts + 1
		fields := map[string]interface{}{"send_attempts": attempts, "updated_at": time.Now()}
		if sendErr != nil {
			fields["send_status"] = schema.AlertSendFailed
			fields["last_error"] = truncate(sendErr.Error(), 1024)
			errs = append(errs, alert.ID+": "+sendErr.Error())
		} else {
			now := time.Now()
			fields["send_status"] = schema.AlertSendSent
			fields["provider_response"] = truncate(response, 2048)
			fields["last_error"] = ""
			fields["sent_at"] = now
		}
		if updateErr := a.Store.UpdateAlertDelivery(ctx, alert.ID, fields); updateErr != nil {
			errs = append(errs, alert.ID+": "+updateErr.Error())
		}
	}
	if len(errs) > 0 {
		return fmt.Errorf("alert delivery failures: %s", strings.Join(errs, "; "))
	}
	return nil
}

func (a *Service) executeCleanup(ctx context.Context, job *schema.JobRun) error {
	now := time.Now()
	prices, err := a.Store.DeletePriceSamplesBefore(ctx, now.AddDate(0, 0, -35))
	if err != nil {
		return err
	}
	alerts, err := a.Store.DeleteAlertsBefore(ctx, now.AddDate(0, 0, -180))
	if err != nil {
		return err
	}
	jobs, err := a.Store.DeleteJobsBefore(ctx, now.AddDate(0, 0, -180))
	if err != nil {
		return err
	}
	job.ScannedCount = int(prices + alerts + jobs)
	job.SuccessCount = job.ScannedCount
	return nil
}

func waitRandom(ctx context.Context, minSeconds, maxSeconds int) error {
	if minSeconds < 0 {
		minSeconds = 0
	}
	if maxSeconds < minSeconds {
		maxSeconds = minSeconds
	}
	delay := minSeconds
	if maxSeconds > minSeconds {
		delay += rand.Intn(maxSeconds - minSeconds + 1)
	}
	if delay == 0 {
		return nil
	}
	timer := time.NewTimer(time.Duration(delay) * time.Second)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}
