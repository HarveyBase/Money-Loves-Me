package risk

import (
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/shopspring/decimal"
	"go.uber.org/zap"

	apperrors "money-loves-me/internal/errors"
	"money-loves-me/internal/logger"
	"money-loves-me/internal/model"
	"money-loves-me/internal/notification"
)

// StopLossSignal 表示由 RiskManager 生成的止损卖出信号。
type StopLossSignal struct {
	Symbol       string
	EntryPrice   decimal.Decimal
	CurrentPrice decimal.Decimal
	Quantity     decimal.Decimal
	LossPercent  decimal.Decimal
	Timestamp    time.Time
}

// StrategyPauser 是暂停所有运行中策略的接口。
type StrategyPauser interface {
	PauseAll() error
}

// AccountValuer 提供用于仓位比例检查的总资产价值。
type AccountValuer interface {
	GetTotalAssetValue() (decimal.Decimal, error)
}

// RiskStore 定义风控配置的持久化接口。
type RiskStore interface {
	Get() (*model.RiskConfig, error)
	Save(config *model.RiskConfig) error
}

// RiskConfig 保存内存中的风控参数。
type RiskConfig struct {
	MaxOrderAmount   decimal.Decimal            // 单笔订单最大金额（USDT）
	MaxDailyLoss     decimal.Decimal            // 每日最大亏损（USDT）
	StopLossPercent  map[string]decimal.Decimal // 每个交易对的止损百分比
	MaxPositionRatio map[string]decimal.Decimal // 每个交易对的最大仓位比例
}

// RiskManager 实现交易系统的风控检查。
type RiskManager struct {
	config   RiskConfig
	store    RiskStore
	notifier *notification.NotificationService
	pauser   StrategyPauser
	log      *logger.Logger
	mu       sync.RWMutex
}

// NewRiskManager 使用给定的依赖项创建一个新的 RiskManager。
func NewRiskManager(
	store RiskStore,
	notifier *notification.NotificationService,
	pauser StrategyPauser,
	log *logger.Logger,
) *RiskManager {
	return &RiskManager{
		config: RiskConfig{
			StopLossPercent:  make(map[string]decimal.Decimal),
			MaxPositionRatio: make(map[string]decimal.Decimal),
		},
		store:    store,
		notifier: notifier,
		pauser:   pauser,
		log:      log,
	}
}

// logWarn 安全地以 WARN 级别记录日志，处理 nil logger。
func (rm *RiskManager) logWarn(msg string, fields ...zap.Field) {
	if rm.log != nil {
		rm.log.Warn(msg, fields...)
	}
}

// logInfo 安全地以 INFO 级别记录日志，处理 nil logger。
func (rm *RiskManager) logInfo(msg string, fields ...zap.Field) {
	if rm.log != nil {
		rm.log.Info(msg, fields...)
	}
}

// logError 安全地以 ERROR 级别记录日志，处理 nil logger。
func (rm *RiskManager) logError(msg string, fields ...zap.Field) {
	if rm.log != nil {
		rm.log.Error(msg, fields...)
	}
}

// GetConfig 返回当前风控配置的副本。
func (rm *RiskManager) GetConfig() RiskConfig {
	rm.mu.RLock()
	defer rm.mu.RUnlock()
	return rm.copyConfig()
}

// SetConfig 更新内存中的风控配置。
func (rm *RiskManager) SetConfig(cfg RiskConfig) {
	rm.mu.Lock()
	defer rm.mu.Unlock()
	rm.config = cfg
}

// copyConfig 返回当前配置的深拷贝（调用者必须持有锁）。
func (rm *RiskManager) copyConfig() RiskConfig {
	slp := make(map[string]decimal.Decimal, len(rm.config.StopLossPercent))
	for k, v := range rm.config.StopLossPercent {
		slp[k] = v
	}
	mpr := make(map[string]decimal.Decimal, len(rm.config.MaxPositionRatio))
	for k, v := range rm.config.MaxPositionRatio {
		mpr[k] = v
	}
	return RiskConfig{
		MaxOrderAmount:   rm.config.MaxOrderAmount,
		MaxDailyLoss:     rm.config.MaxDailyLoss,
		StopLossPercent:  slp,
		MaxPositionRatio: mpr,
	}
}

// CheckOrder 验证订单是否被风控规则允许。
// 检查：(1) 金额 <= MaxOrderAmount，(2) 仓位比例 <= MaxPositionRatio。
func (rm *RiskManager) CheckOrder(symbol string, amount, totalAssetValue decimal.Decimal) error {
	rm.mu.RLock()
	defer rm.mu.RUnlock()

	// 检查单笔订单金额限制。
	if rm.config.MaxOrderAmount.IsPositive() && amount.GreaterThan(rm.config.MaxOrderAmount) {
		rm.logWarn("order rejected: exceeds max order amount",
			zap.String("symbol", symbol),
			zap.String("amount", amount.String()),
			zap.String("limit", rm.config.MaxOrderAmount.String()),
		)
		return apperrors.NewAppError(apperrors.ErrRiskControl,
			fmt.Sprintf("order amount %s exceeds max order amount %s",
				amount.String(), rm.config.MaxOrderAmount.String()),
			"risk", nil)
	}

	// 检查仓位比例限制。
	if maxRatio, ok := rm.config.MaxPositionRatio[symbol]; ok && maxRatio.IsPositive() {
		if totalAssetValue.IsPositive() {
			ratio := amount.Div(totalAssetValue)
			if ratio.GreaterThan(maxRatio) {
				rm.logWarn("order rejected: exceeds max position ratio",
					zap.String("symbol", symbol),
					zap.String("ratio", ratio.String()),
					zap.String("limit", maxRatio.String()),
				)
				return apperrors.NewAppError(apperrors.ErrRiskControl,
					fmt.Sprintf("position ratio %s exceeds max position ratio %s for %s",
						ratio.String(), maxRatio.String(), symbol),
					"risk", nil)
			}
		}
	}

	return nil
}

// CheckDailyLoss 根据给定的交易记录计算当日累计亏损（含手续费），
// 并返回是否应暂停策略。
func (rm *RiskManager) CheckDailyLoss(trades []model.Trade) (shouldPause bool, dailyLoss decimal.Decimal) {
	rm.mu.RLock()
	maxDailyLoss := rm.config.MaxDailyLoss
	rm.mu.RUnlock()

	now := time.Now()
	startOfDay := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())

	dailyLoss = decimal.Zero
	for _, t := range trades {
		if t.ExecutedAt.Before(startOfDay) {
			continue
		}
		// 对于 SELL 交易，利润 = 金额 - 手续费（正值表示盈利）。
		// 对于 BUY 交易，成本 = 金额 + 手续费（负值表示支出）。
		// 每日净盈亏：sum(卖出金额 - 买入金额 - 所有手续费)。
		switch t.Side {
		case "SELL":
			dailyLoss = dailyLoss.Sub(t.Amount) // 卖出增加收入（减少亏损）
		case "BUY":
			dailyLoss = dailyLoss.Add(t.Amount) // 买入增加成本（增加亏损）
		}
		dailyLoss = dailyLoss.Add(t.Fee) // 手续费总是增加亏损
	}

	// dailyLoss > 0 表示净亏损；dailyLoss < 0 表示净盈利。
	if maxDailyLoss.IsPositive() && dailyLoss.GreaterThanOrEqual(maxDailyLoss) {
		return true, dailyLoss
	}

	return false, dailyLoss
}

// GenerateStopLossSignal 检查持仓的亏损是否达到止损阈值，
// 如果达到则返回止损卖出信号。未触发时返回 nil。
func (rm *RiskManager) GenerateStopLossSignal(
	symbol string,
	entryPrice, currentPrice, quantity decimal.Decimal,
) *StopLossSignal {
	rm.mu.RLock()
	stopLossPct, ok := rm.config.StopLossPercent[symbol]
	rm.mu.RUnlock()

	if !ok || !stopLossPct.IsPositive() {
		return nil
	}

	if entryPrice.IsZero() {
		return nil
	}

	// lossPercent = (entryPrice - currentPrice) / entryPrice * 100
	// 正的 lossPercent 表示价格下跌（亏损）。
	lossPercent := entryPrice.Sub(currentPrice).Div(entryPrice).Mul(decimal.NewFromInt(100))

	if lossPercent.GreaterThanOrEqual(stopLossPct) {
		rm.logWarn("stop-loss signal generated",
			zap.String("symbol", symbol),
			zap.String("lossPercent", lossPercent.String()),
			zap.String("threshold", stopLossPct.String()),
		)
		return &StopLossSignal{
			Symbol:       symbol,
			EntryPrice:   entryPrice,
			CurrentPrice: currentPrice,
			Quantity:     quantity,
			LossPercent:  lossPercent,
			Timestamp:    time.Now(),
		}
	}

	return nil
}

// PauseAllStrategies 暂停所有运行中的策略并发送风控告警通知。
func (rm *RiskManager) PauseAllStrategies() error {
	rm.logWarn("pausing all strategies due to risk control trigger")

	if rm.pauser != nil {
		if err := rm.pauser.PauseAll(); err != nil {
			rm.logError("failed to pause strategies", zap.Error(err))
			return apperrors.NewAppError(apperrors.ErrRiskControl,
				"failed to pause strategies", "risk", err)
		}
	}

	if rm.notifier != nil {
		_ = rm.notifier.Send(
			notification.EventRiskAlert,
			"All strategies paused",
			"Risk control triggered: all running strategies have been paused. Please review your risk settings and market conditions.",
		)
	}

	return nil
}

// SaveConfig 将当前内存中的风控配置持久化到数据库。
func (rm *RiskManager) SaveConfig() error {
	rm.mu.RLock()
	cfg := rm.copyConfig()
	rm.mu.RUnlock()

	slpJSON, err := json.Marshal(cfg.StopLossPercent)
	if err != nil {
		return apperrors.NewAppError(apperrors.ErrDatabase,
			"failed to marshal stop loss percents", "risk", err)
	}

	mprJSON, err := json.Marshal(cfg.MaxPositionRatio)
	if err != nil {
		return apperrors.NewAppError(apperrors.ErrDatabase,
			"failed to marshal max position percents", "risk", err)
	}

	dbConfig := &model.RiskConfig{
		MaxOrderAmount:      cfg.MaxOrderAmount,
		MaxDailyLoss:        cfg.MaxDailyLoss,
		StopLossPercents:    slpJSON,
		MaxPositionPercents: mprJSON,
	}

	if err := rm.store.Save(dbConfig); err != nil {
		return apperrors.NewAppError(apperrors.ErrDatabase,
			"failed to save risk config", "risk", err)
	}

	rm.logInfo("risk config saved to database")
	return nil
}

// LoadConfig 从数据库加载风控配置到内存。
func (rm *RiskManager) LoadConfig() error {
	dbConfig, err := rm.store.Get()
	if err != nil {
		return apperrors.NewAppError(apperrors.ErrDatabase,
			"failed to load risk config", "risk", err)
	}

	var slp map[string]decimal.Decimal
	if err := json.Unmarshal(dbConfig.StopLossPercents, &slp); err != nil {
		return apperrors.NewAppError(apperrors.ErrDatabase,
			"failed to unmarshal stop loss percents", "risk", err)
	}

	var mpr map[string]decimal.Decimal
	if err := json.Unmarshal(dbConfig.MaxPositionPercents, &mpr); err != nil {
		return apperrors.NewAppError(apperrors.ErrDatabase,
			"failed to unmarshal max position percents", "risk", err)
	}

	rm.mu.Lock()
	rm.config = RiskConfig{
		MaxOrderAmount:   dbConfig.MaxOrderAmount,
		MaxDailyLoss:     dbConfig.MaxDailyLoss,
		StopLossPercent:  slp,
		MaxPositionRatio: mpr,
	}
	rm.mu.Unlock()

	rm.logInfo("risk config loaded from database")
	return nil
}
