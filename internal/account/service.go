package account

import (
	"context"
	"sync"
	"time"

	"github.com/shopspring/decimal"
	"go.uber.org/zap"

	apperrors "money-loves-me/internal/errors"
	"money-loves-me/internal/logger"
	"money-loves-me/internal/model"
	"money-loves-me/internal/store"
	"money-loves-me/pkg/binance"
)

const (
	// cacheRefreshInterval 是定期刷新余额缓存的时间间隔。
	cacheRefreshInterval = 5 * time.Second
)

// BalanceInfo 保存单个资产的可用和冻结金额。
type BalanceInfo struct {
	Free   decimal.Decimal
	Locked decimal.Decimal
}

// PnL 保存交易对的盈亏信息。
type PnL struct {
	Symbol      string
	RealizedPnL decimal.Decimal // 已实现盈亏（卖出 - 买入 - 手续费）
	TotalFees   decimal.Decimal // 已支付的总手续费
	BuyAmount   decimal.Decimal // 总买入成本
	SellAmount  decimal.Decimal // 总卖出收入
	TradeCount  int
}

// FeeStatistics 保存累计手续费统计信息。
type FeeStatistics struct {
	TotalFees   decimal.Decimal            // 所有交易的总手续费
	FeeByAsset  map[string]decimal.Decimal // 按手续费资产分组的手续费
	FeeBySymbol map[string]decimal.Decimal // 按交易对分组的手续费
	TradeCount  int
}

// MarketPricer 提供资产的当前市场价格。
type MarketPricer interface {
	GetCurrentPrice(symbol string) (decimal.Decimal, error)
}

// BinanceAccountClient 定义 AccountService 使用的 BinanceClient 子集。
type BinanceAccountClient interface {
	GetAccountInfo() (*binance.AccountInfo, error)
}

// AccountService 管理账户余额、资产估值、盈亏和手续费统计。
type AccountService struct {
	client       BinanceAccountClient
	market       MarketPricer
	tradeStore   *store.TradeStore
	accountStore *store.AccountStore
	log          *logger.Logger

	mu    sync.RWMutex
	cache map[string]BalanceInfo // 资产 -> 余额信息

	cancelRefresh context.CancelFunc
}

// NewAccountService 创建一个新的 AccountService 并启动后台缓存刷新协程。
func NewAccountService(
	client BinanceAccountClient,
	market MarketPricer,
	tradeStore *store.TradeStore,
	accountStore *store.AccountStore,
	log *logger.Logger,
) *AccountService {
	ctx, cancel := context.WithCancel(context.Background())
	s := &AccountService{
		client:        client,
		market:        market,
		tradeStore:    tradeStore,
		accountStore:  accountStore,
		log:           log,
		cache:         make(map[string]BalanceInfo),
		cancelRefresh: cancel,
	}
	go s.refreshLoop(ctx)
	return s
}

// Close 停止后台缓存刷新协程。
func (s *AccountService) Close() {
	s.cancelRefresh()
}

// refreshLoop 定期从 Binance API 刷新余额缓存。
func (s *AccountService) refreshLoop(ctx context.Context) {
	// 立即执行一次初始获取。
	s.refreshCache()

	ticker := time.NewTicker(cacheRefreshInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			s.refreshCache()
		}
	}
}

// refreshCache 从 Binance 获取账户信息并更新本地缓存。
func (s *AccountService) refreshCache() {
	info, err := s.client.GetAccountInfo()
	if err != nil {
		s.log.Warn("failed to refresh account balances", zap.Error(err))
		return
	}

	newCache := make(map[string]BalanceInfo, len(info.Balances))
	for _, b := range info.Balances {
		free, err := decimal.NewFromString(b.Free)
		if err != nil {
			s.log.Warn("failed to parse free balance",
				zap.String("asset", b.Asset), zap.Error(err))
			continue
		}
		locked, err := decimal.NewFromString(b.Locked)
		if err != nil {
			s.log.Warn("failed to parse locked balance",
				zap.String("asset", b.Asset), zap.Error(err))
			continue
		}
		// 仅缓存余额非零的资产。
		if free.IsZero() && locked.IsZero() {
			continue
		}
		newCache[b.Asset] = BalanceInfo{Free: free, Locked: locked}
	}

	s.mu.Lock()
	s.cache = newCache
	s.mu.Unlock()

	s.log.Debug("balance cache refreshed", zap.Int("assets", len(newCache)))
}

// GetBalances 返回所有余额非零资产的当前余额。
// 每个条目包含可用（free）和冻结（locked）金额。
func (s *AccountService) GetBalances() (map[string]BalanceInfo, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if len(s.cache) == 0 {
		return nil, apperrors.NewAppError(apperrors.ErrBinanceAPI,
			"balance cache is empty, waiting for initial refresh", "account", nil)
	}

	result := make(map[string]BalanceInfo, len(s.cache))
	for asset, info := range s.cache {
		result[asset] = info
	}
	return result, nil
}

// GetTotalAssetValue 通过使用当前市场价格将所有余额转换为 USDT 来计算总资产价值。
// USDT 余额按 1:1 计算。
func (s *AccountService) GetTotalAssetValue() (decimal.Decimal, error) {
	s.mu.RLock()
	balances := make(map[string]BalanceInfo, len(s.cache))
	for k, v := range s.cache {
		balances[k] = v
	}
	s.mu.RUnlock()

	total := decimal.Zero
	for asset, info := range balances {
		amount := info.Free.Add(info.Locked)
		if amount.IsZero() {
			continue
		}

		if asset == "USDT" {
			total = total.Add(amount)
			continue
		}

		// 获取该资产的 USDT 价格（例如 BTCUSDT）。
		symbol := asset + "USDT"
		price, err := s.market.GetCurrentPrice(symbol)
		if err != nil {
			s.log.Warn("failed to get price for asset, skipping",
				zap.String("asset", asset), zap.Error(err))
			continue
		}

		total = total.Add(amount.Mul(price))
	}

	return total, nil
}

// GetPositionPnL 从所有历史交易中计算指定交易对的已实现盈亏，
// 包含手续费扣除。
// 盈亏 = 卖出金额 - 买入金额 - 总手续费
func (s *AccountService) GetPositionPnL(symbol string) (*PnL, error) {
	if symbol == "" {
		return nil, apperrors.NewAppError(apperrors.ErrValidation,
			"symbol must not be empty", "account", nil)
	}

	trades, err := s.tradeStore.GetByFilter(store.TradeFilter{Symbol: symbol})
	if err != nil {
		return nil, apperrors.NewAppError(apperrors.ErrDatabase,
			"failed to query trades", "account", err)
	}

	pnl := &PnL{
		Symbol:     symbol,
		TradeCount: len(trades),
	}

	for _, t := range trades {
		switch t.Side {
		case "BUY":
			pnl.BuyAmount = pnl.BuyAmount.Add(t.Amount)
		case "SELL":
			pnl.SellAmount = pnl.SellAmount.Add(t.Amount)
		}
		pnl.TotalFees = pnl.TotalFees.Add(t.Fee)
	}

	// 已实现盈亏 = 卖出收入 - 买入成本 - 手续费
	pnl.RealizedPnL = pnl.SellAmount.Sub(pnl.BuyAmount).Sub(pnl.TotalFees)

	return pnl, nil
}

// GetAssetHistory 返回指定时间范围内的历史账户快照。
func (s *AccountService) GetAssetHistory(start, end time.Time) ([]model.AccountSnapshot, error) {
	snapshots, err := s.accountStore.GetByTimeRange(start, end)
	if err != nil {
		return nil, apperrors.NewAppError(apperrors.ErrDatabase,
			"failed to query account snapshots", "account", err)
	}
	return snapshots, nil
}

// GetFeeStats 从所有交易记录中返回累计手续费统计信息。
func (s *AccountService) GetFeeStats() (*FeeStatistics, error) {
	// 获取所有交易（无过滤条件）。
	trades, err := s.tradeStore.GetByFilter(store.TradeFilter{})
	if err != nil {
		return nil, apperrors.NewAppError(apperrors.ErrDatabase,
			"failed to query trades for fee stats", "account", err)
	}

	stats := &FeeStatistics{
		FeeByAsset:  make(map[string]decimal.Decimal),
		FeeBySymbol: make(map[string]decimal.Decimal),
		TradeCount:  len(trades),
	}

	for _, t := range trades {
		stats.TotalFees = stats.TotalFees.Add(t.Fee)

		if t.FeeAsset != "" {
			stats.FeeByAsset[t.FeeAsset] = stats.FeeByAsset[t.FeeAsset].Add(t.Fee)
		}
		stats.FeeBySymbol[t.Symbol] = stats.FeeBySymbol[t.Symbol].Add(t.Fee)
	}

	return stats, nil
}
