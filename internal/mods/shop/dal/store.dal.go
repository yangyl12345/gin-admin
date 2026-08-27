package dal

import (
	"context"
	"time"

	"github.com/LyricTian/gin-admin/v10/internal/mods/shop/schema"
	"github.com/LyricTian/gin-admin/v10/pkg/errors"
	"github.com/LyricTian/gin-admin/v10/pkg/util"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type Store struct {
	DB *gorm.DB
}

func (a *Store) db(ctx context.Context) *gorm.DB { return util.GetDB(ctx, a.DB) }

func (a *Store) GetSetting(ctx context.Context) (*schema.ShopSetting, bool, error) {
	item := new(schema.ShopSetting)
	ok, err := util.FindOne(ctx, a.db(ctx).Where("id = ?", schema.DefaultSettingID), util.QueryOptions{}, item)
	return item, ok, errors.WithStack(err)
}

func (a *Store) CreateSetting(ctx context.Context, item *schema.ShopSetting) error {
	return errors.WithStack(a.db(ctx).Create(item).Error)
}

func (a *Store) UpdateSetting(ctx context.Context, item *schema.ShopSetting) error {
	return errors.WithStack(a.db(ctx).Model(item).Select("candidate_drop_percent", "alert_drop_percent", "recovery_drop_percent", "updated_at").Updates(item).Error)
}

func (a *Store) QueryCategories(ctx context.Context, params schema.JDCategoryQueryParam) (*schema.JDCategoryQueryResult, error) {
	db := a.db(ctx).Model(new(schema.JDCategory))
	if params.LikeName != "" {
		db = db.Where("name LIKE ?", "%"+params.LikeName+"%")
	}
	if params.Status != "" {
		db = db.Where("status = ?", params.Status)
	}
	var list schema.JDCategoryList
	pr, err := util.WrapPageQuery(ctx, db, params.PaginationParam, util.QueryOptions{
		OrderFields: []util.OrderByParam{{Field: "created_at", Direction: util.DESC}},
	}, &list)
	if err != nil {
		return nil, errors.WithStack(err)
	}
	return &schema.JDCategoryQueryResult{Data: list, PageResult: pr}, nil
}

func (a *Store) ListEnabledCategories(ctx context.Context) (schema.JDCategoryList, error) {
	var list schema.JDCategoryList
	err := a.db(ctx).Where("status = ?", schema.StatusEnabled).Order("created_at ASC").Find(&list).Error
	return list, errors.WithStack(err)
}

func (a *Store) GetCategory(ctx context.Context, id string) (*schema.JDCategory, bool, error) {
	item := new(schema.JDCategory)
	ok, err := util.FindOne(ctx, a.db(ctx).Where("id = ?", id), util.QueryOptions{}, item)
	return item, ok, errors.WithStack(err)
}

func (a *Store) CategoryURLExists(ctx context.Context, sourceURL, excludeID string) (bool, error) {
	db := a.db(ctx).Model(new(schema.JDCategory)).Where("source_url = ?", sourceURL)
	if excludeID != "" {
		db = db.Where("id <> ?", excludeID)
	}
	ok, err := util.Exists(ctx, db)
	return ok, errors.WithStack(err)
}

func (a *Store) CreateCategory(ctx context.Context, item *schema.JDCategory) error {
	return errors.WithStack(a.db(ctx).Create(item).Error)
}

func (a *Store) UpdateCategory(ctx context.Context, item *schema.JDCategory) error {
	return errors.WithStack(a.db(ctx).Model(item).Select("name", "source_url", "status", "max_pages", "updated_at").Updates(item).Error)
}

func (a *Store) UpdateCategoryDiscovery(ctx context.Context, id, status, summary string, at time.Time) error {
	return errors.WithStack(a.db(ctx).Model(new(schema.JDCategory)).Where("id = ?", id).Updates(map[string]interface{}{
		"last_discovery_status": status,
		"last_discovery_error":  summary,
		"last_discovered_at":    at,
		"updated_at":            at,
	}).Error)
}

func (a *Store) DeleteCategory(ctx context.Context, id string) error {
	return errors.WithStack(a.db(ctx).Where("id = ?", id).Delete(new(schema.JDCategory)).Error)
}

func (a *Store) DeleteCategoryProducts(ctx context.Context, categoryID string) error {
	return errors.WithStack(a.db(ctx).Where("category_id = ?", categoryID).Delete(new(schema.JDCategoryProduct)).Error)
}

func (a *Store) ProductIDsForCategory(ctx context.Context, categoryID string) ([]string, error) {
	var ids []string
	err := a.db(ctx).Model(new(schema.JDCategoryProduct)).Where("category_id = ?", categoryID).Pluck("product_id", &ids).Error
	return ids, errors.WithStack(err)
}

func (a *Store) QueryProducts(ctx context.Context, params schema.JDProductQueryParam) (*schema.JDProductQueryResult, error) {
	db := a.db(ctx).Model(new(schema.JDProduct)).Distinct()
	if params.CategoryID != "" {
		db = db.Joins("JOIN "+new(schema.JDCategoryProduct).TableName()+" cp ON cp.product_id = "+new(schema.JDProduct).TableName()+".id").Where("cp.category_id = ?", params.CategoryID)
	}
	if params.SKU != "" {
		db = db.Where(new(schema.JDProduct).TableName()+".sku = ?", params.SKU)
	}
	if params.LikeName != "" {
		db = db.Where(new(schema.JDProduct).TableName()+".name LIKE ?", "%"+params.LikeName+"%")
	}
	if params.SelfOperated != nil {
		db = db.Where(new(schema.JDProduct).TableName()+".self_operated = ?", *params.SelfOperated)
	}
	if params.MonitorStatus != "" {
		db = db.Where(new(schema.JDProduct).TableName()+".monitor_status = ?", params.MonitorStatus)
	}
	if params.DiscoveryStatus != "" {
		db = db.Where(new(schema.JDProduct).TableName()+".discovery_status = ?", params.DiscoveryStatus)
	}
	var list schema.JDProductList
	pr, err := util.WrapPageQuery(ctx, db, params.PaginationParam, util.QueryOptions{
		OrderFields: []util.OrderByParam{{Field: new(schema.JDProduct).TableName() + ".last_seen_at", Direction: util.DESC}},
	}, &list)
	if err != nil {
		return nil, errors.WithStack(err)
	}
	return &schema.JDProductQueryResult{Data: list, PageResult: pr}, nil
}

func (a *Store) GetProduct(ctx context.Context, id string) (*schema.JDProduct, bool, error) {
	item := new(schema.JDProduct)
	ok, err := util.FindOne(ctx, a.db(ctx).Where("id = ?", id), util.QueryOptions{}, item)
	return item, ok, errors.WithStack(err)
}

func (a *Store) GetProductBySKU(ctx context.Context, sku string) (*schema.JDProduct, bool, error) {
	item := new(schema.JDProduct)
	ok, err := util.FindOne(ctx, a.db(ctx).Where("sku = ?", sku), util.QueryOptions{}, item)
	return item, ok, errors.WithStack(err)
}

func (a *Store) CountActiveProducts(ctx context.Context) (int64, error) {
	var count int64
	err := a.db(ctx).Model(new(schema.JDProduct)).Where("monitor_status = ? AND discovery_status = ? AND self_operated = ?", schema.MonitorStatusActive, schema.DiscoveryStatusActive, true).Count(&count).Error
	return count, errors.WithStack(err)
}

func (a *Store) CreateProduct(ctx context.Context, item *schema.JDProduct) error {
	return errors.WithStack(a.db(ctx).Create(item).Error)
}

func (a *Store) UpdateProduct(ctx context.Context, item *schema.JDProduct, fields ...string) error {
	db := a.db(ctx).Model(item)
	if len(fields) > 0 {
		db = db.Select(fields)
	}
	return errors.WithStack(db.Updates(item).Error)
}

func (a *Store) UpdateProductColumns(ctx context.Context, id string, fields map[string]interface{}) error {
	return errors.WithStack(a.db(ctx).Model(new(schema.JDProduct)).Where("id = ?", id).Updates(fields).Error)
}

func (a *Store) ListActiveProducts(ctx context.Context, limit int) (schema.JDProductList, error) {
	var list schema.JDProductList
	err := a.db(ctx).Where("monitor_status = ? AND discovery_status = ? AND self_operated = ?", schema.MonitorStatusActive, schema.DiscoveryStatusActive, true).Order("last_seen_at DESC").Limit(limit).Find(&list).Error
	return list, errors.WithStack(err)
}

func (a *Store) ListCheckoutDueProducts(ctx context.Context, at time.Time, limit int) (schema.JDProductList, error) {
	var list schema.JDProductList
	err := a.db(ctx).
		Where("monitor_status = ? AND discovery_status = ? AND self_operated = ?", schema.MonitorStatusActive, schema.DiscoveryStatusActive, true).
		Where("checkout_pending = ? OR next_checkout_at IS NULL OR next_checkout_at <= ?", true, at).
		Order("checkout_pending DESC, next_checkout_at ASC, first_seen_at ASC").
		Limit(limit).Find(&list).Error
	return list, errors.WithStack(err)
}

func (a *Store) ListProductCategories(ctx context.Context, productID string) (schema.JDCategoryList, error) {
	var list schema.JDCategoryList
	err := a.db(ctx).Model(new(schema.JDCategory)).
		Joins("JOIN "+new(schema.JDCategoryProduct).TableName()+" cp ON cp.category_id = "+new(schema.JDCategory).TableName()+".id").
		Where("cp.product_id = ?", productID).Order(new(schema.JDCategory).TableName() + ".name ASC").Find(&list).Error
	return list, errors.WithStack(err)
}

func (a *Store) IncrementCategoryMisses(ctx context.Context, categoryID string) error {
	return errors.WithStack(a.db(ctx).Model(new(schema.JDCategoryProduct)).Where("category_id = ?", categoryID).UpdateColumn("miss_count", gorm.Expr("miss_count + 1")).Error)
}

func (a *Store) UpsertCategoryProduct(ctx context.Context, item *schema.JDCategoryProduct) error {
	return errors.WithStack(a.db(ctx).Clauses(clause.OnConflict{
		Columns: []clause.Column{{Name: "category_id"}, {Name: "product_id"}},
		DoUpdates: clause.Assignments(map[string]interface{}{
			"miss_count":   0,
			"last_seen_at": item.LastSeenAt,
			"updated_at":   item.UpdatedAt,
		}),
	}).Create(item).Error)
}

func (a *Store) HasFreshCategoryRelation(ctx context.Context, productID string) (bool, error) {
	db := a.db(ctx).Model(new(schema.JDCategoryProduct)).
		Joins("JOIN "+new(schema.JDCategory).TableName()+" c ON c.id = "+new(schema.JDCategoryProduct).TableName()+".category_id").
		Where(new(schema.JDCategoryProduct).TableName()+".product_id = ?", productID).
		Where(new(schema.JDCategoryProduct).TableName()+".miss_count < ?", 3).
		Where("c.status = ?", schema.StatusEnabled)
	ok, err := util.Exists(ctx, db)
	return ok, errors.WithStack(err)
}

func (a *Store) CreatePriceSample(ctx context.Context, item *schema.PriceSample) error {
	return errors.WithStack(a.db(ctx).Create(item).Error)
}

func (a *Store) QueryPriceSamples(ctx context.Context, params schema.PriceSampleQueryParam) (*schema.PriceSampleQueryResult, error) {
	db := a.db(ctx).Model(new(schema.PriceSample)).Where("product_id = ?", params.ProductID)
	if params.SampleType != "" {
		db = db.Where("sample_type = ?", params.SampleType)
	}
	if params.StartTime != "" {
		db = db.Where("collected_at >= ?", params.StartTime)
	}
	if params.EndTime != "" {
		db = db.Where("collected_at <= ?", params.EndTime)
	}
	var list schema.PriceSampleList
	pr, err := util.WrapPageQuery(ctx, db, params.PaginationParam, util.QueryOptions{
		OrderFields: []util.OrderByParam{{Field: "collected_at", Direction: util.DESC}},
	}, &list)
	if err != nil {
		return nil, errors.WithStack(err)
	}
	return &schema.PriceSampleQueryResult{Data: list, PageResult: pr}, nil
}

func (a *Store) PriceStatsBefore(ctx context.Context, productID, sampleType string, before, since time.Time) (*schema.PriceStats, error) {
	var row struct {
		Total int64
		Count int64
	}
	err := a.db(ctx).Model(new(schema.PriceSample)).
		Select("COALESCE(SUM(price_fen), 0) AS total, COUNT(*) AS count").
		Where("product_id = ? AND sample_type = ? AND valid = ?", productID, sampleType, true).
		Where("collected_at >= ? AND collected_at < ?", since, before).
		Scan(&row).Error
	if err != nil {
		return nil, errors.WithStack(err)
	}
	average := int64(0)
	if row.Count > 0 {
		average = row.Total / row.Count
		if row.Total%row.Count >= (row.Count+1)/2 {
			average++
		}
	}
	return &schema.PriceStats{AverageFen: average, Count: row.Count}, nil
}

func (a *Store) LatestPriceSample(ctx context.Context, productID, sampleType string) (*schema.PriceSample, bool, error) {
	item := new(schema.PriceSample)
	ok, err := util.FindOne(ctx, a.db(ctx).Where("product_id = ? AND sample_type = ? AND valid = ?", productID, sampleType, true), util.QueryOptions{
		OrderFields: []util.OrderByParam{{Field: "collected_at", Direction: util.DESC}},
	}, item)
	return item, ok, errors.WithStack(err)
}

func (a *Store) DeletePriceSamplesBefore(ctx context.Context, before time.Time) (int64, error) {
	r := a.db(ctx).Where("collected_at < ?", before).Delete(new(schema.PriceSample))
	return r.RowsAffected, errors.WithStack(r.Error)
}

func (a *Store) QueryAlerts(ctx context.Context, params schema.PriceAlertQueryParam) (*schema.PriceAlertQueryResult, error) {
	db := a.db(ctx).Model(new(schema.PriceAlert))
	if params.ProductID != "" {
		db = db.Where("product_id = ?", params.ProductID)
	}
	if params.SendStatus != "" {
		db = db.Where("send_status = ?", params.SendStatus)
	}
	if params.StartTime != "" {
		db = db.Where("created_at >= ?", params.StartTime)
	}
	if params.EndTime != "" {
		db = db.Where("created_at <= ?", params.EndTime)
	}
	var list schema.PriceAlertList
	pr, err := util.WrapPageQuery(ctx, db, params.PaginationParam, util.QueryOptions{
		OrderFields: []util.OrderByParam{{Field: "created_at", Direction: util.DESC}},
	}, &list)
	if err != nil {
		return nil, errors.WithStack(err)
	}
	return &schema.PriceAlertQueryResult{Data: list, PageResult: pr}, nil
}

func (a *Store) CreateAlert(ctx context.Context, item *schema.PriceAlert) error {
	return errors.WithStack(a.db(ctx).Create(item).Error)
}

func (a *Store) ListPendingAlerts(ctx context.Context, limit int) (schema.PriceAlertList, error) {
	var list schema.PriceAlertList
	err := a.db(ctx).Where("send_status IN ? AND send_attempts < ?", []string{schema.AlertSendPending, schema.AlertSendFailed}, 10).Order("created_at ASC").Limit(limit).Find(&list).Error
	return list, errors.WithStack(err)
}

func (a *Store) UpdateAlertDelivery(ctx context.Context, id string, fields map[string]interface{}) error {
	return errors.WithStack(a.db(ctx).Model(new(schema.PriceAlert)).Where("id = ?", id).Updates(fields).Error)
}

func (a *Store) DeleteAlertsBefore(ctx context.Context, before time.Time) (int64, error) {
	r := a.db(ctx).Where("created_at < ?", before).Delete(new(schema.PriceAlert))
	return r.RowsAffected, errors.WithStack(r.Error)
}

func (a *Store) QueryJobs(ctx context.Context, params schema.JobRunQueryParam) (*schema.JobRunQueryResult, error) {
	db := a.db(ctx).Model(new(schema.JobRun))
	if params.JobType != "" {
		db = db.Where("job_type = ?", params.JobType)
	}
	if params.Status != "" {
		db = db.Where("status = ?", params.Status)
	}
	var list schema.JobRunList
	pr, err := util.WrapPageQuery(ctx, db, params.PaginationParam, util.QueryOptions{
		OrderFields: []util.OrderByParam{{Field: "created_at", Direction: util.DESC}},
	}, &list)
	if err != nil {
		return nil, errors.WithStack(err)
	}
	return &schema.JobRunQueryResult{Data: list, PageResult: pr}, nil
}

func (a *Store) CreateJob(ctx context.Context, item *schema.JobRun) error {
	return errors.WithStack(a.db(ctx).Create(item).Error)
}

func (a *Store) UpdateJob(ctx context.Context, id string, fields map[string]interface{}) error {
	return errors.WithStack(a.db(ctx).Model(new(schema.JobRun)).Where("id = ?", id).Updates(fields).Error)
}

func (a *Store) DeleteJobsBefore(ctx context.Context, before time.Time) (int64, error) {
	r := a.db(ctx).Where("created_at < ?", before).Delete(new(schema.JobRun))
	return r.RowsAffected, errors.WithStack(r.Error)
}
