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
	// cacheRefreshInterval is the interval for periodic balance cache refresh.
	cacheRefreshInterval = 5 * time.Second
)

// BalanceInfo holds the free and locked amounts for a single asset.
type BalanceInfo struct {
	Free   decimal.Decimal
	Locked decimal.Decimal
}

// PnL holds the profit and loss information for a trading pair.
type PnL struct {
	Symbol      string
	RealizedPnL decimal.Decimal // realized profit/loss (sells - buys - fees)
	TotalFees   decimal.Decimal // total fees paid
	BuyAmount   decimal.Decimal // total buy cost
	SellAmount  decimal.Decimal // total sell revenue
	TradeCount  int
}

// FeeStatistics holds cumulative fee statistics.
type FeeStatistics struct {
	TotalFees   decimal.Decimal            // total fees across all trades
	FeeByAsset  map[string]decimal.Decimal // fees grouped by fee asset
	FeeBySymbol map[string]decimal.Decimal // fees grouped by trading pair
	TradeCount  int
}

// MarketPricer provides current market prices for assets.
type MarketPricer interface {
	GetCurrentPrice(symbol string) (decimal.Decimal, error)
}

// BinanceAccountClient defines the subset of BinanceClient used by AccountService.
type BinanceAccountClient interface {
	GetAccountInfo() (*binance.AccountInfo, error)
}

// AccountService manages account balances, asset valuation, PnL, and fee statistics.
type AccountService struct {
	client       BinanceAccountClient
	market       MarketPricer
	tradeStore   *store.TradeStore
	accountStore *store.AccountStore
	log          *logger.Logger

	mu    sync.RWMutex
	cache map[string]BalanceInfo // asset -> balance info

	cancelRefresh context.CancelFunc
}

// NewAccountService creates a new AccountService and starts the background
// cache refresh goroutine.
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

// Close stops the background cache refresh goroutine.
func (s *AccountService) Close() {
	s.cancelRefresh()
}

// refreshLoop periodically refreshes the balance cache from the Binance API.
func (s *AccountService) refreshLoop(ctx context.Context) {
	// Do an initial fetch immediately.
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

// refreshCache fetches account info from Binance and updates the local cache.
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
		// Only cache assets with non-zero balances.
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

// GetBalances returns the current balances for all assets with non-zero amounts.
// Each entry contains the free (available) and locked (frozen) amounts.
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

// GetTotalAssetValue calculates the total asset value in USDT by converting
// all balances using current market prices. USDT balances are counted at 1:1.
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

		// Get the USDT price for this asset (e.g., BTCUSDT).
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

// GetPositionPnL calculates the realized profit/loss for a given trading pair
// from all historical trades, with fee deduction.
// PnL = SellAmount - BuyAmount - TotalFees
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

	// Realized PnL = Sell revenue - Buy cost - Fees
	pnl.RealizedPnL = pnl.SellAmount.Sub(pnl.BuyAmount).Sub(pnl.TotalFees)

	return pnl, nil
}

// GetAssetHistory returns historical account snapshots within the given time range.
func (s *AccountService) GetAssetHistory(start, end time.Time) ([]model.AccountSnapshot, error) {
	snapshots, err := s.accountStore.GetByTimeRange(start, end)
	if err != nil {
		return nil, apperrors.NewAppError(apperrors.ErrDatabase,
			"failed to query account snapshots", "account", err)
	}
	return snapshots, nil
}

// GetFeeStats returns cumulative fee statistics from all trade records.
func (s *AccountService) GetFeeStats() (*FeeStatistics, error) {
	// Fetch all trades (no filter).
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
