# 实施计划：币安加密货币交易系统

## 概述

基于设计文档，将系统按模块逐步实现。从基础设施层（配置管理、日志、API 客户端）开始，逐步构建数据服务层（行情、账户、通知）、核心业务层（策略引擎、订单管理、风控、回测、优化器），最后实现 Web 层（HTTP/WebSocket 服务、React 前端）。每个模块实现后紧跟对应的属性测试和单元测试，确保增量验证。

## Tasks

- [x] 1. 项目初始化与基础设施搭建
  - [x] 1.1 初始化 Go 项目结构和依赖
    - 创建 `go.mod`，引入核心依赖：`gin`、`gorilla/websocket`、`gorm`、`mysql driver`、`viper`、`zap`、`shopspring/decimal`、`pgregory.net/rapid`、`robfig/cron`
    - 按设计文档创建目录结构：`cmd/server/`、`internal/`（各子模块）、`pkg/binance/`、`web/`、`configs/`、`migrations/`
    - _需求: 12.1_

  - [x] 1.2 实现 Config Manager（配置管理模块）
    - 创建 `internal/config/config.go`，定义 `Config` 结构体及所有子配置（`ServerConfig`、`BinanceConfig`、`DatabaseConfig`、`LogConfig`、`TradingConfig`、`RiskConfig`、`OptimizerConfig`）
    - 使用 Viper 加载 YAML 配置文件，实现配置验证逻辑（必填字段检查、类型校验）
    - 创建 `configs/config.yaml` 默认配置模板
    - 启动时验证配置完整性，无效配置拒绝启动并输出错误信息
    - _需求: 12.1, 12.6_

  - [x] 1.3 实现 AES-256 加解密模块
    - 创建 `internal/config/crypto.go`，实现 `Encrypt(key, plaintext)` 和 `Decrypt(key, ciphertext)` 函数
    - 使用 Go `crypto/aes` + GCM 模式实现 AES-256 加密
    - 对配置文件中的 API Key、Secret Key、数据库密码进行加密存储和解密读取
    - _需求: 1.4, 12.2_

  - [x] 1.4 编写 Property 1 属性测试：AES-256 加密解密往返
    - **Property 1: 对于任意字符串，AES-256 加密后再解密应得到原始值，且密文不包含明文**
    - 在 `internal/config/crypto_test.go` 中使用 `rapid` 库实现
    - **验证: 需求 1.4, 12.2**

  - [x] 1.5 编写 Property 25 属性测试：YAML 配置解析往返
    - **Property 25: 对于任意有效配置结构体，序列化为 YAML 后再反序列化应得到等价结构体**
    - 在 `internal/config/config_test.go` 中使用 `rapid` 库实现
    - **验证: 需求 12.1**

  - [x] 1.6 编写 Property 26 属性测试：配置验证拒绝无效配置
    - **Property 26: 对于任意缺少必填字段或类型错误的配置，验证器应返回错误并拒绝加载**
    - 在 `internal/config/config_test.go` 中使用 `rapid` 库实现
    - **验证: 需求 12.6**

  - [x] 1.7 实现结构化日志模块
    - 创建 `internal/logger/logger.go`，基于 Zap 实现结构化日志
    - 每条日志包含时间戳、模块名称、日志级别（DEBUG/INFO/WARN/ERROR）和日志内容
    - 实现日志轮转：单文件上限 100MB，保留最近 30 天
    - _需求: 9.3, 9.6_

  - [x] 1.8 编写 Property 30 属性测试：结构化日志完整性
    - **Property 30: 对于任意日志条目，必须包含时间戳、模块名称、日志级别和日志内容，且均不为空**
    - 在 `internal/logger/logger_test.go` 中使用 `rapid` 库实现
    - **验证: 需求 9.3**

  - [x] 1.9 实现统一错误类型
    - 创建 `internal/errors/errors.go`，定义 `AppError` 结构体和错误码常量（`ErrNetwork`、`ErrAuth`、`ErrValidation`、`ErrRiskControl`、`ErrDatabase`、`ErrConfig`、`ErrBinanceAPI`）
    - _需求: 1.2, 3.7_

- [x] 2. 检查点 - 确保基础设施模块测试通过
  - 确保所有测试通过，如有问题请询问用户。

- [x] 3. 数据库与数据模型
  - [x] 3.1 创建 MySQL 数据库迁移脚本
    - 在 `migrations/` 目录下创建建表 SQL 脚本
    - 包含所有表：`orders`、`trades`、`strategies`、`account_snapshots`、`backtest_results`、`optimization_records`、`notifications`、`risk_configs`、`users`、`notification_settings`
    - 定义索引、外键和字段约束
    - _需求: 9.1_

  - [x] 3.2 实现 GORM 数据模型
    - 创建各模型文件：`internal/model/order.go`、`trade.go`、`strategy.go`、`account_snapshot.go`、`backtest_result.go`、`optimization_record.go`、`notification.go`、`risk_config.go`、`user.go`
    - 定义 GORM 标签、JSON 字段映射
    - 实现数据库初始化和自动迁移逻辑
    - _需求: 9.1, 9.2_

  - [x] 3.3 实现数据持久化存储层
    - 创建 `internal/store/` 目录，实现各模型的 CRUD 操作
    - 实现通用的时间范围查询和条件过滤方法
    - 实现数据库连接管理和重试逻辑（失败重试 3 次）
    - _需求: 9.1, 3.6, 10.6_

  - [x] 3.4 编写 Property 5 属性测试：数据持久化往返
    - **Property 5: 对于任意交易记录、订单记录、回测结果或优化记录，写入 MySQL 后再读取应得到等价记录**
    - 在 `internal/store/persistence_test.go` 中使用 `rapid` 库和 `testcontainers-go` 实现
    - **验证: 需求 9.1, 3.6, 10.6**

  - [x] 3.5 编写 Property 12 属性测试：时间范围和条件过滤正确性
    - **Property 12: 对于任意时间范围和筛选条件，返回的记录时间戳必须在范围内且匹配条件**
    - 在 `internal/store/query_test.go` 中使用 `rapid` 库实现
    - **验证: 需求 5.5, 7.5, 9.7, 10.7**

- [x] 4. 检查点 - 确保数据层测试通过
  - 确保所有测试通过，如有问题请询问用户。

- [x] 5. API Client（币安 API 客户端）
  - [x] 5.1 实现 HMAC-SHA256 签名模块
    - 创建 `pkg/binance/signer.go`，实现 `HMACSigner` 结构体和 `Sign(payload string) string` 方法
    - 使用 Go `crypto/hmac` + `crypto/sha256` 实现
    - _需求: 1.3_

  - [x] 5.2 编写 Property 2 属性测试：HMAC-SHA256 签名验证
    - **Property 2: 对于任意载荷和密钥，签名应通过标准验证；相同输入产生相同签名，不同输入产生不同签名**
    - 在 `pkg/binance/signer_test.go` 中使用 `rapid` 库实现
    - **验证: 需求 1.3**

  - [x] 5.3 实现币安 REST API 客户端
    - 创建 `pkg/binance/client.go` 和 `pkg/binance/rest.go`
    - 实现 `BinanceClient` 结构体，包含 API Key、Secret Key、HTTP 客户端和签名器
    - 实现 REST API 方法：`GetKlines`、`CreateOrder`、`CancelOrder`、`GetAccountInfo`、`GetExchangeInfo`、`GetOrderBook`
    - 所有认证请求使用 HMAC-SHA256 签名
    - 实现请求限速控制（1200 次/分钟）
    - _需求: 1.1, 1.3_

  - [x] 5.4 实现币安 WebSocket 客户端
    - 创建 `pkg/binance/websocket.go`
    - 实现 WebSocket 连接管理：心跳（30 秒 ping）、超时检测（60 秒）
    - 实现订阅方法：`SubscribeKline`、`SubscribeOrderBook`、`SubscribeUserData`
    - 实现自动重连机制：断线后 3 秒内重试，指数退避，最多 5 次
    - 重连后自动重新订阅所有数据流
    - _需求: 1.5, 1.6, 2.1, 2.6_

  - [x] 5.5 编写 Property 3 属性测试：无效凭证返回结构化错误
    - **Property 3: 对于任意无效 API Key 或 Secret Key，认证请求应返回包含错误码和描述的错误，不返回成功**
    - 在 `pkg/binance/client_test.go` 中使用 `rapid` 库和 `httptest` 模拟实现
    - **验证: 需求 1.2**

  - [x] 5.6 编写 Property 16 属性测试：WebSocket 断线重订阅
    - **Property 16: 对于任意已订阅的数据流集合，断线重连后应自动重新订阅所有数据流**
    - 在 `internal/market/service_test.go` 中使用 `rapid` 库实现
    - **验证: 需求 2.6**

- [x] 6. Market Data Service（行情服务）
  - [x] 6.1 实现行情数据服务
    - 创建 `internal/market/service.go`
    - 实现 `MarketDataService` 结构体，包含订阅者管理、K线缓存、订单簿缓存
    - 实现 `DataConsumer` 接口和发布-订阅模式
    - 实现方法：`Subscribe`、`Unsubscribe`、`GetHistoricalKlines`、`GetCurrentPrice`、`GetOrderBook`
    - 支持 K线周期：1m、5m、15m、1h、4h、1d
    - 行情数据分发延迟不超过 100 毫秒
    - WebSocket 断线时降级为 REST API 轮询
    - _需求: 2.1, 2.2, 2.3, 2.4, 2.5, 2.6_

- [x] 7. 检查点 - 确保 API 客户端和行情服务测试通过
  - 确保所有测试通过，如有问题请询问用户。

- [x] 8. Account Service（账户服务）
  - [x] 8.1 实现账户服务
    - 创建 `internal/account/service.go`
    - 实现 `AccountService` 结构体，包含余额缓存和定时刷新
    - 实现方法：`GetBalances`（可用余额+冻结余额）、`GetTotalAssetValue`（USDT 计价）、`GetPositionPnL`（含手续费扣除）、`GetAssetHistory`、`GetFeeStats`（累计手续费统计）
    - 余额变化后 5 秒内更新本地缓存
    - _需求: 5.1, 5.2, 5.3, 5.4, 5.5, 5.6_

  - [x] 8.2 编写 Property 10 属性测试：总资产价值计算正确性
    - **Property 10: 对于任意余额集合和 USDT 价格，总资产价值应等于各币种余额乘以价格的总和**
    - 在 `internal/account/service_test.go` 中使用 `rapid` 库实现
    - **验证: 需求 5.3**

  - [x] 8.3 编写 Property 11 属性测试：盈亏计算包含手续费
    - **Property 11: 对于任意买卖交易记录，盈亏应等于（卖出总额 - 买入总额 - 总手续费）；累计手续费等于各笔手续费之和**
    - 在 `internal/account/service_test.go` 中使用 `rapid` 库实现
    - **验证: 需求 5.4, 5.6, 6.8**

- [x] 9. Notification Service（通知服务）
  - [x] 9.1 实现通知服务
    - 创建 `internal/notification/service.go`
    - 实现 `NotificationService` 结构体，包含数据库存储、WebSocket 推送和事件过滤
    - 定义事件类型常量：`ORDER_FILLED`、`SIGNAL_TRIGGERED`、`RISK_ALERT`、`API_DISCONNECT`、`BACKTEST_COMPLETE`、`OPTIMIZE_COMPLETE`
    - 实现方法：`Send`、`GetNotifications`（时间倒序）、`MarkAsRead`、`SetEventFilter`
    - 每条通知包含时间戳、事件类型和详细描述
    - _需求: 8.1, 8.2, 8.3, 8.4, 8.5_

  - [x] 9.2 编写 Property 17 属性测试：通知时间倒序和事件过滤
    - **Property 17: 通知列表按时间严格倒序；启用事件过滤后仅返回匹配类型的通知**
    - 在 `internal/notification/service_test.go` 中使用 `rapid` 库实现
    - **验证: 需求 8.4, 8.5**

  - [x] 9.3 编写 Property 18 属性测试：事件触发通知生成
    - **Property 18: 对于任意指定事件类型，事件发生时应生成包含时间戳、事件类型和描述的通知**
    - 在 `internal/notification/service_test.go` 中使用 `rapid` 库实现
    - **验证: 需求 8.1, 8.3**

- [x] 10. 检查点 - 确保数据服务层测试通过
  - 确保所有测试通过，如有问题请询问用户。

- [x] 11. Risk Manager（风控管理）
  - [x] 11.1 实现风控管理模块
    - 创建 `internal/risk/manager.go`
    - 实现 `RiskManager` 结构体和 `RiskConfig`（单笔最大金额、每日最大亏损、止损百分比、最大持仓比例）
    - 实现 `CheckOrder`：验证单笔金额上限和持仓比例上限
    - 实现 `CheckDailyLoss`：计算当日累计亏损（含手续费），达到上限时暂停所有策略
    - 实现止损信号生成：持仓亏损达到止损百分比时自动生成止损卖出信号
    - 实现 `PauseAllStrategies`：暂停所有运行中策略并通知用户
    - 风控配置持久化到 MySQL `risk_configs` 表
    - _需求: 6.1, 6.2, 6.3, 6.4, 6.5, 6.6, 6.7, 6.8_

  - [x] 11.2 编写 Property 13 属性测试：风控拒绝超限订单
    - **Property 13: 对于任意订单，金额超过单笔上限或导致持仓比例超限时，应被拒绝**
    - 在 `internal/risk/manager_test.go` 中使用 `rapid` 库实现
    - **验证: 需求 6.3, 6.7**

  - [x] 11.3 编写 Property 14 属性测试：每日亏损阈值触发策略暂停
    - **Property 14: 对于任意交易序列，当日累计亏损（含手续费）达到上限时应暂停所有策略**
    - 在 `internal/risk/manager_test.go` 中使用 `rapid` 库实现
    - **验证: 需求 6.4**

  - [x] 11.4 编写 Property 15 属性测试：止损信号在阈值触发
    - **Property 15: 对于任意持仓和止损百分比，亏损达到阈值时应生成止损卖出信号**
    - 在 `internal/risk/manager_test.go` 中使用 `rapid` 库实现
    - **验证: 需求 6.5**

- [x] 12. Order Manager（订单管理）
  - [x] 12.1 实现订单验证器
    - 创建 `internal/order/validator.go`
    - 实现订单参数验证：交易对存在性、数量精度和最小交易量、价格精度
    - 从 `ExchangeInfo` 获取交易规则进行校验
    - _需求: 3.2_

  - [x] 12.2 编写 Property 4 属性测试：订单参数验证正确性
    - **Property 4: 对于任意订单，参数不合规时应拒绝并返回验证错误；参数合规时应通过**
    - 在 `internal/order/validator_test.go` 中使用 `rapid` 库实现
    - **验证: 需求 3.2**

  - [x] 12.3 实现订单管理器
    - 创建 `internal/order/manager.go`
    - 实现 `OrderManager` 结构体
    - 实现 `SubmitOrder`：验证参数 → 风控检查 → 提交到币安 → 记录订单（含策略决策依据）→ 失败时通知用户
    - 支持订单类型：限价单、市价单、止损限价单、止盈限价单
    - 实现 `CancelOrder`：发送取消请求并更新本地状态
    - 实现 `GetActiveOrders`、`GetOrderHistory`（支持按交易对和时间筛选）
    - 通过 WebSocket 每 2 秒更新活跃订单状态
    - 所有订单记录持久化到 MySQL（含手续费信息）
    - _需求: 3.1, 3.2, 3.3, 3.4, 3.5, 3.6, 3.7_

  - [x] 12.4 编写 Property 6 属性测试：交易记录完整性
    - **Property 6: 对于任意交易记录，必须包含所有必填非空字段（交易时间、交易对、方向、价格、数量、金额、手续费、策略名称、决策依据、订单 ID、前后余额）**
    - 在 `internal/order/trade_test.go` 中使用 `rapid` 库实现
    - **验证: 需求 9.2, 4.3**

  - [x] 12.5 实现 CSV 导出功能
    - 在 `internal/order/manager.go` 中实现 `ExportCSV` 方法
    - 支持按时间范围导出交易记录为 CSV 格式
    - _需求: 9.5_

  - [x] 12.6 编写 Property 28 属性测试：CSV 导出往返
    - **Property 28: 对于任意交易记录集合，导出 CSV 后再解析应得到等价数据**
    - 在 `internal/order/export_test.go` 中使用 `rapid` 库实现
    - **验证: 需求 9.5**

- [x] 13. Strategy Engine（策略引擎）
  - [x] 13.1 实现策略接口和内置策略
    - 创建 `internal/strategy/engine.go`，定义 `Strategy` 接口（`Name`、`Calculate`、`GetParams`、`SetParams`、`EstimateFee`）
    - 定义 `Signal`、`SignalReason`、`StrategyParams` 数据结构
    - 创建 `internal/strategy/ma_cross.go`：均线交叉策略（默认短期 7、长期 25）
    - 创建 `internal/strategy/rsi.go`：RSI 超买超卖策略（默认周期 14、超买 70、超卖 30）
    - 创建 `internal/strategy/bollinger.go`：布林带突破策略（默认周期 20、标准差倍数 2.0）
    - 每个策略自动初始化有效默认参数
    - _需求: 4.1, 4.2_

  - [x] 13.2 编写 Property 7 属性测试：策略自动初始化有效默认参数
    - **Property 7: 对于任意内置策略，GetParams() 应返回所有参数为正数的有效默认参数集**
    - 在 `internal/strategy/engine_test.go` 中使用 `rapid` 库实现
    - **验证: 需求 4.2**

  - [x] 13.3 实现策略引擎核心逻辑
    - 在 `internal/strategy/engine.go` 中实现 `StrategyEngine` 结构体
    - 实现 `Start`：启动策略引擎，持续接收行情数据并进行策略计算
    - 实现 `Stop`：停止生成新信号，保留已提交订单跟踪
    - 实现 `EvaluateMarket`：自动评估市场状况并选择适合的策略组合
    - 信号生成时将预估手续费纳入评估，仅在扣除手续费后有正收益预期时生成信号
    - 信号传递给 OrderManager 执行，记录信号生成时间、策略名称、交易对、方向和决策依据
    - 每个策略维护独立运行日志
    - _需求: 4.3, 4.4, 4.5, 4.6, 4.7, 4.8_

  - [x] 13.4 编写 Property 8 属性测试：停止交易后不产生新信号
    - **Property 8: 调用 Stop() 后，无论接收多少行情更新，不应生成任何新交易信号**
    - 在 `internal/strategy/engine_test.go` 中使用 `rapid` 库实现
    - **验证: 需求 4.6**

  - [x] 13.5 编写 Property 9 属性测试：手续费感知的信号生成
    - **Property 9: 对于任意交易信号，扣除手续费后预期收益必须为正；否则信号不应被生成**
    - 在 `internal/strategy/engine_test.go` 中使用 `rapid` 库实现
    - **验证: 需求 4.8**

- [x] 14. 检查点 - 确保核心业务模块测试通过
  - 确保所有测试通过，如有问题请询问用户。

- [x] 15. Backtester（回测模块）
  - [x] 15.1 实现回测引擎
    - 创建 `internal/backtest/backtester.go`
    - 实现 `Backtester` 结构体和 `BacktestConfig`（交易对、策略、时间范围、初始资金、费率、滑点）
    - 实现 `Run`：加载历史 K线数据，模拟策略执行，按 Maker/Taker 费率计算每笔手续费，模拟滑点影响
    - 实现 `BatchRun`：对同一策略使用不同参数组合进行批量回测
    - 生成 `BacktestResult`：总收益率、净收益（扣除手续费）、最大回撤、胜率、盈亏比、总交易次数、总手续费、交易明细、权益曲线
    - 回测结果持久化到 MySQL `backtest_results` 表
    - 实现 `GetResults`：查询指定策略的历史回测记录
    - _需求: 10.1, 10.2, 10.3, 10.4, 10.5, 10.6, 10.7_

  - [x] 15.2 编写 Property 19 属性测试：回测手续费计算正确性
    - **Property 19: 对于任意回测交易，手续费等于成交金额乘以费率；总手续费等于各笔手续费之和**
    - 在 `internal/backtest/backtester_test.go` 中使用 `rapid` 库实现
    - **验证: 需求 10.2**

  - [x] 15.3 编写 Property 20 属性测试：回测报告指标一致性
    - **Property 20: 净收益等于总收益减总手续费；胜率等于盈利交易数除以总交易数；总交易次数等于交易明细长度**
    - 在 `internal/backtest/backtester_test.go` 中使用 `rapid` 库实现
    - **验证: 需求 10.3**

  - [x] 15.4 编写 Property 21 属性测试：回测滑点模拟
    - **Property 21: 对于任意回测交易，实际执行价格与信号价格的差异等于滑点百分比乘以信号价格**
    - 在 `internal/backtest/backtester_test.go` 中使用 `rapid` 库实现
    - **验证: 需求 10.5**

- [x] 16. Strategy Optimizer（策略优化器）
  - [x] 16.1 实现策略优化器
    - 创建 `internal/optimizer/optimizer.go`
    - 实现 `StrategyOptimizer` 结构体和 `OptimizerConfig`（优化周期默认 24h、回看天数默认 30、最大参数变化 30%）
    - 实现 `RunOptimization`：
      1. 从 MySQL 读取历史交易记录，分析策略表现
      2. 在当前参数 ±30% 范围内生成候选参数组合
      3. 对每个候选参数进行回测（含手续费和滑点）
      4. 以净收益率为主要目标，兼顾最大回撤和胜率
      5. 若最优候选优于当前参数，自动更新策略引擎参数
      6. 若最优候选净收益为负，保留当前参数并通知用户
    - 使用 `robfig/cron` 实现定时触发
    - 记录优化详细日志（优化前后参数、指标对比、分析结论）到 `optimization_records` 表
    - 实现 `GetHistory`：查询优化历史记录
    - _需求: 11.1, 11.2, 11.3, 11.4, 11.5, 11.6, 11.7, 11.8_

  - [x] 16.2 编写 Property 22 属性测试：优化器以净收益率为目标
    - **Property 22: 对于任意两组参数的回测结果，优化器应选择净收益率更高的参数组合**
    - 在 `internal/optimizer/optimizer_test.go` 中使用 `rapid` 库实现
    - **验证: 需求 11.4**

  - [x] 16.3 编写 Property 23 属性测试：优化器参数变化幅度限制
    - **Property 23: 对于任意参数优化，每个参数变化幅度不超过 30%**
    - 在 `internal/optimizer/optimizer_test.go` 中使用 `rapid` 库实现
    - **验证: 需求 11.7**

  - [x] 16.4 编写 Property 24 属性测试：优化器决策正确性
    - **Property 24: 最优候选净收益为正且优于当前参数时更新；净收益为负时保持不变**
    - 在 `internal/optimizer/optimizer_test.go` 中使用 `rapid` 库实现
    - **验证: 需求 11.5, 11.8**

- [x] 17. 检查点 - 确保回测和优化器测试通过
  - 确保所有测试通过，如有问题请询问用户。

- [x] 18. 启动恢复与配置持久化
  - [x] 18.1 实现系统启动恢复逻辑
    - 在 `internal/restore/restore.go` 中实现从 MySQL 恢复策略配置和风控参数的逻辑
    - 系统启动时从数据库读取上次运行的策略参数和风控配置
    - _需求: 9.4_

  - [x] 18.2 编写 Property 29 属性测试：策略配置启动恢复
    - **Property 29: 对于任意已保存的策略配置和风控参数，重启后恢复的配置应与保存前完全一致**
    - 在 `internal/config/restore_test.go` 中使用 `rapid` 库实现
    - **验证: 需求 9.4**

- [x] 19. Web 层 - HTTP Server 与 API 路由
  - [x] 19.1 实现用户认证模块
    - 创建 `internal/server/auth.go`
    - 实现用户名密码登录（`POST /api/v1/auth/login`）
    - 密码使用 bcrypt 哈希存储
    - 实现连续 3 次登录失败锁定账户 15 分钟
    - 实现 JWT Token 认证中间件
    - _需求: 12.3, 12.5_

  - [x] 19.2 编写 Property 27 属性测试：账户锁定机制
    - **Property 27: 连续 3 次登录失败后账户锁定 15 分钟；锁定期间即使密码正确也拒绝登录**
    - 在 `internal/server/auth_test.go` 中使用 `rapid` 库实现
    - **验证: 需求 12.5**

  - [x] 19.3 实现 HTTP API 路由和处理器
    - 创建 `internal/server/router.go` 和 `internal/server/handler.go`
    - 使用 Gin 框架实现所有 API 端点：
      - 行情：`GET /api/v1/market/klines/:symbol`、`GET /api/v1/market/orderbook/:symbol`
      - 订单：`POST /api/v1/orders`、`DELETE /api/v1/orders/:id`、`GET /api/v1/orders`、`GET /api/v1/orders/export`
      - 账户：`GET /api/v1/account/balances`、`GET /api/v1/account/pnl`、`GET /api/v1/account/fees`
      - 策略：`POST /api/v1/strategy/start`、`POST /api/v1/strategy/stop`、`GET /api/v1/strategy/status`
      - 风控：`GET /api/v1/risk/config`、`PUT /api/v1/risk/config`
      - 回测：`POST /api/v1/backtest/run`、`GET /api/v1/backtest/results`
      - 优化：`GET /api/v1/optimizer/history`
      - 通知：`GET /api/v1/notifications`、`PUT /api/v1/notifications/:id/read`、`PUT /api/v1/notifications/settings`
      - 交易记录：`GET /api/v1/trades`
      - 认证：`POST /api/v1/auth/login`
    - 所有端点（除登录外）需 JWT 认证
    - _需求: 7.4, 7.5, 7.7, 7.10, 7.11, 7.12, 9.7_

  - [x] 19.4 实现 WebSocket 服务
    - 创建 `internal/server/websocket.go`
    - 实现 `WebSocketHub` 管理所有客户端连接
    - 实现实时数据推送：行情更新、订单状态变更、通知消息
    - 客户端无需手动刷新即可接收实时更新
    - _需求: 7.9_

- [x] 20. 检查点 - 确保 Web 层后端测试通过
  - 确保所有测试通过，如有问题请询问用户。

- [x] 21. Web UI 前端（React + TypeScript）
  - [x] 21.1 初始化 React 前端项目
    - 在 `web/` 目录下初始化 React + TypeScript 项目
    - 引入依赖：`lightweight-charts`（TradingView K线图表）、`axios`、WebSocket 客户端
    - 创建基础项目结构：`components/`、`pages/`、`services/`
    - _需求: 7.1_

  - [x] 21.2 实现 API 服务层和 WebSocket 客户端
    - 创建 `web/src/services/api.ts`，封装所有 HTTP API 调用
    - 创建 `web/src/services/websocket.ts`，实现 WebSocket 连接管理和消息分发
    - _需求: 7.9_

  - [x] 21.3 实现登录页面
    - 创建登录表单页面，用户名密码认证
    - JWT Token 管理（存储、刷新、过期处理）
    - _需求: 12.3_

  - [x] 21.4 实现 K线图表和行情展示
    - 使用 Lightweight Charts 实现实时 K线图表，支持缩放、拖拽和切换时间周期
    - 在 K线图上叠加技术指标（均线、RSI、布林带）
    - 实现实时订单簿深度图
    - _需求: 7.1, 7.2, 7.3_

  - [x] 21.5 实现订单管理面板
    - 创建订单创建表单（选择订单类型、输入交易对、数量、价格）
    - 展示当前活跃订单列表和历史订单列表，支持按交易对和时间筛选
    - _需求: 7.4, 7.5_

  - [x] 21.6 实现账户资产概览面板
    - 展示总资产价值、各币种余额、持仓盈亏和累计手续费
    - _需求: 7.6_

  - [x] 21.7 实现自动交易控制面板
    - 一键启动/停止自动交易按钮
    - 展示当前系统自动选择的策略及其参数
    - 展示策略运行状态和交易信号历史记录
    - _需求: 7.7, 7.8_

  - [x] 21.8 实现风控设置面板
    - 配置各项风控参数（单笔最大金额、每日最大亏损、止损百分比、最大持仓比例）
    - _需求: 7.10_

  - [x] 21.9 实现回测结果展示面板
    - 以图表形式展示收益曲线、最大回撤、胜率和手续费消耗
    - _需求: 7.11_

  - [x] 21.10 实现策略优化历史面板
    - 展示每次参数调整记录和优化前后性能对比
    - _需求: 7.12_

  - [x] 21.11 实现通知消息面板
    - 弹窗或消息列表展示通知
    - 支持已读/未读标记和事件类型过滤配置
    - _需求: 8.2, 8.4, 8.5_

  - [x] 21.12 实现交易记录查询面板
    - 支持按交易对、时间范围、策略名称筛选
    - 展示每笔交易的完整决策依据详情
    - _需求: 9.7_

- [x] 22. 检查点 - 确保前端页面功能完整
  - 确保所有测试通过，如有问题请询问用户。

- [x] 23. 系统集成与程序入口
  - [x] 23.1 实现程序入口和模块组装
    - 创建 `cmd/server/main.go`
    - 实现系统启动流程：加载配置 → 验证配置 → 初始化数据库 → 恢复策略配置 → 初始化各模块 → 启动 HTTP/WebSocket 服务 → 启动策略优化定时任务
    - 实现优雅关闭：停止策略引擎 → 关闭 WebSocket 连接 → 关闭数据库连接
    - 配置 HTTPS（TLS 1.2+）
    - _需求: 9.4, 12.4, 12.6_

  - [x] 23.2 创建 Makefile
    - 定义构建、测试、运行、数据库迁移等常用命令
    - _需求: 无（开发便利性）_

- [x] 24. 最终检查点 - 确保所有测试通过
  - 运行全部单元测试和属性测试，确保所有测试通过。
  - 如有问题请询问用户。

## 备注

- 标记 `*` 的任务为可选任务，可跳过以加速 MVP 开发
- 每个任务引用了具体的需求编号，确保需求可追溯
- 检查点确保增量验证，及时发现问题
- 属性测试验证通用正确性属性，单元测试验证具体示例和边界情况
- Go 后端使用 `pgregory.net/rapid` 作为属性测试库
- 数据库集成测试使用 `testcontainers-go` 启动 MySQL 容器
