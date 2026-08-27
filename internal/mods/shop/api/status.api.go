package api

import (
	"github.com/LyricTian/gin-admin/v10/internal/mods/shop/biz"
	"github.com/LyricTian/gin-admin/v10/internal/mods/shop/schema"
	"github.com/LyricTian/gin-admin/v10/pkg/util"
	"github.com/gin-gonic/gin"
)

// Status exposes the shop module status.
type Status struct {
	StatusBIZ *biz.Status
}

// StatusResponse keeps the public schema reference explicit for Go and Swag.
type StatusResponse = schema.Status

// @Tags ShopAPI
// @Summary Get shop module status
// @Success 200 {object} util.ResponseResult{data=schema.Status}
// @Router /api/v1/shop/status [get]
func (a *Status) Get(c *gin.Context) {
	data, err := a.StatusBIZ.Get(c.Request.Context())
	if err != nil {
		util.ResError(c, err)
		return
	}
	util.ResSuccess(c, data)
}
