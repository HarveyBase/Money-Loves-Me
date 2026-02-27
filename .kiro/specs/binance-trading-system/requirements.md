# 需求文档：币安加密货币交易系统

## 简介

本系统是一个基于 Go 语言开发的加密货币自动交易系统，通过接入币安（Binance）API 实现全自动化交易功能。系统提供 Web UI 可视化界面，支持实时行情监控、自动策略交易、历史行情回测、策略自我优化、订单管理和账户资产管理。系统的交易策略由系统自动决策和优化，用户无需具备交易知识，系统通过历史行情回测验证策略有效性并自动调整参数，所有回测和实盘交易均将交易手续费纳入计算。

## 术语表

- **Trading_System**: 本加密货币自动交易系统的整体应用
- **API_Client**: 负责与币安 REST API 和 WebSocket API 通信的客户端模块
- **Order_Manager**: 负责创建、提交、跟踪和取消订单的模块
- **Strategy_Engine**: 负责自动选择、执行交易策略并生成交易信号的模块，策略参数由系统自动决策
- **Market_Data_Service**: 负责获取和分发实时及历史行情数据的模块
- **Account_Service**: 负责管理用户账户信息和资产余额的模块
- **Web_UI**: 基于浏览器的可视化用户界面
- **Risk_Manager**: 负责风险控制和资金管理的模块
- **Notification_Service**: 负责向用户发送交易通知和告警的模块
- **Backtester**: 负责使用历史行情数据对交易策略进行回测验证的模块
- **Strategy_Optimizer**: 负责根据回测结果自动调整和优化策略参数的模块
- **K线数据**: 包含开盘价、最高价、最低价、收盘价和成交量的时间序列数据（OHLCV）
- **交易对**: 两种加密货币的交易组合，如 BTC/USDT
- **止损单**: 当价格达到预设止损价时自动触发的卖出订单
- **止盈单**: 当价格达到预设止盈价时自动触发的卖出订单
- **交易手续费**: 币安对每笔交易收取的费用，包括 Maker 费率和 Taker 费率
- **回测**: 使用历史行情数据模拟策略执行，验证策略在过去市场条件下的表现
- **净收益**: 扣除所有交易手续费后的实际盈亏金额

## 需求

### 需求 1：币安 API 连接与认证

**用户故事：** 作为交易者，我希望系统能安全地连接到币安 API，以便我可以访问市场数据和执行交易操作。

#### 验收标准

1. WHEN 用户提供有效的 API Key 和 Secret Key，THE API_Client SHALL 成功建立与币安 API 的认证连接，并在 5 秒内返回连接状态
2. WHEN 用户提供无效的 API Key 或 Secret Key，THE API_Client SHALL 返回明确的认证失败错误信息，包含错误码和错误描述
3. THE API_Client SHALL 使用 HMAC-SHA256 签名对所有需要认证的请求进行签名
4. THE Trading_System SHALL 将 API Key 和 Secret Key 以加密形式存储在本地配置文件中，禁止明文存储
5. WHEN 与币安 API 的连接断开，THE API_Client SHALL 在 3 秒内自动尝试重新连接，最多重试 5 次
6. IF 重连 5 次均失败，THEN THE API_Client SHALL 记录错误日志并通过 Notification_Service 通知用户连接中断

### 需求 2：实时行情数据获取

**用户故事：** 作为交易者，我希望能实时获取加密货币的行情数据，以便我可以做出及时的交易决策。

#### 验收标准

1. THE Market_Data_Service SHALL 通过币安 WebSocket API 订阅指定交易对的实时价格推送
2. WHEN 收到新的行情数据，THE Market_Data_Service SHALL 在 100 毫秒内将数据分发给所有已订阅的消费者（Web_UI、Strategy_Engine）
3. THE Market_Data_Service SHALL 支持获取以下时间周期的 K线数据：1 分钟、5 分钟、15 分钟、1 小时、4 小时、1 天
4. WHEN 用户请求历史 K线数据，THE Market_Data_Service SHALL 通过币安 REST API 获取并返回指定交易对和时间范围的历史数据
5. THE Market_Data_Service SHALL 获取并维护指定交易对的实时订单簿深度数据（买一至买二十、卖一至卖二十）
6. IF WebSocket 连接中断，THEN THE Market_Data_Service SHALL 自动重新订阅所有之前订阅的数据流

### 需求 3：订单管理

**用户故事：** 作为交易者，我希望能创建和管理各种类型的订单，以便我可以灵活地执行交易策略。

#### 验收标准

1. THE Order_Manager SHALL 支持以下订单类型：限价单、市价单、止损限价单、止盈限价单
2. WHEN 用户提交一个新订单，THE Order_Manager SHALL 在提交前验证订单参数（交易对、数量、价格）是否符合币安的交易规则
3. WHEN 订单提交成功，THE Order_Manager SHALL 返回币安分配的订单 ID，并将订单状态设置为"已提交"
4. WHILE 订单处于活跃状态，THE Order_Manager SHALL 每 2 秒通过 WebSocket 更新订单的最新状态（部分成交、完全成交、已取消）
5. WHEN 用户请求取消一个活跃订单，THE Order_Manager SHALL 向币安 API 发送取消请求，并在收到确认后更新本地订单状态为"已取消"
6. THE Order_Manager SHALL 将所有订单记录（包含订单类型、交易对、数量、价格、状态、时间戳、交易手续费）持久化存储到本地数据库
7. IF 订单提交失败，THEN THE Order_Manager SHALL 记录失败原因并通过 Notification_Service 通知用户

### 需求 4：交易策略引擎（自动决策）

**用户故事：** 作为不具备交易知识的用户，我希望系统能自动选择和执行交易策略，以便我无需手动配置策略参数即可实现自动化交易。

#### 验收标准

1. THE Strategy_Engine SHALL 内置至少以下交易策略：均线交叉策略（MA Cross）、RSI 超买超卖策略、布林带突破策略
2. THE Strategy_Engine SHALL 自动为每个策略选择初始参数，用户无需手动配置策略参数
3. WHEN Strategy_Engine 生成一个交易信号（买入或卖出），THE Strategy_Engine SHALL 将信号传递给 Order_Manager 执行，并记录信号的生成时间、策略名称、交易对和方向
4. WHILE 策略处于运行状态，THE Strategy_Engine SHALL 持续接收 Market_Data_Service 推送的实时数据并进行策略计算
5. WHEN 用户启动自动交易，THE Strategy_Engine SHALL 自动评估当前市场状况并选择适合的策略组合进行执行
6. WHEN 用户停止自动交易，THE Strategy_Engine SHALL 立即停止生成新的交易信号，但保留已提交订单的跟踪
7. THE Strategy_Engine SHALL 为每个策略维护独立的运行日志，记录每次信号生成和执行结果
8. WHEN Strategy_Engine 生成交易信号时，THE Strategy_Engine SHALL 将预估交易手续费纳入信号评估，仅在扣除手续费后仍有正收益预期时才生成信号

### 需求 5：账户与资产管理

**用户故事：** 作为交易者，我希望能实时查看我的账户资产状况，以便我可以了解资金分布和盈亏情况。

#### 验收标准

1. THE Account_Service SHALL 通过币安 API 获取用户账户中所有币种的可用余额和冻结余额
2. WHEN 账户余额发生变化（订单成交、充值、提现），THE Account_Service SHALL 在 5 秒内更新本地缓存的余额数据
3. THE Account_Service SHALL 计算并展示用户的总资产价值（以 USDT 计价）
4. THE Account_Service SHALL 计算每个交易对的持仓盈亏（包含已实现盈亏和未实现盈亏），盈亏计算须扣除交易手续费
5. WHEN 用户请求资产历史记录，THE Account_Service SHALL 返回指定时间范围内的资产变动明细
6. THE Account_Service SHALL 记录每笔交易产生的手续费金额和币种，并提供累计手续费统计

### 需求 6：风险控制

**用户故事：** 作为交易者，我希望系统具备风险控制机制，以便我可以限制潜在损失并保护资金安全。

#### 验收标准

1. THE Risk_Manager SHALL 允许用户设置单笔交易的最大金额上限（以 USDT 计价）
2. THE Risk_Manager SHALL 允许用户设置每日最大亏损金额上限（以 USDT 计价）
3. WHEN 单笔交易金额超过用户设置的上限，THE Risk_Manager SHALL 拒绝该交易并通知用户
4. WHEN 当日累计亏损（含交易手续费）达到用户设置的上限，THE Risk_Manager SHALL 自动暂停所有运行中的策略，并通过 Notification_Service 通知用户
5. THE Risk_Manager SHALL 允许用户为每个交易对设置止损百分比，WHEN 持仓亏损达到该百分比，THE Risk_Manager SHALL 自动生成止损卖出信号
6. THE Risk_Manager SHALL 允许用户设置单个交易对的最大持仓比例（占总资产的百分比）
7. IF 新订单将导致某交易对持仓比例超过设定上限，THEN THE Risk_Manager SHALL 拒绝该订单并通知用户
8. THE Risk_Manager SHALL 在计算盈亏和风险指标时，将交易手续费作为成本纳入计算


### 需求 7：Web UI 可视化界面

**用户故事：** 作为交易者，我希望通过直观的 Web 界面监控市场和管理交易，以便我可以高效地操作交易系统。

#### 验收标准

1. THE Web_UI SHALL 展示实时 K线图表，支持缩放、拖拽和切换时间周期
2. THE Web_UI SHALL 在 K线图表上叠加显示技术指标（均线、RSI、布林带）
3. THE Web_UI SHALL 展示实时订单簿深度图，以可视化方式呈现买卖盘分布
4. THE Web_UI SHALL 提供订单创建表单，允许用户选择订单类型、输入交易对、数量和价格
5. THE Web_UI SHALL 展示当前活跃订单列表和历史订单列表，支持按交易对和时间筛选
6. THE Web_UI SHALL 展示账户资产概览面板，包含总资产价值、各币种余额、持仓盈亏和累计手续费
7. THE Web_UI SHALL 提供自动交易控制面板，允许用户一键启动或停止自动交易，并展示当前系统自动选择的策略及其参数
8. THE Web_UI SHALL 展示策略运行状态和交易信号历史记录
9. WHEN 有新的交易信号或订单状态变更，THE Web_UI SHALL 通过 WebSocket 实时更新界面，无需用户手动刷新
10. THE Web_UI SHALL 提供风险控制设置面板，允许用户配置各项风控参数
11. THE Web_UI SHALL 提供回测结果展示面板，以图表形式展示策略回测的收益曲线、最大回撤、胜率和手续费消耗
12. THE Web_UI SHALL 提供策略优化历史面板，展示 Strategy_Optimizer 每次参数调整的记录和优化前后的性能对比

### 需求 8：通知与告警

**用户故事：** 作为交易者，我希望在重要事件发生时收到及时通知，以便我不会错过关键的交易机会或风险事件。

#### 验收标准

1. THE Notification_Service SHALL 在以下事件发生时生成通知：订单成交、策略信号触发、风控告警、API 连接异常、回测完成、策略参数优化完成
2. THE Notification_Service SHALL 在 Web_UI 中以弹窗或消息列表形式展示通知
3. THE Notification_Service SHALL 为每条通知记录时间戳、事件类型和详细描述
4. WHEN 用户查看通知列表，THE Notification_Service SHALL 按时间倒序展示所有通知，并标记已读和未读状态
5. THE Notification_Service SHALL 允许用户配置需要接收通知的事件类型

### 需求 9：数据持久化与日志

**用户故事：** 作为交易者，我希望系统能持久化存储交易数据和运行日志，以便我可以回顾历史交易和排查问题。

#### 验收标准

1. THE Trading_System SHALL 使用 MySQL 数据库持久化存储订单记录、交易记录、账户快照、回测结果和策略优化历史
2. WHEN 每笔交易（买入或卖出）执行完成，THE Trading_System SHALL 将详细交易记录写入 MySQL 数据库，记录内容包含：交易时间、交易对、交易方向（买入/卖出）、成交价格、成交数量、成交金额、交易手续费、触发策略名称、策略决策依据（包含当时的技术指标数值、信号触发条件、市场状态评估）、订单 ID、交易前后账户余额
3. THE Trading_System SHALL 记录结构化运行日志，包含时间戳、模块名称、日志级别（DEBUG、INFO、WARN、ERROR）和日志内容
4. WHEN Trading_System 启动时，THE Trading_System SHALL 从 MySQL 数据库恢复上次运行的策略配置和风控参数
5. THE Trading_System SHALL 支持导出指定时间范围的交易记录为 CSV 格式文件
6. THE Trading_System SHALL 对日志文件实施轮转策略，单个日志文件大小上限为 100MB，保留最近 30 天的日志
7. THE Web_UI SHALL 提供交易记录查询面板，支持按交易对、时间范围、策略名称筛选交易记录，并展示每笔交易的完整决策依据详情

### 需求 10：历史行情回测

**用户故事：** 作为用户，我希望系统能使用历史行情数据回测交易策略，以便在实盘交易前验证策略的有效性和盈利能力。

#### 验收标准

1. THE Backtester SHALL 支持加载指定交易对和时间范围的历史 K线数据进行策略回测
2. THE Backtester SHALL 在回测过程中模拟真实交易环境，包括按币安费率计算每笔交易的 Maker 和 Taker 手续费
3. WHEN 回测完成，THE Backtester SHALL 生成回测报告，包含以下指标：总收益率、净收益（扣除手续费）、最大回撤、胜率、盈亏比、总交易次数和总手续费消耗
4. THE Backtester SHALL 支持对同一策略使用不同参数组合进行批量回测，以找到最优参数
5. THE Backtester SHALL 在回测中模拟滑点影响，避免回测结果过于理想化
6. THE Backtester SHALL 将每次回测的结果和参数持久化存储到 MySQL 数据库，供 Strategy_Optimizer 和 Web_UI 查询
7. WHEN 用户请求查看回测结果，THE Backtester SHALL 返回指定策略的所有历史回测记录

### 需求 11：策略自我优化

**用户故事：** 作为用户，我希望系统能根据回测结果自动优化策略参数，以便策略能持续适应市场变化并提升盈利能力。

#### 验收标准

1. THE Strategy_Optimizer SHALL 定期（可配置周期，默认每 24 小时）自动触发策略回测和参数优化流程
2. WHEN 优化流程触发，THE Strategy_Optimizer SHALL 从 MySQL 数据库读取历史交易记录（包含策略决策依据），分析各策略在不同市场条件下的实际表现
3. THE Strategy_Optimizer SHALL 结合 MySQL 中的历史交易记录和最近一段时间（可配置，默认 30 天）的历史行情数据，对所有内置策略进行回测
4. THE Strategy_Optimizer SHALL 以扣除手续费后的净收益率作为主要优化目标，同时考虑最大回撤和胜率
5. WHEN Strategy_Optimizer 找到比当前参数表现更优的参数组合，THE Strategy_Optimizer SHALL 自动更新 Strategy_Engine 使用的策略参数
6. THE Strategy_Optimizer SHALL 记录每次优化的详细日志，包含优化前参数、优化后参数、优化前后的回测指标对比，以及从历史交易记录中提取的关键分析结论
7. THE Strategy_Optimizer SHALL 设置参数调整幅度上限，单次优化中每个参数的变化幅度不超过当前值的 30%，避免参数剧烈波动
8. IF 优化后的策略在回测中净收益为负，THEN THE Strategy_Optimizer SHALL 保留当前参数不变，并通过 Notification_Service 通知用户当前市场条件下策略表现不佳

### 需求 12：系统配置与安全

**用户故事：** 作为交易者，我希望系统具备完善的配置管理和安全机制，以便我可以安全地使用交易系统。

#### 验收标准

1. THE Trading_System SHALL 通过 YAML 配置文件管理系统参数（API 端点、MySQL 数据库连接配置（主机、端口、用户名、密码、数据库名）、日志级别、默认交易对列表）
2. THE Trading_System SHALL 对配置文件中的敏感信息（API Key、Secret Key、MySQL 数据库密码）进行 AES-256 加密存储
3. THE Web_UI SHALL 要求用户通过用户名和密码登录后才能访问系统功能
4. THE Web_UI SHALL 使用 HTTPS（TLS 1.2 或更高版本）加密所有客户端与服务端之间的通信
5. WHEN 用户连续 3 次登录失败，THE Web_UI SHALL 锁定该账户 15 分钟
6. THE Trading_System SHALL 在启动时验证配置文件的完整性和格式正确性，IF 配置文件无效，THEN THE Trading_System SHALL 输出明确的错误信息并拒绝启动
