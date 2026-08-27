package config

import (
	"fmt"

	"github.com/LyricTian/gin-admin/v10/pkg/encoding/json"
	"github.com/LyricTian/gin-admin/v10/pkg/logging"
)

type Config struct {
	Logger     logging.LoggerConfig
	General    General
	Storage    Storage
	Middleware Middleware
	Util       Util
	Shop       Shop
}

type General struct {
	AppName            string `default:"ginadmin"`
	Version            string `default:"v10.1.0"`
	Debug              bool
	PprofAddr          string
	DisableSwagger     bool
	DisablePrintConfig bool
	WorkDir            string // From command arguments
	HTTP               struct {
		Addr            string `default:":8040"`
		ShutdownTimeout int    `default:"10"` // seconds
		ReadTimeout     int    `default:"60"` // seconds
		WriteTimeout    int    `default:"60"` // seconds
		IdleTimeout     int    `default:"10"` // seconds
		CertFile        string
		KeyFile         string
	}
}

type Storage struct {
	DB struct {
		Debug        bool
		Type         string `default:"mysql"` // mysql/postgres
		DSN          string // database source name
		MaxLifetime  int    `default:"86400"` // seconds
		MaxIdleTime  int    `default:"3600"`  // seconds
		MaxOpenConns int    `default:"100"`   // connections
		MaxIdleConns int    `default:"50"`    // connections
		TablePrefix  string `default:""`
		AutoMigrate  bool
		PrepareStmt  bool
		Resolver     []struct {
			DBType   string   // mysql/postgres
			Sources  []string // DSN
			Replicas []string // DSN
			Tables   []string
		}
	}
}

type Util struct {
	Prometheus struct {
		Enable         bool
		Port           int `default:"9100"`
		LogApis        []string
		LogMethods     []string
		DefaultCollect bool
	}
}

// Shop contains non-secret runtime settings for JD price monitoring.
// SERVERCHAN_SEND_KEY is intentionally read from the environment instead of
// this structure because Config.Print prints the loaded configuration.
type Shop struct {
	Enable bool `default:"true"`
	JD     struct {
		ChromeExecutable  string `default:"/Applications/Google Chrome.app/Contents/MacOS/Google Chrome"`
		UserDataDir       string `default:"data/jd-profile"`
		Headless          bool   `default:"true"`
		DebugArtifacts    bool
		MaxProducts       int `default:"1000"`
		PublicWorkers     int `default:"4"`
		CheckoutBatch     int `default:"20"`
		PublicDelayMin    int `default:"1"`  // seconds
		PublicDelayMax    int `default:"3"`  // seconds
		CheckoutDelayMin  int `default:"10"` // seconds
		CheckoutDelayMax  int `default:"30"` // seconds
		NavigationTimeout int `default:"45"` // seconds
	}
	Scheduler struct {
		DiscoverSpec      string `default:"0 */6 * * *"`
		PublicScanSpec    string `default:"*/15 * * * *"`
		CheckoutSpec      string `default:"@every 1m"`
		CleanupSpec       string `default:"0 3 * * *"`
		AlertDeliverySpec string `default:"@every 5m"`
	}
	ServerChan struct {
		BaseURL        string `default:"https://sctapi.ftqq.com"`
		RequestTimeout int    `default:"10"` // seconds
	}
}

func (c *Config) IsDebug() bool {
	return c.General.Debug
}

func (c *Config) String() string {
	b, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		panic("Failed to marshal config: " + err.Error())
	}
	return string(b)
}

func (c *Config) Print() {
	if c.General.DisablePrintConfig {
		return
	}
	fmt.Println("// ----------------------- Load configurations start ------------------------")
	fmt.Println(c.String())
	fmt.Println("// ----------------------- Load configurations end --------------------------")
}

func (c *Config) FormatTableName(name string) string {
	return c.Storage.DB.TablePrefix + name
}
