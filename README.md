# 京东自营价格监控服务

这是一个基于 Go、Gin、GORM、Wire、Chromedp 和 Cron 的个人京东自营商品价格监控后端。

服务只保留 `/api/v1/shop` 领域，不包含用户、角色、菜单、登录、JWT、验证码或 Casbin。所有 Shop API 均不需要认证，请只在可信内网或本机使用，不要直接暴露到公网。

## 环境要求

- Go 1.19+
- MySQL 或 PostgreSQL
- Google Chrome（macOS 默认路径为 `/Applications/Google Chrome.app/Contents/MacOS/Google Chrome`）
- 可选：Server酱 SendKey，用于发送微信通知

## 首次运行

先配置 `configs/dev/server.toml` 中的数据库连接。服务停止时，使用独立 Chrome Profile 人工登录京东：

```bash
go run main.go jd-login -d configs -c dev
```

程序不会读取或保存账号密码、短信码、验证码或二维码内容，也不会绕过京东验证。

如果需要微信通知，在启动服务的同一个终端设置：

```bash
export SERVERCHAN_SEND_KEY="你的 SendKey"
make start
```

SendKey 只从环境变量读取，不写入 TOML、数据库、Swagger 或日志。缺少 SendKey 不影响价格采集，待发送告警会保留。

Swagger 默认地址：

```text
http://127.0.0.1:8040/swagger/index.html
```

健康检查：

```text
GET /health
```

## 一键价格对比

首次使用时，先在服务停止状态下打开独立 Chrome，并由本人依次完成京东移动端与桌面搜索登录：

```bash
make jd-login
```

然后提供一个真实的京东分类或搜索列表 HTTPS 地址，一键创建分类、发现商品、采集公开价与结算价，并等待异步任务完成：

```bash
CATEGORY_URL='https://list.jd.com/list.html?...' make price-compare
```

分类只需创建一次。后续执行时可以直接运行：

```bash
make price-compare
```

脚本会在服务未运行时执行 `make serve-d`，相同分类 URL 已存在时不会重复创建，最终在终端输出公开价、结算价、30 日结算均价和价差，并把完整结果写入已被 Git 忽略的 `data/price-comparison.json`。

只需要公开价、不需要京东登录时，可以运行：

```bash
PUBLIC_ONLY=1 CATEGORY_URL='https://list.jd.com/list.html?...' make price-compare
```

`CATEGORY_URL` 推荐传入纯 `https://...` 地址；如果误粘贴成 `[标题](https://...)` 或 `<https://...>`，脚本也会自动提取真实地址。可通过 `CATEGORY_NAME`、`MAX_PAGES`、`COMPARE_LIMIT`、`JOB_TIMEOUT`、`OUTPUT_FILE` 和 `BASE_URL` 调整分类名称、发现页数、输出数量、等待时间、结果文件和服务地址。运行 `bash scripts/price_compare.sh help` 查看完整说明。账号、密码、短信码、二维码和验证码仍必须由本人在 Chrome 中处理，脚本不会读取、保存或绕过这些验证信息。

## Shop API

所有接口均位于 `/api/v1/shop`，不需要 `Authorization` 请求头。

设置：

- `GET /settings`
- `PUT /settings`

分类：

- `GET /categories`
- `GET /categories/:id`
- `POST /categories`
- `PUT /categories/:id`
- `DELETE /categories/:id`

商品、价格与告警：

- `GET /products`
- `GET /products/:id`
- `PUT /products/:id`
- `GET /products/:id/prices`
- `GET /alerts`

任务、浏览器会话与通知：

- `GET /jobs`
- `POST /jobs/:type/run`
- `GET /session`
- `POST /notifications/test`
- `GET /status`

手工任务类型只允许 `discover`、`public-scan` 和 `checkout-sample`。

## 默认调度

- 分类发现：每 6 小时
- 公开移动价扫描：每 15 分钟
- 结算采样队列：每分钟
- 待发送告警：每 5 分钟
- 数据清理：每天凌晨 3 点

Chrome 会话保存在已被 Git 忽略的 `data/jd-profile`。请勿把该目录复制进仓库或交给他人。结算预览固定数量为 1，使用账号默认地址，只读取订单确认页价格，不点击“提交订单”或支付入口。

## 开发命令

```bash
make wire
make swagger
go test ./internal/mods/shop/...
go test ./test -run 'TestShop|TestRemovedAdminRoutes'
go build ./...
```

## 目录结构

```text
cmd/                         启动、停止、版本和 jd-login 命令
configs/dev/                 运行配置
internal/bootstrap/          HTTP 与服务生命周期
internal/config/             配置模型与加载
internal/mods/shop/api/      Shop HTTP API
internal/mods/shop/biz/      调度、价格和告警业务逻辑
internal/mods/shop/dal/      GORM 数据访问
internal/mods/shop/jd/       京东 Chrome 适配器
internal/mods/shop/notify/   Server酱通知器
internal/mods/shop/schema/   Shop 数据模型和请求/响应结构
internal/wirex/              Wire 依赖注入
test/                        Shop 集成测试
```

代码删除不会自动删除已有数据库中的旧用户、角色或菜单表。如需清理旧表，请先备份数据库并单独执行经过确认的数据库迁移。
