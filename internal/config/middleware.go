package config

type Middleware struct {
	Recovery struct {
		Skip int `default:"3"` // skip the first n stack frames
	}
	CORS struct {
		Enable                 bool
		AllowAllOrigins        bool
		AllowOrigins           []string
		AllowMethods           []string
		AllowHeaders           []string
		AllowCredentials       bool
		ExposeHeaders          []string
		MaxAge                 int
		AllowWildcard          bool
		AllowBrowserExtensions bool
		AllowWebSockets        bool
		AllowFiles             bool
	}
	Trace struct {
		SkippedPathPrefixes []string
		RequestHeaderKey    string `default:"X-Request-Id"`
		ResponseTraceKey    string `default:"X-Trace-Id"`
	}
	Logger struct {
		SkippedPathPrefixes      []string
		MaxOutputRequestBodyLen  int `default:"4096"`
		MaxOutputResponseBodyLen int `default:"1024"`
	}
	CopyBody struct {
		SkippedPathPrefixes []string
		MaxContentLen       int64 `default:"33554432"` // max content length (default 32MB)
	}
	RateLimiter struct {
		Enable              bool
		SkippedPathPrefixes []string
		Period              int // seconds
		MaxRequestsPerIP    int
		Store               struct {
			Type   string // memory/redis
			Memory struct {
				Expiration      int `default:"3600"` // seconds
				CleanupInterval int `default:"60"`   // seconds
			}
			Redis struct {
				Addr     string
				Username string
				Password string
				DB       int
			}
		}
	}
	Static struct {
		Dir string // Static files directory (From command arguments)
	}
}
