package test

import (
	"context"
	"fmt"
	"net/http"
	"sync"
	"testing"
	"time"

	"github.com/LyricTian/gin-admin/v10/internal/config"
	"github.com/LyricTian/gin-admin/v10/internal/mods/shop/schema"
	"github.com/LyricTian/gin-admin/v10/pkg/util"
	"github.com/gavv/httpexpect/v2"
	"github.com/stretchr/testify/assert"
)

type fakeJDClient struct {
	mu            sync.Mutex
	publicPrice   int64
	checkoutPrice int64
}

func (a *fakeJDClient) SessionStatus(context.Context) (*schema.SessionStatus, error) {
	return &schema.SessionStatus{Authenticated: true, LastCheckedAt: time.Now()}, nil
}

func (a *fakeJDClient) DiscoverCategory(context.Context, *schema.JDCategory) ([]schema.DiscoveredProduct, error) {
	return []schema.DiscoveredProduct{{
		SKU: "100000000001", Name: "测试京东自营商品",
		CanonicalURL: "https://item.m.jd.com/product/100000000001.html", SelfOperated: true,
	}}, nil
}

func (a *fakeJDClient) FetchPublicPrice(context.Context, *schema.JDProduct) (*schema.PriceObservation, error) {
	a.mu.Lock()
	price := a.publicPrice
	a.mu.Unlock()
	return &schema.PriceObservation{SKU: "100000000001", Name: "测试京东自营商品", PriceFen: price, Currency: "CNY", SelfOperated: true}, nil
}

func (a *fakeJDClient) FetchCheckoutPreview(context.Context, *schema.JDProduct) (*schema.PriceObservation, error) {
	a.mu.Lock()
	price := a.checkoutPrice
	a.mu.Unlock()
	return &schema.PriceObservation{SKU: "100000000001", Name: "测试京东自营商品", PriceFen: price, Currency: "CNY", SelfOperated: true}, nil
}

func (a *fakeJDClient) Close() error { return nil }

func (a *fakeJDClient) setPublicPrice(price int64) {
	a.mu.Lock()
	a.publicPrice = price
	a.mu.Unlock()
}

func (a *fakeJDClient) setCheckoutPrice(price int64) {
	a.mu.Lock()
	a.checkoutPrice = price
	a.mu.Unlock()
}

type fakeNotifier struct {
	mu       sync.Mutex
	messages []string
}

func (*fakeNotifier) Available() bool { return true }

func (a *fakeNotifier) Send(_ context.Context, title, description string) (string, error) {
	a.mu.Lock()
	a.messages = append(a.messages, title+"\n"+description)
	a.mu.Unlock()
	return `{"code":0}`, nil
}

func (a *fakeNotifier) count() int {
	a.mu.Lock()
	defer a.mu.Unlock()
	return len(a.messages)
}

func TestShopStatus(t *testing.T) {
	e := tester(t)

	var status schema.Status
	e.GET(baseAPI+"/shop/status").WithHeader("Authorization", "Bearer ignored-after-auth-removal").
		Expect().Status(http.StatusOK).
		JSON().Decode(&util.ResponseResult{Data: &status})

	assert.Equal(t, "shop", status.Name)
	assert.Equal(t, "ready", status.Status)
}

func TestRemovedAdminRoutes(t *testing.T) {
	e := tester(t)
	e.POST(baseAPI + "/login").Expect().Status(http.StatusNotFound)
	e.GET(baseAPI + "/captcha/id").Expect().Status(http.StatusNotFound)
	e.GET(baseAPI + "/current/user").Expect().Status(http.StatusNotFound)
	e.GET(baseAPI + "/menus").Expect().Status(http.StatusNotFound)
	e.GET(baseAPI + "/roles").Expect().Status(http.StatusNotFound)
	e.GET(baseAPI + "/users").Expect().Status(http.StatusNotFound)
	e.GET(baseAPI + "/loggers").Expect().Status(http.StatusNotFound)
}

func TestShopSettings(t *testing.T) {
	e := tester(t)

	var setting schema.ShopSetting
	e.GET(baseAPI + "/shop/settings").Expect().Status(http.StatusOK).
		JSON().Decode(&util.ResponseResult{Data: &setting})
	assert.Equal(t, 15, setting.CandidateDropPercent)
	assert.Equal(t, 20, setting.AlertDropPercent)
	assert.Equal(t, 15, setting.RecoveryDropPercent)

	e.PUT(baseAPI + "/shop/settings").WithJSON(schema.ShopSettingForm{
		CandidateDropPercent: 16, AlertDropPercent: 21, RecoveryDropPercent: 14,
	}).Expect().Status(http.StatusOK)

	e.PUT(baseAPI + "/shop/settings").WithJSON(schema.ShopSettingForm{
		CandidateDropPercent: 25, AlertDropPercent: 20, RecoveryDropPercent: 15,
	}).Expect().Status(http.StatusBadRequest)

	e.PUT(baseAPI + "/shop/settings").WithJSON(schema.ShopSettingForm{
		CandidateDropPercent: 15, AlertDropPercent: 20, RecoveryDropPercent: 15,
	}).Expect().Status(http.StatusOK)
}

func TestShopCategoryCRUD(t *testing.T) {
	e := tester(t)
	sourceURL := fmt.Sprintf("https://list.jd.com/list.html?cat=%d", time.Now().UnixNano())
	form := schema.JDCategoryForm{Name: "测试分类", SourceURL: sourceURL, Status: schema.StatusEnabled, MaxPages: 2}

	var created schema.JDCategory
	e.POST(baseAPI + "/shop/categories").WithJSON(form).Expect().Status(http.StatusOK).
		JSON().Decode(&util.ResponseResult{Data: &created})
	assert.NotEmpty(t, created.ID)
	assert.Equal(t, sourceURL, created.SourceURL)

	e.POST(baseAPI + "/shop/categories").WithJSON(form).Expect().Status(http.StatusConflict)
	e.POST(baseAPI + "/shop/categories").WithJSON(schema.JDCategoryForm{
		Name: "非法", SourceURL: "https://example.com/list", Status: schema.StatusEnabled, MaxPages: 1,
	}).Expect().Status(http.StatusBadRequest)

	form.Name = "更新分类"
	form.Status = schema.StatusDisabled
	e.PUT(baseAPI + "/shop/categories/" + created.ID).WithJSON(form).Expect().Status(http.StatusOK)

	var updated schema.JDCategory
	e.GET(baseAPI + "/shop/categories/" + created.ID).Expect().Status(http.StatusOK).
		JSON().Decode(&util.ResponseResult{Data: &updated})
	assert.Equal(t, "更新分类", updated.Name)
	assert.Equal(t, schema.StatusDisabled, updated.Status)

	e.DELETE(baseAPI + "/shop/categories/" + created.ID).Expect().Status(http.StatusOK)
	e.GET(baseAPI + "/shop/categories/" + created.ID).Expect().Status(http.StatusNotFound)
}

func TestShopMonitoringWorkflow(t *testing.T) {
	e := tester(t)
	service := appInjector.M.Shop.ManagementAPI.Service
	fakeJD := &fakeJDClient{publicPrice: 10000, checkoutPrice: 7999}
	fakeNotify := new(fakeNotifier)
	originalJD, originalNotifier := service.JDClient, service.Notifier
	originalPublicMin, originalPublicMax := config.C.Shop.JD.PublicDelayMin, config.C.Shop.JD.PublicDelayMax
	originalCheckoutMin, originalCheckoutMax := config.C.Shop.JD.CheckoutDelayMin, config.C.Shop.JD.CheckoutDelayMax
	service.JDClient, service.Notifier = fakeJD, fakeNotify
	config.C.Shop.JD.PublicDelayMin, config.C.Shop.JD.PublicDelayMax = 0, 0
	config.C.Shop.JD.CheckoutDelayMin, config.C.Shop.JD.CheckoutDelayMax = 0, 0
	defer func() {
		service.JDClient, service.Notifier = originalJD, originalNotifier
		config.C.Shop.JD.PublicDelayMin, config.C.Shop.JD.PublicDelayMax = originalPublicMin, originalPublicMax
		config.C.Shop.JD.CheckoutDelayMin, config.C.Shop.JD.CheckoutDelayMax = originalCheckoutMin, originalCheckoutMax
	}()

	form := schema.JDCategoryForm{
		Name: "监控流程测试", SourceURL: fmt.Sprintf("https://list.jd.com/list.html?workflow=%d", time.Now().UnixNano()),
		Status: schema.StatusEnabled, MaxPages: 1,
	}
	var category schema.JDCategory
	e.POST(baseAPI + "/shop/categories").WithJSON(form).Expect().Status(http.StatusOK).
		JSON().Decode(&util.ResponseResult{Data: &category})

	discoverJob := triggerShopJob(t, e, schema.JobTypeDiscover)
	waitShopJob(t, discoverJob.ID)

	var products schema.JDProductList
	e.GET(baseAPI+"/shop/products").WithQuery("current", 1).WithQuery("pageSize", 10).
		Expect().Status(http.StatusOK).JSON().Decode(&util.ResponseResult{Data: &products})
	if len(products) != 1 {
		t.Fatalf("discovered products = %d, want 1", len(products))
	}
	product := products[0]
	assert.True(t, product.SelfOperated)

	firstPublicJob := triggerShopJob(t, e, schema.JobTypePublicScan)
	waitShopJob(t, firstPublicJob.ID)
	fakeJD.setPublicPrice(8000)
	secondPublicJob := triggerShopJob(t, e, schema.JobTypePublicScan)
	waitShopJob(t, secondPublicJob.ID)

	var refreshed schema.JDProduct
	if err := appInjector.DB.Where("id = ?", product.ID).First(&refreshed).Error; err != nil {
		t.Fatalf("load product: %v", err)
	}
	assert.True(t, refreshed.CheckoutPending, "20%% public drop should enter checkout queue")

	now := time.Now()
	if err := appInjector.DB.Model(new(schema.JDProduct)).Where("id = ?", product.ID).Updates(map[string]interface{}{
		"first_seen_at": now.Add(-48 * time.Hour), "checkout_pending": true, "next_checkout_at": now,
	}).Error; err != nil {
		t.Fatalf("warm product: %v", err)
	}
	for i := 1; i <= 3; i++ {
		sample := &schema.PriceSample{
			ID: util.NewXID(), ProductID: product.ID, SampleType: schema.SampleTypeCheckout,
			PriceFen: 10000, Currency: "CNY", Source: "test", Valid: true,
			CollectedAt: now.Add(-time.Duration(4-i) * time.Hour), CreatedAt: now,
		}
		if err := appInjector.DB.Create(sample).Error; err != nil {
			t.Fatalf("create baseline sample: %v", err)
		}
	}

	checkoutJob := triggerShopJob(t, e, schema.JobTypeCheckoutSample)
	waitShopJob(t, checkoutJob.ID)
	assert.Equal(t, 1, fakeNotify.count())
	assertProductAlertState(t, product.ID, schema.AlertStateAlerting)

	queueProduct(t, product.ID)
	fakeJD.setCheckoutPrice(7000)
	waitShopJob(t, triggerShopJob(t, e, schema.JobTypeCheckoutSample).ID)
	assert.Equal(t, 1, fakeNotify.count(), "persistent low price should not notify again")

	queueProduct(t, product.ID)
	fakeJD.setCheckoutPrice(9000)
	waitShopJob(t, triggerShopJob(t, e, schema.JobTypeCheckoutSample).ID)
	assertProductAlertState(t, product.ID, schema.AlertStateArmed)

	queueProduct(t, product.ID)
	fakeJD.setCheckoutPrice(6000)
	waitShopJob(t, triggerShopJob(t, e, schema.JobTypeCheckoutSample).ID)
	assert.Equal(t, 2, fakeNotify.count(), "a new drop after recovery should notify again")
	assertProductAlertState(t, product.ID, schema.AlertStateAlerting)

	var alerts int64
	if err := appInjector.DB.Model(new(schema.PriceAlert)).Where("product_id = ? AND send_status = ?", product.ID, schema.AlertSendSent).Count(&alerts).Error; err != nil {
		t.Fatalf("count alerts: %v", err)
	}
	assert.Equal(t, int64(2), alerts)
}

func triggerShopJob(t *testing.T, e *httpexpect.Expect, jobType string) schema.JobRun {
	t.Helper()
	var job schema.JobRun
	e.POST(baseAPI + "/shop/jobs/" + jobType + "/run").Expect().Status(http.StatusOK).
		JSON().Decode(&util.ResponseResult{Data: &job})
	return job
}

func waitShopJob(t *testing.T, id string) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		var job schema.JobRun
		if err := appInjector.DB.Where("id = ?", id).First(&job).Error; err == nil {
			if job.Status == schema.JobStatusSucceeded {
				return
			}
			if job.Status == schema.JobStatusFailed || job.Status == schema.JobStatusSkipped {
				t.Fatalf("job %s ended as %s: %s", id, job.Status, job.ErrorSummary)
			}
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("job %s did not finish", id)
}

func queueProduct(t *testing.T, id string) {
	t.Helper()
	if err := appInjector.DB.Model(new(schema.JDProduct)).Where("id = ?", id).Updates(map[string]interface{}{
		"checkout_pending": true, "next_checkout_at": time.Now(),
	}).Error; err != nil {
		t.Fatalf("queue product: %v", err)
	}
}

func assertProductAlertState(t *testing.T, id, want string) {
	t.Helper()
	var product schema.JDProduct
	if err := appInjector.DB.Where("id = ?", id).First(&product).Error; err != nil {
		t.Fatalf("load product: %v", err)
	}
	assert.Equal(t, want, product.AlertState)
}
