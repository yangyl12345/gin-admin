package test

import (
	"net/http"
	"testing"

	"github.com/LyricTian/gin-admin/v10/internal/mods/shop/schema"
	"github.com/LyricTian/gin-admin/v10/pkg/util"
	"github.com/stretchr/testify/assert"
)

func TestShopStatus(t *testing.T) {
	e := tester(t)

	var status schema.Status
	e.GET(baseAPI + "/shop/status").
		Expect().Status(http.StatusOK).
		JSON().Decode(&util.ResponseResult{Data: &status})

	assert.Equal(t, "shop", status.Name)
	assert.Equal(t, "ready", status.Status)
}
