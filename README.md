# Money-Loves-Me

基于 Go 语言的币安加密货币全自动交易系统。通过接入币安 REST API 和 WebSocket API，实现实时行情获取、多策略自动交易、风险控制、历史回测、策略自我优化，并提供 React Web UI 进行可视化管理。

---

## 目录

- [功能概览](#功能概览)
- [技术栈](#技术栈)
- [项目结构](#项目结构)
- [环境要求](#环境要求)
- [安装部署](#安装部署)
- [配置说明](#配置说明)
- [数据库初始化](#数据库初始化)
- [启动运行](#启动运行)
- [前端构建](#前端构建)
- [API 使用指南](#api-使用指南)
- [策略引擎](#策略引擎)
- [风控管理](#风控管理)
- [回测系统](#回测系统)
- [策略优化器](#策略优化器)
- [通知系统](#通知系统)
- [安全机制](#安全机制)
- [测试](#测试)
- [常见问题](#常见问题)
- [许可证](#许可证)

---

## 功能概览

| 模块 | 功能 |
|------|------|
| 行情服务 | WebSocket 实时推送 K线和订单簿，支持 1m/5m/15m/1h/4h/1d 周期，断线自动降级为 REST 轮询 |
| 策略引擎 | 内置均线交叉、RSI、布林带三种策略，自动评估市场选择策略，手续费感知过滤低收益信号 |
| 订单管理 | 限价单/市价单/止损限价单/止盈限价单，自动验证参数精度，记录完整策略决策依据 |
| 风控管理 | 单笔限额、持仓比例上限、每日亏损上限自动暂停策略、自动止损信号生成 |
| 历史回测 | 手续费计算（Maker/Taker）、滑点模拟、批量参数回测、生成收益曲线和完整报告 |
| 策略优化 | 定时在 ±30% 参数范围搜索最优组合，以净收益率为目标，自动更新或保留当前参数 |
| 通知系统 | 6 种事件类型（订单成交/策略信号/风控告警/API 断线/回测完成/优化完成），支持过滤 |
| 账户服务 | 余额缓存、总资产 USDT 计价、持仓盈亏（含手续费扣除）、累计手续费统计 |
| Web UI | K线图表、订单管理、资产概览、自动交易控制、风控设置、回测结果、优化历史、通知、交易记录 |
| 安全 | AES-256 加密敏感配置、bcrypt 密码哈希、JWT 认证、登录失败锁定 |

---

## 技术栈

| 层级 | 技术 | 说明 |
|------|------|------|
| 语言 | Go 1.23+ | 高并发，适合实时行情处理 |
| Web 框架 | Gin | 高性能 HTTP 框架 |
| ORM | GORM | 支持 MySQL，自动迁移 |
| 日志 | Zap + lumberjack | 结构化 JSON 日志，自动轮转 |
| 配置 | Viper | YAML 配置文件管理 |
| 数据库 | MySQL 8.0+ | 关系型存储，10 张核心表 |
| 前端 | React 18 + TypeScript | SPA 单页应用 |
| 图表 | Lightweight Charts | TradingView 开源 K线图表库 |
| 构建 | Vite | 前端构建工具 |
| 测试 | pgregory.net/rapid | 属性测试（Property-Based Testing） |
| 定时任务 | robfig/cron | 策略优化定时触发 |

---

## 项目结构

```
Money-Loves-Me/
├── cmd/server/main.go           # 程序入口，模块组装和优雅关闭
├── configs/config.yaml          # 配置文件（含敏感信息，已 gitignore）
├── migrations/001_init.sql      # MySQL 建表脚本（10 张表）
├── internal/
│   ├── account/service.go       # 账户服务：余额、盈亏、手续费统计
│   ├── backtest/backtester.go   # 回测引擎：手续费、滑点、批量回测
│   ├── config/
│   │   ├── config.go            # 配置结构体 + Viper 加载 + 验证
│   │   └── crypto.go            # AES-256-GCM 加解密
│   ├── errors/errors.go         # 统一错误类型（7 种错误码）
│   ├── logger/logger.go         # 结构化日志（Zap + 日志轮转）
│   ├── market/service.go        # 行情服务：发布-订阅、缓存、REST 降级
│   ├── model/                   # GORM 数据模型（11 个文件）
│   ├── notification/service.go  # 通知服务：6 种事件、过滤、时间倒序
│   ├── optimizer/optimizer.go   # 策略优化器：参数搜索、回测评估
│   ├── order/
│   │   ├── manager.go           # 订单管理：提交、取消、状态跟踪
│   │   ├── validator.go         # 订单验证：精度、最小量、最小名义值
│   │   └── export.go            # CSV 导出/解析
│   ├── restore/restore.go       # 启动恢复：从 DB 恢复策略和风控配置
│   ├── risk/manager.go          # 风控管理：限额、止损、每日亏损
│   ├── server/
│   │   ├── auth.go              # JWT 认证 + bcrypt + 账户锁定
│   │   ├── router.go            # HTTP 路由（22 个 API 端点）
│   │   ├── handler.go           # HTTP 处理器
│   │   └── websocket.go         # WebSocket Hub（实时推送）
│   ├── store/                   # 数据持久化层（10 个 Store + 重试逻辑）
│   └── strategy/
│       ├── engine.go            # 策略接口 + 信号/参数类型定义
│       ├── strategy_engine.go   # 策略引擎核心：启停、评估、手续费过滤
│       ├── ma_cross.go          # 均线交叉策略
│       ├── rsi.go               # RSI 超买超卖策略
│       └── bollinger.go         # 布林带突破策略
├── pkg/binance/
│   ├── client.go                # 币安客户端：限速、签名
│   ├── rest.go                  # REST API 封装
│   ├── websocket.go             # WebSocket：心跳、重连、重订阅
│   ├── signer.go                # HMAC-SHA256 签名
│   └── types.go                 # 数据类型定义
├── web/                         # React 前端
│   ├── src/
│   │   ├── services/api.ts      # HTTP API 封装（axios）
│   │   ├── services/websocket.ts# WebSocket 客户端
│   │   ├── pages/               # 登录页、仪表盘页
│   │   └── components/          # 9 个功能面板组件
│   ├── package.json
│   └── vite.config.ts
├── Makefile                     # 构建、测试、运行命令
├── go.mod
└── go.sum
```

---

## 环境要求

| 依赖 | 版本 | 说明 |
|------|------|------|
| Go | 1.23+ | 后端编译运行 |
| MySQL | 8.0+ | 数据存储 |
| Node.js | 18+ | 前端构建（可选，仅需构建 Web UI 时） |
| npm | 9+ | 前端包管理 |

---

## 安装部署

### 1. 克隆项目

```bash
git clone <repo-url>
cd Money-Loves-Me
```

### 2. 安装 Go 依赖

```bash
go mod download
```

### 3. 编译后端

```bash
make build
# 生成 bin/server 可执行文件
```

或手动编译：

```bash
go build -o bin/server ./cmd/server
```

---

## 配置说明

配置文件位于 `configs/config.yaml`。首次使用需要从模板创建并填写实际值。

### 配置文件完整示例

```yaml
# 服务器配置
server:
  host: "0.0.0.0"        # 监听地址
  port: 8080              # 监听端口
  mode: "release"         # debug（开发）/ release（生产）/ test

# 币安 API 配置
binance:
  api_key: "加密后的API_KEY"       # AES-256 加密后的值
  secret_key: "加密后的SECRET_KEY"  # AES-256 加密后的值
  base_url: "https://api.binance.com"
  ws_url: "wss://stream.binance.com:9443"

# 数据库配置
database:
  host: "127.0.0.1"
  port: 3306
  user: "trading"
  password: "加密后的密码"   # AES-256 加密后的值
  db_name: "trading_system"

# 日志配置
log:
  level: "INFO"           # DEBUG / INFO / WARN / ERROR
  file_path: "logs/trading.log"
  max_size_mb: 100        # 单个日志文件最大 100MB
  max_age_days: 30        # 保留最近 30 天

# 交易配置
trading:
  default_pairs:          # 默认监控的交易对
    - "BTCUSDT"
    - "ETHUSDT"

# 风控配置
risk:
  max_order_amount: "1000"      # 单笔最大金额（USDT）
  max_daily_loss: "500"         # 每日最大亏损（USDT）
  stop_loss_percent:            # 各交易对止损百分比
    BTCUSDT: "0.05"             # 5%
    ETHUSDT: "0.05"
  max_position_percent:         # 各交易对最大持仓比例
    BTCUSDT: "0.3"              # 30%
    ETHUSDT: "0.3"

# 优化器配置
optimizer:
  interval: "24h"               # 优化周期
  lookback_days: 30             # 回看天数
  max_param_change: 0.3         # 单次最大参数变化幅度（30%）
```

### 敏感信息加密

配置文件中的 `api_key`、`secret_key` 和数据库 `password` 使用 AES-256-GCM 加密存储。加密方式：

```go
import "money-loves-me/internal/config"

// 密钥必须是 32 字节（AES-256）
key := []byte("your-32-byte-encryption-key!!!!") // 恰好 32 字节

// 加密
encrypted, err := config.Encrypt(key, "你的明文API_KEY")

// 解密
plaintext, err := config.Decrypt(key, encrypted)
```

你可以编写一个小工具来生成加密后的值，然后填入配置文件。加密密钥需要通过环境变量或其他安全方式传递给程序。

---

## 数据库初始化

### 1. 创建数据库和用户

```sql
CREATE DATABASE trading_system CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci;
CREATE USER 'trading'@'localhost' IDENTIFIED BY '你的密码';
GRANT ALL PRIVILEGES ON trading_system.* TO 'trading'@'localhost';
FLUSH PRIVILEGES;
```

### 2. 执行建表脚本

```bash
mysql -u trading -p trading_system < migrations/001_init.sql
```

脚本会创建以下 10 张表：

| 表名 | 说明 |
|------|------|
| `users` | 用户账户（用户名、密码哈希、登录失败计数、锁定时间） |
| `strategies` | 策略配置（名称、类型、参数 JSON、是否激活） |
| `orders` | 订单记录（交易对、方向、类型、数量、价格、状态、手续费） |
| `trades` | 交易明细（价格、数量、金额、手续费、策略决策依据 JSON、前后余额） |
| `account_snapshots` | 账户快照（总资产 USDT、各币种余额 JSON） |
| `backtest_results` | 回测结果（收益率、净收益、最大回撤、胜率、权益曲线） |
| `optimization_records` | 优化记录（旧参数、新参数、旧指标、新指标、是否应用） |
| `notifications` | 通知消息（事件类型、标题、描述、已读状态） |
| `risk_configs` | 风控配置（限额、止损百分比、持仓比例） |
| `notification_settings` | 通知设置（用户启用的事件类型） |

### 3. 创建初始用户

系统启动后需要手动在数据库中创建第一个用户：

```sql
-- 密码使用 bcrypt 哈希，以下示例密码为 "admin123"
-- 实际使用时请通过程序生成 bcrypt 哈希
INSERT INTO users (username, password_hash) VALUES ('admin', '$2a$10$...');
```

或者在 Go 代码中生成：

```go
import "money-loves-me/internal/server"

hash, _ := server.HashPassword("你的密码")
// 将 hash 插入数据库
```

---

## 启动运行

### 直接运行

```bash
make run
# 或
go run ./cmd/server
```

### 使用编译后的二进制

```bash
make build
./bin/server
```

### 启动流程

系统启动时会按以下顺序执行：

1. 加载 `configs/config.yaml` 配置文件
2. 验证配置完整性（缺少必填字段会拒绝启动）
3. 初始化结构化日志
4. 连接 MySQL 数据库并执行自动迁移
5. 初始化各模块（Store、Auth、Handler、WebSocket Hub）
6. 启动 HTTP 服务（默认端口 8080）
7. 等待 SIGINT/SIGTERM 信号进行优雅关闭

### 优雅关闭

收到终止信号后，系统会：
- 停止接受新的 HTTP 请求
- 等待进行中的请求完成（最多 10 秒）
- 关闭数据库连接
- 刷新日志缓冲区

---

## 前端构建

### 安装依赖

```bash
make web-install
# 或
cd web && npm install
```

### 开发模式

```bash
cd web && npm run dev
```

开发服务器会自动代理 `/api` 和 `/ws` 请求到后端 `localhost:8080`。

### 生产构建

```bash
make web-build
# 或
cd web && npx tsc --noEmit && npx vite build
```

构建产物输出到 `web/dist/`，可以用 Nginx 或其他静态文件服务器托管。

### Nginx 配置示例

```nginx
server {
    listen 80;
    server_name your-domain.com;

    # 前端静态文件
    location / {
        root /path/to/Money-Loves-Me/web/dist;
        try_files $uri $uri/ /index.html;
    }

    # API 代理
    location /api/ {
        proxy_pass http://127.0.0.1:8080;
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
    }

    # WebSocket 代理
    location /ws {
        proxy_pass http://127.0.0.1:8080;
        proxy_http_version 1.1;
        proxy_set_header Upgrade $http_upgrade;
        proxy_set_header Connection "upgrade";
    }
}
```

---

## API 使用指南

所有 API 端点（除登录外）需要在请求头中携带 JWT Token：

```
Authorization: Bearer <token>
```

### 认证

#### 登录

```bash
curl -X POST http://localhost:8080/api/v1/auth/login \
  -H "Content-Type: application/json" \
  -d '{"username": "admin", "password": "你的密码"}'
```

响应：

```json
{
  "token": "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9...",
  "expires_at": "2026-02-28T18:00:00Z"
}
```

Token 有效期 24 小时。过期后需重新登录。

### 行情数据

```bash
# 获取 K线数据
curl -H "Authorization: Bearer $TOKEN" \
  http://localhost:8080/api/v1/market/klines/BTCUSDT

# 获取订单簿
curl -H "Authorization: Bearer $TOKEN" \
  http://localhost:8080/api/v1/market/orderbook/BTCUSDT
```

### 订单操作

```bash
# 创建限价买单
curl -X POST http://localhost:8080/api/v1/orders \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "symbol": "BTCUSDT",
    "side": "BUY",
    "type": "LIMIT",
    "quantity": "0.001",
    "price": "40000"
  }'

# 取消订单
curl -X DELETE http://localhost:8080/api/v1/orders/12345 \
  -H "Authorization: Bearer $TOKEN"

# 查询订单列表（支持按交易对和时间筛选）
curl -H "Authorization: Bearer $TOKEN" \
  "http://localhost:8080/api/v1/orders?symbol=BTCUSDT"

# 导出交易记录为 CSV
curl -H "Authorization: Bearer $TOKEN" \
  http://localhost:8080/api/v1/orders/export -o trades.csv
```

支持的订单类型：
- `LIMIT` — 限价单
- `MARKET` — 市价单
- `STOP_LOSS_LIMIT` — 止损限价单
- `TAKE_PROFIT_LIMIT` — 止盈限价单

### 账户信息

```bash
# 查看余额
curl -H "Authorization: Bearer $TOKEN" \
  http://localhost:8080/api/v1/account/balances

# 查看盈亏
curl -H "Authorization: Bearer $TOKEN" \
  http://localhost:8080/api/v1/account/pnl

# 查看累计手续费
curl -H "Authorization: Bearer $TOKEN" \
  http://localhost:8080/api/v1/account/fees
```

### 自动交易控制

```bash
# 启动自动交易
curl -X POST http://localhost:8080/api/v1/strategy/start \
  -H "Authorization: Bearer $TOKEN"

# 停止自动交易
curl -X POST http://localhost:8080/api/v1/strategy/stop \
  -H "Authorization: Bearer $TOKEN"

# 查看策略状态
curl -H "Authorization: Bearer $TOKEN" \
  http://localhost:8080/api/v1/strategy/status
```

### 风控配置

```bash
# 查看当前风控配置
curl -H "Authorization: Bearer $TOKEN" \
  http://localhost:8080/api/v1/risk/config

# 更新风控配置
curl -X PUT http://localhost:8080/api/v1/risk/config \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "max_order_amount": "2000",
    "max_daily_loss": "1000",
    "stop_loss_percent": {"BTCUSDT": "0.03"},
    "max_position_percent": {"BTCUSDT": "0.25"}
  }'
```

### 回测

```bash
# 运行回测
curl -X POST http://localhost:8080/api/v1/backtest/run \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "symbol": "BTCUSDT",
    "strategy": "MA_CROSS",
    "start_time": "2025-01-01T00:00:00Z",
    "end_time": "2025-12-31T23:59:59Z",
    "initial_capital": "10000",
    "fee_rate": "0.001",
    "slippage": "0.001"
  }'

# 查看回测结果
curl -H "Authorization: Bearer $TOKEN" \
  http://localhost:8080/api/v1/backtest/results
```

### 通知

```bash
# 查看通知列表
curl -H "Authorization: Bearer $TOKEN" \
  http://localhost:8080/api/v1/notifications

# 标记通知为已读
curl -X PUT http://localhost:8080/api/v1/notifications/1/read \
  -H "Authorization: Bearer $TOKEN"

# 更新通知设置（选择接收哪些事件类型）
curl -X PUT http://localhost:8080/api/v1/notifications/settings \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "enabled_events": ["ORDER_FILLED", "RISK_ALERT", "API_DISCONNECT"]
  }'
```

### WebSocket 实时推送

连接 `ws://localhost:8080/ws` 接收实时数据。消息格式：

```json
{
  "type": "market",          // market / order / notification
  "data": { ... }
}
```

消息类型：
- `market` — 行情更新（K线、价格）
- `order` — 订单状态变更
- `notification` — 通知消息

### 完整 API 端点列表

| 方法 | 路径 | 认证 | 说明 |
|------|------|------|------|
| POST | `/api/v1/auth/login` | 否 | 用户登录，返回 JWT Token |
| GET | `/api/v1/market/klines/:symbol` | 是 | 获取 K线数据 |
| GET | `/api/v1/market/orderbook/:symbol` | 是 | 获取订单簿深度 |
| POST | `/api/v1/orders` | 是 | 创建订单 |
| DELETE | `/api/v1/orders/:id` | 是 | 取消订单 |
| GET | `/api/v1/orders` | 是 | 查询订单列表 |
| GET | `/api/v1/orders/export` | 是 | 导出交易记录 CSV |
| GET | `/api/v1/account/balances` | 是 | 查看账户余额 |
| GET | `/api/v1/account/pnl` | 是 | 查看持仓盈亏 |
| GET | `/api/v1/account/fees` | 是 | 查看累计手续费 |
| POST | `/api/v1/strategy/start` | 是 | 启动自动交易 |
| POST | `/api/v1/strategy/stop` | 是 | 停止自动交易 |
| GET | `/api/v1/strategy/status` | 是 | 查看策略运行状态 |
| GET | `/api/v1/risk/config` | 是 | 获取风控配置 |
| PUT | `/api/v1/risk/config` | 是 | 更新风控配置 |
| POST | `/api/v1/backtest/run` | 是 | 运行回测 |
| GET | `/api/v1/backtest/results` | 是 | 查看回测结果 |
| GET | `/api/v1/optimizer/history` | 是 | 查看优化历史 |
| GET | `/api/v1/notifications` | 是 | 获取通知列表 |
| PUT | `/api/v1/notifications/:id/read` | 是 | 标记通知已读 |
| PUT | `/api/v1/notifications/settings` | 是 | 更新通知设置 |
| GET | `/api/v1/trades` | 是 | 查询交易记录 |
| WS | `/ws` | 否 | WebSocket 实时推送 |

---

## 策略引擎

### 内置策略

系统内置三种交易策略，每种策略自动初始化有效默认参数：

#### 1. 均线交叉策略（MA Cross）

- 短期均线周期：默认 7
- 长期均线周期：默认 25
- 买入信号：短期均线上穿长期均线（金叉）
- 卖出信号：短期均线下穿长期均线（死叉）

#### 2. RSI 超买超卖策略

- RSI 周期：默认 14
- 超买阈值：默认 70
- 超卖阈值：默认 30
- 买入信号：RSI 从超卖区域回升
- 卖出信号：RSI 从超买区域回落

#### 3. 布林带突破策略（Bollinger Bands）

- 周期：默认 20
- 标准差倍数：默认 2.0
- 买入信号：价格触及下轨
- 卖出信号：价格触及上轨

### 手续费感知

策略引擎在生成信号时会自动评估手续费影响：

1. 计算信号的预期收益
2. 使用 Taker 费率估算手续费
3. 仅当 `预期收益 > 预估手续费` 时才生成信号
4. 低收益信号会被过滤，避免手续费侵蚀利润

### 信号记录

每个信号包含完整的决策依据：

```json
{
  "strategy": "MA_CROSS",
  "symbol": "BTCUSDT",
  "side": "BUY",
  "price": "42350.50",
  "quantity": "0.01",
  "expected_profit": "15.30",
  "reason": {
    "indicators": {"MA7": 42350.5, "MA25": 41800.2},
    "trigger_rule": "MA7 上穿 MA25，形成金叉",
    "market_state": "上升趋势"
  }
}
```

---

## 风控管理

### 风控规则

| 规则 | 说明 | 触发动作 |
|------|------|---------|
| 单笔限额 | 订单金额超过 `max_order_amount` | 拒绝订单 |
| 持仓比例 | 某交易对持仓超过 `max_position_percent` | 拒绝订单 |
| 每日亏损 | 当日累计亏损（含手续费）达到 `max_daily_loss` | 暂停所有策略 + 通知用户 |
| 自动止损 | 持仓亏损达到 `stop_loss_percent` | 生成止损卖出信号 |

### 风控检查流程

```
订单提交 → 验证参数精度 → 检查单笔限额 → 检查持仓比例 → 提交到币安
                                                              ↓
                                                         记录订单 + 手续费
                                                              ↓
                                                    检查每日亏损 → 超限则暂停策略
```

### 配置持久化

风控配置保存在 MySQL `risk_configs` 表中。系统重启时自动从数据库恢复上次的风控参数，无需重新配置。

---

## 回测系统

### 回测功能

- 加载指定时间范围的历史 K线数据
- 按策略逻辑模拟交易执行
- 按 Maker/Taker 费率计算每笔手续费
- 模拟滑点影响（买入价格上浮、卖出价格下浮）
- 支持批量参数回测（同一策略不同参数组合）

### 回测报告指标

| 指标 | 计算方式 |
|------|---------|
| 总收益率 | (最终权益 - 初始资金) / 初始资金 |
| 净收益 | 最终权益 - 初始资金（已扣除手续费） |
| 最大回撤 | 权益曲线中从峰值到谷值的最大跌幅百分比 |
| 胜率 | 盈利交易数 / 总卖出交易数 |
| 盈亏比 | 总盈利 / 总亏损 |
| 总交易次数 | 交易明细列表长度 |
| 总手续费 | 所有单笔手续费之和 |

### 滑点模拟

```
买入实际价格 = 信号价格 × (1 + 滑点百分比)
卖出实际价格 = 信号价格 × (1 - 滑点百分比)
```

例如滑点 0.1%（0.001）：信号价格 40000 USDT 的买单实际以 40040 USDT 成交。

---

## 策略优化器

### 优化流程

1. 定时触发（默认每 24 小时）
2. 读取当前策略参数
3. 在当前参数 ±30% 范围内生成 10 组候选参数
4. 对每组候选参数运行回测（含手续费和滑点）
5. 选择净收益率最高的候选参数
6. 决策逻辑：
   - 最优候选净收益为正 **且** 优于当前参数 → 自动更新策略参数
   - 最优候选净收益为负 → 保留当前参数，通知用户
7. 记录优化详细日志到 `optimization_records` 表

### 参数约束

- 每个参数单次变化幅度不超过 30%
- 参数值始终保持为正数
- 优化记录包含旧参数、新参数、旧指标、新指标和分析结论

---

## 通知系统

### 事件类型

| 事件类型 | 常量 | 触发场景 |
|---------|------|---------|
| 订单成交 | `ORDER_FILLED` | 订单在币安完全成交 |
| 策略信号 | `SIGNAL_TRIGGERED` | 策略引擎生成新的交易信号 |
| 风控告警 | `RISK_ALERT` | 每日亏损超限、订单被风控拒绝 |
| API 断线 | `API_DISCONNECT` | 币安 WebSocket 连接断开 |
| 回测完成 | `BACKTEST_COMPLETE` | 回测任务执行完毕 |
| 优化完成 | `OPTIMIZE_COMPLETE` | 策略优化周期执行完毕 |

### 通知特性

- 按创建时间倒序排列
- 支持已读/未读标记
- 支持按事件类型过滤（用户可选择只接收特定类型的通知）
- 通过 WebSocket 实时推送到前端

---

## 安全机制

### AES-256 加密

配置文件中的敏感信息（API Key、Secret Key、数据库密码）使用 AES-256-GCM 模式加密存储。加密特性：

- 256 位密钥（32 字节）
- GCM 认证加密模式，同时保证机密性和完整性
- 每次加密使用随机 Nonce，相同明文产生不同密文
- 密文以 Base64 编码存储

### 密码安全

- 用户密码使用 bcrypt 哈希存储（默认 cost = 10）
- 数据库中不存储明文密码
- 密码验证通过 bcrypt 的恒定时间比较，防止时序攻击

### JWT 认证

- 使用 HMAC-SHA256 签名
- Token 有效期 24 小时
- 所有 API 端点（除登录外）需要携带有效 Token
- Token 过期后前端自动跳转到登录页

### 账户锁定

- 连续 3 次登录失败后，账户锁定 15 分钟
- 锁定期间即使密码正确也拒绝登录
- 成功登录后自动重置失败计数

### HMAC-SHA256 签名

所有发送到币安的认证请求使用 HMAC-SHA256 签名：

- 相同载荷和密钥始终产生相同签名
- 不同载荷产生不同签名
- 签名附加在请求参数中发送

---

## 测试

### 运行全部测试

```bash
make test
```

### 运行详细输出

```bash
make test-verbose
```

### 运行单个包的测试

```bash
go test money-loves-me/internal/strategy -count=1 -v
```

### 属性测试

系统包含 30 个属性测试，使用 `pgregory.net/rapid` 库。每个属性测试运行 100 次随机迭代，验证系统在所有有效输入下的正确性。

| 编号 | 属性 | 测试文件 |
|------|------|---------|
| 1 | AES-256 加密解密往返 | `internal/config/crypto_test.go` |
| 2 | HMAC-SHA256 签名验证 | `pkg/binance/signer_test.go` |
| 3 | 无效凭证返回结构化错误 | `pkg/binance/client_test.go` |
| 4 | 订单参数验证正确性 | `internal/order/validator_test.go` |
| 5 | 数据持久化往返 | `internal/store/persistence_test.go` |
| 6 | 交易记录完整性 | `internal/order/trade_test.go` |
| 7 | 策略默认参数有效性 | `internal/strategy/engine_test.go` |
| 8 | 停止后无新信号 | `internal/strategy/engine_test.go` |
| 9 | 手续费感知信号生成 | `internal/strategy/engine_test.go` |
| 10 | 总资产价值计算 | `internal/account/service_test.go` |
| 11 | 盈亏含手续费计算 | `internal/account/service_test.go` |
| 12 | 时间范围过滤 | `internal/store/query_test.go` |
| 13 | 风控拒绝超限订单 | `internal/risk/manager_test.go` |
| 14 | 每日亏损阈值暂停 | `internal/risk/manager_test.go` |
| 15 | 止损信号触发 | `internal/risk/manager_test.go` |
| 16 | WebSocket 重订阅 | `pkg/binance/websocket_test.go` |
| 17 | 通知排序和过滤 | `internal/notification/service_test.go` |
| 18 | 事件触发通知 | `internal/notification/service_test.go` |
| 19 | 回测手续费计算 | `internal/backtest/backtester_test.go` |
| 20 | 回测报告指标一致性 | `internal/backtest/backtester_test.go` |
| 21 | 回测滑点模拟 | `internal/backtest/backtester_test.go` |
| 22 | 优化器净收益目标 | `internal/optimizer/optimizer_test.go` |
| 23 | 参数变化幅度限制 | `internal/optimizer/optimizer_test.go` |
| 24 | 优化器决策正确性 | `internal/optimizer/optimizer_test.go` |
| 25 | YAML 配置往返 | `internal/config/config_test.go` |
| 26 | 配置验证拒绝无效 | `internal/config/config_test.go` |
| 27 | 账户锁定机制 | `internal/server/auth_test.go` |
| 28 | CSV 导出往返 | `internal/order/export_test.go` |
| 29 | 策略配置启动恢复 | `internal/restore/restore_test.go` |
| 30 | 结构化日志完整性 | `internal/logger/logger_test.go` |

---

## 常见问题

### Q: 配置文件中的加密值怎么生成？

编写一个小工具调用 `config.Encrypt(key, plaintext)` 即可。密钥必须是 32 字节。建议通过环境变量传递密钥，不要硬编码在代码中。

### Q: 系统重启后策略参数会丢失吗？

不会。策略参数和风控配置持久化在 MySQL 中，系统启动时通过 `internal/restore` 模块自动恢复。

### Q: 币安 WebSocket 断线怎么办？

系统自动处理：断线后 3 秒开始重连，使用指数退避策略，最多重试 5 次。重连成功后自动重新订阅所有数据流。重连期间自动降级为 REST API 轮询。

### Q: 每日亏损上限触发后怎么恢复？

当日累计亏损达到上限后，所有策略自动暂停并通知用户。次日亏损计数重置，可以通过 API 重新启动策略。也可以调高 `max_daily_loss` 配置值。

### Q: 回测数据从哪里来？

回测使用币安 REST API 获取历史 K线数据。确保配置了有效的 API Key 并且网络可以访问币安 API。

### Q: 优化器会不会把参数改得很离谱？

不会。每次优化每个参数的变化幅度不超过 30%。如果所有候选参数的回测净收益都是负数，系统会保留当前参数不变。

### Q: 数据库集成测试用的什么？

测试使用 SQLite 内存数据库（`gorm.io/driver/sqlite`），不需要启动 MySQL 容器。所有 Store 层测试和属性测试都可以直接运行。

### Q: 前端开发时怎么连接后端？

`vite.config.ts` 已配置代理，开发模式下 `/api` 和 `/ws` 请求自动转发到 `localhost:8080`。先启动后端 `make run`，再启动前端 `cd web && npm run dev`。

---

## Makefile 命令

| 命令 | 说明 |
|------|------|
| `make build` | 编译后端，输出 `bin/server` |
| `make run` | 直接运行后端 |
| `make test` | 运行全部测试 |
| `make test-verbose` | 运行全部测试（详细输出） |
| `make clean` | 清理构建产物和前端依赖 |
| `make web-install` | 安装前端依赖 |
| `make web-build` | 构建前端生产版本 |
| `make all` | 编译后端 + 构建前端 |

---

## 许可证

MIT
