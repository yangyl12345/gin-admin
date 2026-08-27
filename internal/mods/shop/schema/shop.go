package schema

import (
	"fmt"
	"net/url"
	"path"
	"strings"
	"time"

	"github.com/LyricTian/gin-admin/v10/internal/config"
	"github.com/LyricTian/gin-admin/v10/pkg/errors"
	"github.com/LyricTian/gin-admin/v10/pkg/util"
)

const (
	DefaultSettingID = "default"

	StatusEnabled  = "enabled"
	StatusDisabled = "disabled"

	MonitorStatusActive = "active"
	MonitorStatusPaused = "paused"

	DiscoveryStatusActive = "active"
	DiscoveryStatusStale  = "stale"
	DiscoveryStatusCapped = "capped"

	AlertStateArmed    = "armed"
	AlertStateAlerting = "alerting"

	SampleTypePublic   = "public"
	SampleTypeCheckout = "checkout"

	AlertSendPending = "pending"
	AlertSendSent    = "sent"
	AlertSendFailed  = "failed"

	JobTypeDiscover       = "discover"
	JobTypePublicScan     = "public-scan"
	JobTypeCheckoutSample = "checkout-sample"
	JobTypeCleanup        = "cleanup"
	JobTypeAlertDelivery  = "alert-delivery"

	JobTriggerSchedule = "schedule"
	JobTriggerManual   = "manual"

	JobStatusQueued    = "queued"
	JobStatusRunning   = "running"
	JobStatusSucceeded = "succeeded"
	JobStatusFailed    = "failed"
	JobStatusSkipped   = "skipped"
)

type ShopSetting struct {
	ID                   string    `gorm:"size:20;primaryKey" json:"id"`
	CandidateDropPercent int       `gorm:"not null" json:"candidate_drop_percent"`
	AlertDropPercent     int       `gorm:"not null" json:"alert_drop_percent"`
	RecoveryDropPercent  int       `gorm:"not null" json:"recovery_drop_percent"`
	CreatedAt            time.Time `json:"created_at"`
	UpdatedAt            time.Time `json:"updated_at"`
}

func (*ShopSetting) TableName() string { return config.C.FormatTableName("shop_setting") }

type ShopSettingForm struct {
	CandidateDropPercent int `json:"candidate_drop_percent" binding:"required,min=1,max=90"`
	AlertDropPercent     int `json:"alert_drop_percent" binding:"required,min=1,max=90"`
	RecoveryDropPercent  int `json:"recovery_drop_percent" binding:"required,min=1,max=90"`
}

func (a *ShopSettingForm) Validate() error {
	if a.CandidateDropPercent > a.AlertDropPercent {
		return errors.BadRequest("invalid_candidate_threshold", "candidate_drop_percent must not exceed alert_drop_percent")
	}
	if a.RecoveryDropPercent >= a.AlertDropPercent {
		return errors.BadRequest("invalid_recovery_threshold", "recovery_drop_percent must be lower than alert_drop_percent")
	}
	return nil
}

type JDCategory struct {
	ID                  string     `gorm:"size:20;primaryKey" json:"id"`
	Name                string     `gorm:"size:128;not null" json:"name"`
	SourceURL           string     `gorm:"size:768;uniqueIndex;not null" json:"source_url"`
	Status              string     `gorm:"size:20;index;not null" json:"status"`
	MaxPages            int        `gorm:"not null" json:"max_pages"`
	LastDiscoveryStatus string     `gorm:"size:20;index" json:"last_discovery_status"`
	LastDiscoveryError  string     `gorm:"size:1024" json:"last_discovery_error,omitempty"`
	LastDiscoveredAt    *time.Time `gorm:"index" json:"last_discovered_at,omitempty"`
	CreatedAt           time.Time  `json:"created_at"`
	UpdatedAt           time.Time  `json:"updated_at"`
}

func (*JDCategory) TableName() string { return config.C.FormatTableName("shop_jd_category") }

type JDCategoryForm struct {
	Name      string `json:"name" binding:"required,max=128"`
	SourceURL string `json:"source_url" binding:"required,max=768"`
	Status    string `json:"status" binding:"required,oneof=enabled disabled"`
	MaxPages  int    `json:"max_pages" binding:"required,min=1,max=50"`
}

func (a *JDCategoryForm) Validate() error {
	a.Name = strings.TrimSpace(a.Name)
	a.SourceURL = strings.TrimSpace(a.SourceURL)
	if err := ValidateJDCategoryURL(a.SourceURL); err != nil {
		return err
	}
	return nil
}

type JDCategoryQueryParam struct {
	util.PaginationParam
	LikeName string `form:"name"`
	Status   string `form:"status" binding:"omitempty,oneof=enabled disabled"`
}

type JDCategoryQueryResult struct {
	Data       JDCategoryList
	PageResult *util.PaginationResult
}

type JDCategoryList []*JDCategory

type JDProduct struct {
	ID              string     `gorm:"size:20;primaryKey" json:"id"`
	SKU             string     `gorm:"size:64;uniqueIndex;not null" json:"sku"`
	Name            string     `gorm:"size:512;not null" json:"name"`
	CanonicalURL    string     `gorm:"size:1024;not null" json:"canonical_url"`
	ImageURL        string     `gorm:"size:1024" json:"image_url,omitempty"`
	SelfOperated    bool       `gorm:"index;not null" json:"self_operated"`
	MonitorStatus   string     `gorm:"size:20;index;not null" json:"monitor_status"`
	DiscoveryStatus string     `gorm:"size:20;index;not null" json:"discovery_status"`
	AlertState      string     `gorm:"size:20;index;not null" json:"alert_state"`
	CheckoutPending bool       `gorm:"index;not null" json:"checkout_pending"`
	NextCheckoutAt  *time.Time `gorm:"index" json:"next_checkout_at,omitempty"`
	FirstSeenAt     time.Time  `gorm:"index;not null" json:"first_seen_at"`
	LastSeenAt      time.Time  `gorm:"index;not null" json:"last_seen_at"`
	CreatedAt       time.Time  `json:"created_at"`
	UpdatedAt       time.Time  `json:"updated_at"`
}

func (*JDProduct) TableName() string { return config.C.FormatTableName("shop_jd_product") }

type JDProductForm struct {
	MonitorStatus string `json:"monitor_status" binding:"required,oneof=active paused"`
}

type JDProductQueryParam struct {
	util.PaginationParam
	SKU             string `form:"sku"`
	LikeName        string `form:"name"`
	CategoryID      string `form:"categoryID"`
	SelfOperated    *bool  `form:"selfOperated"`
	MonitorStatus   string `form:"monitorStatus" binding:"omitempty,oneof=active paused"`
	DiscoveryStatus string `form:"discoveryStatus" binding:"omitempty,oneof=active stale capped"`
}

type JDProductQueryResult struct {
	Data       JDProductList
	PageResult *util.PaginationResult
}

type JDProductList []*JDProduct

type JDCategoryProduct struct {
	ID          string    `gorm:"size:20;primaryKey" json:"id"`
	CategoryID  string    `gorm:"size:20;uniqueIndex:idx_shop_category_product;index;not null" json:"category_id"`
	ProductID   string    `gorm:"size:20;uniqueIndex:idx_shop_category_product;index;not null" json:"product_id"`
	MissCount   int       `gorm:"index;not null" json:"miss_count"`
	FirstSeenAt time.Time `gorm:"not null" json:"first_seen_at"`
	LastSeenAt  time.Time `gorm:"index;not null" json:"last_seen_at"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

func (*JDCategoryProduct) TableName() string {
	return config.C.FormatTableName("shop_jd_category_product")
}

type PriceSample struct {
	ID          string    `gorm:"size:20;primaryKey" json:"id"`
	ProductID   string    `gorm:"size:20;index:idx_shop_price_product_type_time,priority:1;not null" json:"product_id"`
	SampleType  string    `gorm:"size:20;index:idx_shop_price_product_type_time,priority:2;not null" json:"sample_type"`
	PriceFen    int64     `gorm:"not null" json:"price_fen"`
	Currency    string    `gorm:"size:8;not null" json:"currency"`
	Source      string    `gorm:"size:64;not null" json:"source"`
	Valid       bool      `gorm:"index;not null" json:"valid"`
	CollectedAt time.Time `gorm:"index:idx_shop_price_product_type_time,priority:3;index;not null" json:"collected_at"`
	CreatedAt   time.Time `json:"created_at"`
	PriceYuan   string    `gorm:"-" json:"price_yuan"`
}

func (*PriceSample) TableName() string { return config.C.FormatTableName("shop_price_sample") }

func (a *PriceSample) FillDisplay() { a.PriceYuan = FormatPriceYuan(a.PriceFen) }

type PriceSampleQueryParam struct {
	util.PaginationParam
	ProductID  string `form:"-"`
	SampleType string `form:"type" binding:"omitempty,oneof=public checkout"`
	StartTime  string `form:"startTime"`
	EndTime    string `form:"endTime"`
}

type PriceSampleQueryResult struct {
	Data       PriceSampleList
	PageResult *util.PaginationResult
}

type PriceSampleList []*PriceSample

type PriceStats struct {
	AverageFen int64
	Count      int64
}

type PriceAlert struct {
	ID                string     `gorm:"size:20;primaryKey" json:"id"`
	ProductID         string     `gorm:"size:20;index;not null" json:"product_id"`
	TriggerSampleID   string     `gorm:"size:20;uniqueIndex;not null" json:"trigger_sample_id"`
	BaselinePriceFen  int64      `gorm:"not null" json:"baseline_price_fen"`
	CurrentPriceFen   int64      `gorm:"not null" json:"current_price_fen"`
	DropBasisPoints   int        `gorm:"not null" json:"drop_basis_points"`
	SendStatus        string     `gorm:"size:20;index;not null" json:"send_status"`
	SendAttempts      int        `gorm:"not null" json:"send_attempts"`
	ProviderResponse  string     `gorm:"size:2048" json:"provider_response,omitempty"`
	LastError         string     `gorm:"size:1024" json:"last_error,omitempty"`
	SentAt            *time.Time `gorm:"index" json:"sent_at,omitempty"`
	CreatedAt         time.Time  `gorm:"index;not null" json:"created_at"`
	UpdatedAt         time.Time  `json:"updated_at"`
	BaselinePriceYuan string     `gorm:"-" json:"baseline_price_yuan"`
	CurrentPriceYuan  string     `gorm:"-" json:"current_price_yuan"`
	Product           *JDProduct `gorm:"-" json:"product,omitempty"`
}

func (*PriceAlert) TableName() string { return config.C.FormatTableName("shop_price_alert") }

func (a *PriceAlert) FillDisplay() {
	a.BaselinePriceYuan = FormatPriceYuan(a.BaselinePriceFen)
	a.CurrentPriceYuan = FormatPriceYuan(a.CurrentPriceFen)
}

type PriceAlertQueryParam struct {
	util.PaginationParam
	ProductID  string `form:"productID"`
	SendStatus string `form:"sendStatus" binding:"omitempty,oneof=pending sent failed"`
	StartTime  string `form:"startTime"`
	EndTime    string `form:"endTime"`
}

type PriceAlertQueryResult struct {
	Data       PriceAlertList
	PageResult *util.PaginationResult
}

type PriceAlertList []*PriceAlert

type JobRun struct {
	ID           string     `gorm:"size:20;primaryKey" json:"id"`
	JobType      string     `gorm:"size:32;index;not null" json:"job_type"`
	Trigger      string     `gorm:"size:20;index;not null" json:"trigger"`
	Status       string     `gorm:"size:20;index;not null" json:"status"`
	ScannedCount int        `gorm:"not null" json:"scanned_count"`
	SuccessCount int        `gorm:"not null" json:"success_count"`
	FailureCount int        `gorm:"not null" json:"failure_count"`
	Cursor       string     `gorm:"size:128" json:"cursor,omitempty"`
	ErrorSummary string     `gorm:"size:2048" json:"error_summary,omitempty"`
	StartedAt    *time.Time `gorm:"index" json:"started_at,omitempty"`
	FinishedAt   *time.Time `gorm:"index" json:"finished_at,omitempty"`
	CreatedAt    time.Time  `gorm:"index;not null" json:"created_at"`
	UpdatedAt    time.Time  `json:"updated_at"`
}

func (*JobRun) TableName() string { return config.C.FormatTableName("shop_job_run") }

type JobRunQueryParam struct {
	util.PaginationParam
	JobType string `form:"type"`
	Status  string `form:"status"`
}

type JobRunQueryResult struct {
	Data       JobRunList
	PageResult *util.PaginationResult
}

type JobRunList []*JobRun

type SessionStatus struct {
	Authenticated  bool      `json:"authenticated"`
	CaptchaBlocked bool      `json:"captcha_blocked"`
	LastCheckedAt  time.Time `json:"last_checked_at"`
	ErrorSummary   string    `json:"error_summary,omitempty"`
}

type NotificationTestResult struct {
	Provider string `json:"provider"`
	Response string `json:"response"`
}

type ProductDetail struct {
	*JDProduct
	Categories          JDCategoryList `json:"categories"`
	LatestPublicPrice   *PriceSample   `json:"latest_public_price,omitempty"`
	LatestCheckoutPrice *PriceSample   `json:"latest_checkout_price,omitempty"`
	CheckoutAverageFen  int64          `json:"checkout_average_fen"`
	CheckoutAverageYuan string         `json:"checkout_average_yuan"`
}

type DiscoveredProduct struct {
	SKU          string
	Name         string
	CanonicalURL string
	ImageURL     string
	SelfOperated bool
}

type PriceObservation struct {
	SKU          string
	Name         string
	CanonicalURL string
	PriceFen     int64
	Currency     string
	SelfOperated bool
}

func ValidateJDCategoryURL(raw string) error {
	u, err := url.Parse(raw)
	if err != nil || u.Scheme != "https" || u.User != nil {
		return errors.BadRequest("invalid_jd_url", "source_url must be a valid HTTPS JD category URL")
	}
	host := strings.ToLower(u.Hostname())
	allowed := map[string]struct{}{
		"list.jd.com": {}, "search.jd.com": {}, "channel.jd.com": {},
		"m.jd.com": {}, "so.m.jd.com": {}, "pro.m.jd.com": {},
	}
	if _, ok := allowed[host]; !ok {
		return errors.BadRequest("invalid_jd_url", "source_url host is not an allowed JD category host")
	}
	if port := u.Port(); port != "" && port != "443" {
		return errors.BadRequest("invalid_jd_url", "source_url must not use a custom port")
	}
	if !validJDCategoryPath(host, u.Path) {
		return errors.BadRequest("invalid_jd_url", "source_url path is not a recognized JD category or list path")
	}
	u.Fragment = ""
	return nil
}

func validJDCategoryPath(host, rawPath string) bool {
	cleanPath := strings.ToLower(path.Clean("/" + strings.TrimPrefix(rawPath, "/")))
	switch host {
	case "list.jd.com":
		return cleanPath == "/list.html" || strings.HasPrefix(cleanPath, "/list/")
	case "search.jd.com":
		return cleanPath == "/search" || strings.HasPrefix(cleanPath, "/search/")
	case "channel.jd.com":
		return cleanPath != "/" && strings.HasSuffix(cleanPath, ".html")
	case "m.jd.com":
		return cleanPath == "/category/all.html" || strings.HasPrefix(cleanPath, "/category/")
	case "so.m.jd.com":
		return strings.HasPrefix(cleanPath, "/ware/search")
	case "pro.m.jd.com":
		return strings.HasPrefix(cleanPath, "/mall/active/")
	default:
		return false
	}
}

func ValidateJDNavigationURL(raw string) error {
	u, err := url.Parse(raw)
	if err != nil || u.Scheme != "https" || u.User != nil {
		return errors.BadRequest("invalid_jd_navigation", "JD navigation left the allowed HTTPS origin")
	}
	host := strings.ToLower(u.Hostname())
	if host != "jd.com" && !strings.HasSuffix(host, ".jd.com") {
		return errors.BadRequest("invalid_jd_navigation", "JD navigation left the allowed domain")
	}
	if port := u.Port(); port != "" && port != "443" {
		return errors.BadRequest("invalid_jd_navigation", "JD navigation used a custom port")
	}
	return nil
}

func FormatPriceYuan(fen int64) string {
	return fmt.Sprintf("%d.%02d", fen/100, fen%100)
}

func ValidJobType(jobType string) bool {
	switch jobType {
	case JobTypeDiscover, JobTypePublicScan, JobTypeCheckoutSample, JobTypeCleanup, JobTypeAlertDelivery:
		return true
	default:
		return false
	}
}

func ValidManualJobType(jobType string) bool {
	switch jobType {
	case JobTypeDiscover, JobTypePublicScan, JobTypeCheckoutSample:
		return true
	default:
		return false
	}
}
