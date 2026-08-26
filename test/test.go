package test

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"testing"

	"github.com/LyricTian/gin-admin/v10/internal/config"
	"github.com/LyricTian/gin-admin/v10/internal/wirex"
	"github.com/gavv/httpexpect/v2"
	"github.com/gin-gonic/gin"
)

const (
	baseAPI = "/api/v1"
)

var (
	app *gin.Engine
)

func init() {
	config.MustLoad("")

	// Tests must be self-contained: RBAC loads Casbin during Init and queries
	// the role table, so the schema has to exist before that step.
	config.C.Storage.DB.Type = "sqlite3"
	config.C.Storage.DB.DSN = filepath.Join(os.TempDir(), fmt.Sprintf("gin-admin-test-%d.db", os.Getpid()))
	config.C.Storage.DB.AutoMigrate = true
	_ = os.Remove(config.C.Storage.DB.DSN)

	ctx := context.Background()
	injector, _, err := wirex.BuildInjector(ctx)
	if err != nil {
		panic(err)
	}

	if err := injector.M.Init(ctx); err != nil {
		panic(err)
	}

	app = gin.New()
	err = injector.M.RegisterRouters(ctx, app)
	if err != nil {
		panic(err)
	}
}

func tester(t *testing.T) *httpexpect.Expect {
	return httpexpect.WithConfig(httpexpect.Config{
		Client: &http.Client{
			Transport: httpexpect.NewBinder(app),
			Jar:       httpexpect.NewCookieJar(),
		},
		Reporter: httpexpect.NewAssertReporter(t),
		Printers: []httpexpect.Printer{
			httpexpect.NewDebugPrinter(t, true),
		},
	})
}
