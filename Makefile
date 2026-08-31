.PHONY: start build serve serve-d stop price-compare jd-login wire swagger agent-ui agent-ui-check agent-start

NOW = $(shell date -u '+%Y%m%d%I%M%S')

RELEASE_VERSION = v10.1.0

APP 			= ginadmin
SERVER_BIN  	= ${APP}
GIT_COUNT 		= $(shell git rev-list --all --count)
GIT_HASH        = $(shell git rev-parse --short HEAD)
RELEASE_TAG     = $(RELEASE_VERSION).$(GIT_COUNT).$(GIT_HASH)

CONFIG_DIR       = ./configs
CONFIG_FILES     = dev
STATIC_DIR       = ./build/dist
START_ARGS       = -d $(CONFIG_DIR) -c $(CONFIG_FILES) -s $(STATIC_DIR)

all: start

start:
	@go run -ldflags "-X main.VERSION=$(RELEASE_TAG)" main.go start $(START_ARGS)

build:
	@go build -ldflags "-w -s -X main.VERSION=$(RELEASE_TAG)" -o $(SERVER_BIN)

build-linux:
	CGO_ENABLED=1 GOOS=linux GOARCH=amd64 CC="zig cc -target x86_64-linux-musl" CXX="zig c++ -target x86_64-linux-musl" CGO_CFLAGS="-D_LARGEFILE64_SOURCE" go build -ldflags "-w -s -X main.VERSION=$(RELEASE_TAG)" -o $(SERVER_BIN)_linux_amd64

# Pinned because older Wire releases fail with current Go toolchains.
wire:
	@go run github.com/google/wire/cmd/wire@v0.7.0 gen ./internal/wirex

# go install github.com/swaggo/swag/cmd/swag@latest
swagger:
	@swag init --parseDependency --generalInfo ./main.go --output ./internal/swagger

# https://github.com/OpenAPITools/openapi-generator

clean:
	rm -rf data $(SERVER_BIN)

serve: build
	./$(SERVER_BIN) start $(START_ARGS)

serve-d: build
	./$(SERVER_BIN) start $(START_ARGS) --daemon

stop:
	./$(SERVER_BIN) stop

price-compare:
	@bash scripts/price_compare.sh run

jd-login:
	@bash scripts/price_compare.sh login

agent-ui:
	@npm --prefix web/agent-ui ci
	@npm --prefix web/agent-ui run build

agent-ui-check:
	@npm --prefix web/agent-ui run typecheck
	@npm --prefix web/agent-ui test
	@npm --prefix web/agent-ui run build

agent-start: agent-ui
	@test -n "$$KIMI_API_KEY" || (echo "KIMI_API_KEY is required" >&2; exit 1)
	@test -n "$$AGENT_API_KEY" || (echo "AGENT_API_KEY is required" >&2; exit 1)
	@go run -ldflags "-X main.VERSION=$(RELEASE_TAG)" main.go start $(START_ARGS)
