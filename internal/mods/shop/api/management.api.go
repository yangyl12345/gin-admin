package api

import (
	"github.com/LyricTian/gin-admin/v10/internal/mods/shop/biz"
	"github.com/LyricTian/gin-admin/v10/internal/mods/shop/schema"
	"github.com/LyricTian/gin-admin/v10/pkg/util"
	"github.com/gin-gonic/gin"
)

type Management struct {
	Service *biz.Service
}

// @Tags ShopAPI
// @Summary Get shop monitoring settings
// @Success 200 {object} util.ResponseResult{data=schema.ShopSetting}
// @Failure 500 {object} util.ResponseResult
// @Router /api/v1/shop/settings [get]
func (a *Management) GetSetting(c *gin.Context) {
	item, err := a.Service.GetSetting(c.Request.Context())
	if err != nil {
		util.ResError(c, err)
		return
	}
	util.ResSuccess(c, item)
}

// @Tags ShopAPI
// @Summary Update shop monitoring settings
// @Param body body schema.ShopSettingForm true "settings"
// @Success 200 {object} util.ResponseResult
// @Failure 400 {object} util.ResponseResult
// @Failure 500 {object} util.ResponseResult
// @Router /api/v1/shop/settings [put]
func (a *Management) UpdateSetting(c *gin.Context) {
	form := new(schema.ShopSettingForm)
	if err := util.ParseJSON(c, form); err != nil {
		util.ResError(c, err)
		return
	}
	if err := a.Service.UpdateSetting(c.Request.Context(), form); err != nil {
		util.ResError(c, err)
		return
	}
	util.ResOK(c)
}

// @Tags ShopAPI
// @Summary Query JD categories
// @Param current query int true "pagination index" default(1)
// @Param pageSize query int true "pagination size" default(10)
// @Param name query string false "category name"
// @Param status query string false "enabled or disabled"
// @Success 200 {object} util.ResponseResult{data=[]schema.JDCategory}
// @Failure 400 {object} util.ResponseResult
// @Failure 500 {object} util.ResponseResult
// @Router /api/v1/shop/categories [get]
func (a *Management) QueryCategories(c *gin.Context) {
	var params schema.JDCategoryQueryParam
	if err := util.ParseQuery(c, &params); err != nil {
		util.ResError(c, err)
		return
	}
	result, err := a.Service.QueryCategories(c.Request.Context(), params)
	if err != nil {
		util.ResError(c, err)
		return
	}
	util.ResPage(c, result.Data, result.PageResult)
}

// @Tags ShopAPI
// @Summary Get JD category
// @Param id path string true "category ID"
// @Success 200 {object} util.ResponseResult{data=schema.JDCategory}
// @Failure 404 {object} util.ResponseResult
// @Failure 500 {object} util.ResponseResult
// @Router /api/v1/shop/categories/{id} [get]
func (a *Management) GetCategory(c *gin.Context) {
	item, err := a.Service.GetCategory(c.Request.Context(), c.Param("id"))
	if err != nil {
		util.ResError(c, err)
		return
	}
	util.ResSuccess(c, item)
}

// @Tags ShopAPI
// @Summary Create JD category
// @Param body body schema.JDCategoryForm true "category"
// @Success 200 {object} util.ResponseResult{data=schema.JDCategory}
// @Failure 400 {object} util.ResponseResult
// @Failure 409 {object} util.ResponseResult
// @Failure 500 {object} util.ResponseResult
// @Router /api/v1/shop/categories [post]
func (a *Management) CreateCategory(c *gin.Context) {
	form := new(schema.JDCategoryForm)
	if err := util.ParseJSON(c, form); err != nil {
		util.ResError(c, err)
		return
	}
	item, err := a.Service.CreateCategory(c.Request.Context(), form)
	if err != nil {
		util.ResError(c, err)
		return
	}
	util.ResSuccess(c, item)
}

// @Tags ShopAPI
// @Summary Update JD category
// @Param id path string true "category ID"
// @Param body body schema.JDCategoryForm true "category"
// @Success 200 {object} util.ResponseResult
// @Failure 400 {object} util.ResponseResult
// @Failure 404 {object} util.ResponseResult
// @Failure 409 {object} util.ResponseResult
// @Failure 500 {object} util.ResponseResult
// @Router /api/v1/shop/categories/{id} [put]
func (a *Management) UpdateCategory(c *gin.Context) {
	form := new(schema.JDCategoryForm)
	if err := util.ParseJSON(c, form); err != nil {
		util.ResError(c, err)
		return
	}
	if err := a.Service.UpdateCategory(c.Request.Context(), c.Param("id"), form); err != nil {
		util.ResError(c, err)
		return
	}
	util.ResOK(c)
}

// @Tags ShopAPI
// @Summary Delete JD category
// @Param id path string true "category ID"
// @Success 200 {object} util.ResponseResult
// @Failure 404 {object} util.ResponseResult
// @Failure 500 {object} util.ResponseResult
// @Router /api/v1/shop/categories/{id} [delete]
func (a *Management) DeleteCategory(c *gin.Context) {
	if err := a.Service.DeleteCategory(c.Request.Context(), c.Param("id")); err != nil {
		util.ResError(c, err)
		return
	}
	util.ResOK(c)
}

// @Tags ShopAPI
// @Summary Query JD products
// @Param current query int true "pagination index" default(1)
// @Param pageSize query int true "pagination size" default(10)
// @Param sku query string false "JD SKU"
// @Param name query string false "product name"
// @Param categoryID query string false "category ID"
// @Param selfOperated query bool false "self-operated"
// @Param monitorStatus query string false "active or paused"
// @Param discoveryStatus query string false "active, stale, or capped"
// @Success 200 {object} util.ResponseResult{data=[]schema.JDProduct}
// @Failure 400 {object} util.ResponseResult
// @Failure 500 {object} util.ResponseResult
// @Router /api/v1/shop/products [get]
func (a *Management) QueryProducts(c *gin.Context) {
	var params schema.JDProductQueryParam
	if err := util.ParseQuery(c, &params); err != nil {
		util.ResError(c, err)
		return
	}
	result, err := a.Service.QueryProducts(c.Request.Context(), params)
	if err != nil {
		util.ResError(c, err)
		return
	}
	util.ResPage(c, result.Data, result.PageResult)
}

// @Tags ShopAPI
// @Summary Get JD product details
// @Param id path string true "product ID"
// @Success 200 {object} util.ResponseResult{data=schema.ProductDetail}
// @Failure 404 {object} util.ResponseResult
// @Failure 500 {object} util.ResponseResult
// @Router /api/v1/shop/products/{id} [get]
func (a *Management) GetProduct(c *gin.Context) {
	item, err := a.Service.GetProduct(c.Request.Context(), c.Param("id"))
	if err != nil {
		util.ResError(c, err)
		return
	}
	util.ResSuccess(c, item)
}

// @Tags ShopAPI
// @Summary Pause or resume JD product monitoring
// @Param id path string true "product ID"
// @Param body body schema.JDProductForm true "monitor status"
// @Success 200 {object} util.ResponseResult
// @Failure 400 {object} util.ResponseResult
// @Failure 404 {object} util.ResponseResult
// @Failure 500 {object} util.ResponseResult
// @Router /api/v1/shop/products/{id} [put]
func (a *Management) UpdateProduct(c *gin.Context) {
	form := new(schema.JDProductForm)
	if err := util.ParseJSON(c, form); err != nil {
		util.ResError(c, err)
		return
	}
	if err := a.Service.UpdateProduct(c.Request.Context(), c.Param("id"), form); err != nil {
		util.ResError(c, err)
		return
	}
	util.ResOK(c)
}

// @Tags ShopAPI
// @Summary Query JD product price history
// @Param id path string true "product ID"
// @Param current query int true "pagination index" default(1)
// @Param pageSize query int true "pagination size" default(10)
// @Param type query string false "public or checkout"
// @Param startTime query string false "start time"
// @Param endTime query string false "end time"
// @Success 200 {object} util.ResponseResult{data=[]schema.PriceSample}
// @Failure 400 {object} util.ResponseResult
// @Failure 404 {object} util.ResponseResult
// @Failure 500 {object} util.ResponseResult
// @Router /api/v1/shop/products/{id}/prices [get]
func (a *Management) QueryPrices(c *gin.Context) {
	var params schema.PriceSampleQueryParam
	if err := util.ParseQuery(c, &params); err != nil {
		util.ResError(c, err)
		return
	}
	result, err := a.Service.QueryPriceSamples(c.Request.Context(), c.Param("id"), params)
	if err != nil {
		util.ResError(c, err)
		return
	}
	util.ResPage(c, result.Data, result.PageResult)
}

// @Tags ShopAPI
// @Summary Query price alerts
// @Param current query int true "pagination index" default(1)
// @Param pageSize query int true "pagination size" default(10)
// @Param productID query string false "product ID"
// @Param sendStatus query string false "pending, sent, or failed"
// @Param startTime query string false "start time"
// @Param endTime query string false "end time"
// @Success 200 {object} util.ResponseResult{data=[]schema.PriceAlert}
// @Failure 400 {object} util.ResponseResult
// @Failure 500 {object} util.ResponseResult
// @Router /api/v1/shop/alerts [get]
func (a *Management) QueryAlerts(c *gin.Context) {
	var params schema.PriceAlertQueryParam
	if err := util.ParseQuery(c, &params); err != nil {
		util.ResError(c, err)
		return
	}
	result, err := a.Service.QueryAlerts(c.Request.Context(), params)
	if err != nil {
		util.ResError(c, err)
		return
	}
	util.ResPage(c, result.Data, result.PageResult)
}

// @Tags ShopAPI
// @Summary Query shop job runs
// @Param current query int true "pagination index" default(1)
// @Param pageSize query int true "pagination size" default(10)
// @Param type query string false "job type"
// @Param status query string false "job status"
// @Success 200 {object} util.ResponseResult{data=[]schema.JobRun}
// @Failure 400 {object} util.ResponseResult
// @Failure 500 {object} util.ResponseResult
// @Router /api/v1/shop/jobs [get]
func (a *Management) QueryJobs(c *gin.Context) {
	var params schema.JobRunQueryParam
	if err := util.ParseQuery(c, &params); err != nil {
		util.ResError(c, err)
		return
	}
	result, err := a.Service.QueryJobs(c.Request.Context(), params)
	if err != nil {
		util.ResError(c, err)
		return
	}
	util.ResPage(c, result.Data, result.PageResult)
}

// @Tags ShopAPI
// @Summary Trigger an asynchronous shop job
// @Param type path string true "discover, public-scan, or checkout-sample"
// @Success 200 {object} util.ResponseResult{data=schema.JobRun}
// @Failure 400 {object} util.ResponseResult
// @Failure 500 {object} util.ResponseResult
// @Router /api/v1/shop/jobs/{type}/run [post]
func (a *Management) RunJob(c *gin.Context) {
	job, err := a.Service.TriggerJob(c.Request.Context(), c.Param("type"), schema.JobTriggerManual)
	if err != nil {
		util.ResError(c, err)
		return
	}
	util.ResSuccess(c, job)
}

// @Tags ShopAPI
// @Summary Get JD browser session status
// @Success 200 {object} util.ResponseResult{data=schema.SessionStatus}
// @Failure 500 {object} util.ResponseResult
// @Router /api/v1/shop/session [get]
func (a *Management) SessionStatus(c *gin.Context) {
	status, err := a.Service.SessionStatus(c.Request.Context())
	if err != nil {
		util.ResError(c, err)
		return
	}
	util.ResSuccess(c, status)
}

// @Tags ShopAPI
// @Summary Send a ServerChan test notification
// @Success 200 {object} util.ResponseResult{data=schema.NotificationTestResult}
// @Failure 500 {object} util.ResponseResult
// @Router /api/v1/shop/notifications/test [post]
func (a *Management) TestNotification(c *gin.Context) {
	result, err := a.Service.TestNotification(c.Request.Context())
	if err != nil {
		util.ResError(c, err)
		return
	}
	util.ResSuccess(c, result)
}
