# [Gin](https://github.com/gin-gonic/gin)-Admin

> 基于 Golang + Gin + GORM 2.0 + Casbin 2.0 + Wire DI 的轻量级、灵活、优雅且功能齐全的 RBAC 脚手架。

## 前端项目

- [基于 Ant Design React 实现的前端项目](https://github.com/gin-admin/gin-admin-frontend)
- [基于 Vue.js 实现的前端项目](https://github.com/gin-admin/gin-admin-vue)

## 安装依赖工具

- [Go](https://golang.org/) 1.19+
- [Wire](github.com/google/wire) `go install github.com/google/wire/cmd/wire@latest`
- [Swag](github.com/swaggo/swag) `go install github.com/swaggo/swag/cmd/swag@latest`
- [GIN-ADMIN-CLI](https://github.com/gin-admin/gin-admin-cli) `go install github.com/gin-admin/gin-admin-cli/v10@latest`

## 快速开始

### 创建新的项目

> 可通过 `gin-admin-cli help new` 查看命令的详细说明

```bash
gin-admin-cli new -d ~/go/src --name testapp --desc 'A test API service based on golang.' --pkg 'github.com/xxx/testapp' --git-url https://gitee.com/lyric/gin-admin.git
```

### 启动服务

> 通过更改 `configs/dev/server.toml` 配置文件中的 `MenuFile = "menu_cn.json"` 可以切换到中文菜单

```bash
cd ~/go/src/testapp

make start
# or
go run main.go start
```

### 编译服务

```bash
make build
# or
go build -ldflags "-w -s -X main.VERSION=v1.0.0" -o testapp
```

### 代码生成

> 可通过 `gin-admin-cli help gen` 查看命令的详细说明

#### 准备配置文件 `dictionary.yaml`

```yaml
- name: Dictionary
  comment: 字典管理
  disable_pagination: true
  fill_gorm_commit: true
  fill_router_prefix: true
  tpl_type: "tree"
  fields:
    - name: Code
      type: string
      comment: Code of dictionary (unique for same parent)
      gorm_tag: "size:32;"
      form:
        binding_tag: "required,max=32"
    - name: Name
      type: string
      comment: Display name of dictionary
      gorm_tag: "size:128;index"
      query:
        name: LikeName
        in_query: true
        form_tag: name
        op: LIKE
      form:
        binding_tag: "required,max=128"
    - name: Description
      type: string
      comment: Details about dictionary
      gorm_tag: "size:1024"
      form: {}
    - name: Sequence
      type: int
      comment: Sequence for sorting
      gorm_tag: "index;"
      order: DESC
      form: {}
    - name: Status
      type: string
      comment: Status of dictionary (disabled, enabled)
      gorm_tag: "size:20;index"
      query: {}
      form:
        binding_tag: "required,oneof=disabled enabled"
```

```bash
gin-admin-cli gen -d . -m SYS -c dictionary.yaml
```

### 删除模块

> 可通过 `gin-admin-cli help remove` 查看命令的详细说明

```bash
gin-admin-cli rm -d . -m CMS --structs Article
```

### 生成 Swagger 文档

> 通过 [Swag](github.com/swaggo/swag) 可以自动生成 Swagger 文档

```bash
make swagger
# or
swag init --parseDependency --generalInfo ./main.go --output ./internal/swagger
```

### 生成依赖注入代码

> 依赖注入本身的作用是解决了各个模块间层级依赖繁琐的初始化过程，通过 [Wire](github.com/google/wire) 可以自动生成依赖注入代码，简化依赖注入的过程。

```bash
make wire
# or
wire gen ./internal/wirex
```

## 项目结构概览

```text
├── cmd                             (命令行定义目录)
│   ├── start.go                    (启动命令)
│   ├── stop.go                     (停止命令)
│   └── version.go                  (版本命令)
├── configs
│   ├── dev
│   │   ├── logging.toml            (日志配置文件)
│   │   ├── middleware.toml         (中间件配置文件)
│   │   └── server.toml             (服务配置文件)
│   ├── menu.json                   (初始化菜单文件)
│   └── rbac_model.conf             (Casbin RBAC 模型配置文件)
├── internal
│   ├── bootstrap                   (初始化目录)
│   │   ├── bootstrap.go            (初始化)
│   │   ├── http.go                 (HTTP 服务)
│   │   └── logger.go               (日志服务)
│   ├── config                      (配置文件目录)
│   │   ├── config.go               (配置文件初始化)
│   │   ├── consts.go               (常量定义)
│   │   ├── middleware.go           (中间件配置)
│   │   └── parse.go                (配置文件解析)
│   ├── mods
│   │   ├── rbac                    (RBAC 模块)
│   │   │   ├── api                 (API层)
│   │   │   ├── biz                 (业务逻辑层)
│   │   │   ├── dal                 (数据访问层)
│   │   │   ├── schema              (数据模型层)
│   │   │   ├── casbin.go           (Casbin 初始化)
│   │   │   ├── main.go             (RBAC 模块入口)
│   │   │   └── wire.go             (RBAC 依赖注入初始化)
│   │   └── mods.go
│   ├── utility
│   │   └── prom
│   │       └── prom.go             (Prometheus 监控，用于集成 prometheus)
│   └── wirex                       (依赖注入目录，包含了依赖组的定义和初始化)
│       ├── injector.go
│       ├── wire.go
│       └── wire_gen.go
├── pkg                             (公共包目录)
│   ├── cachex                      (缓存包)
│   ├── crypto                      (加密包)
│   │   ├── aes                     (AES加密)
│   │   ├── hash                    (哈希加密)
│   │   └── rand                    (随机数)
│   ├── encoding                    (编码包)
│   │   ├── json                    (JSON编码)
│   │   ├── toml                    (TOML编码)
│   │   └── yaml                    (YAML编码)
│   ├── errors                      (错误处理包)
│   ├── gormx                       (Gorm扩展包)
│   ├── jwtx                        (JWT包)
│   ├── logging                     (日志包)
│   ├── mail                        (邮件包)
│   ├── middleware                  (中间件包)
│   ├── oss                         (对象存储包)
│   ├── promx                       (Prometheus包)
│   └── util                        (工具包)
├── test                            (单元测试目录)
│   ├── menu_test.go
│   ├── role_test.go
│   ├── test.go
│   └── user_test.go
├── Dockerfile
├── Makefile
├── README.md
├── go.mod
├── go.sum
└── main.go                         (入口文件)
```
