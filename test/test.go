package test

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/LyricTian/gin-admin/v10/internal/config"
	"github.com/LyricTian/gin-admin/v10/internal/wirex"
	"github.com/gavv/httpexpect/v2"
	"github.com/gin-gonic/gin"
	sdmysql "github.com/go-sql-driver/mysql"
)

const (
	baseAPI = "/api/v1"
)

var (
	app         *gin.Engine
	appInjector *wirex.Injector
)

func init() {
	_, sourceFile, _, ok := runtime.Caller(0)
	if !ok {
		panic("failed to locate test source directory")
	}
	repoRoot := filepath.Dir(filepath.Dir(sourceFile))
	config.MustLoad(filepath.Join(repoRoot, "configs"), "dev")

	config.C.Storage.DB.Type = "mysql"
	config.C.Storage.DB.DSN = testMySQLDSN()
	config.C.Storage.DB.AutoMigrate = true
	config.C.General.WorkDir = repoRoot
	config.C.Shop.Enable = false

	ctx := context.Background()
	injector, _, err := wirex.BuildInjector(ctx)
	if err != nil {
		panic(err)
	}

	if err := injector.M.Init(ctx); err != nil {
		panic(err)
	}
	appInjector = injector

	app = gin.New()
	err = injector.M.RegisterRouters(ctx, app)
	if err != nil {
		panic(err)
	}
}

func testMySQLDSN() string {
	if dsn := os.Getenv("GIN_ADMIN_TEST_MYSQL_DSN"); dsn != "" {
		return dsn
	}

	dsnConfig, err := sdmysql.ParseDSN(config.C.Storage.DB.DSN)
	if err != nil {
		panic(err)
	}
	if dsnConfig.DBName == "" {
		panic("test MySQL DSN must specify a database name")
	}
	dsnConfig.DBName += fmt.Sprintf("_test_%d", os.Getpid())
	return dsnConfig.FormatDSN()
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
