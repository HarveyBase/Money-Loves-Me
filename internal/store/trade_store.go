package store

import (
	"time"

	"money-loves-me/internal/model"

	"gorm.io/gorm"
)

// TradeStore handles CRUD operations for trades.
type TradeStore struct {
	*Store
}

// NewTradeStore creates a new TradeStore.
func NewTradeStore(db *gorm.DB) *TradeStore {
	return &TradeStore{Store: NewStore(db)}
}

// Create inserts a new trade record.
func (s *TradeStore) Create(trade *model.Trade) error {
	return s.withRetry(func() error {
		return s.db.Create(trade).Error
	})
}

// GetByOrderID retrieves all trades for a given order ID.
func (s *TradeStore) GetByOrderID(orderID int64) ([]model.Trade, error) {
	var trades []model.Trade
	err := s.withRetry(func() error {
		return s.db.Where("order_id = ?", orderID).Order("executed_at DESC").Find(&trades).Error
	})
	if err != nil {
		return nil, err
	}
	return trades, nil
}

// TradeFilter defines filtering criteria for trade queries.
type TradeFilter struct {
	Symbol       string
	StrategyName string
	Start        time.Time
	End          time.Time
}

// GetByFilter retrieves trades matching the given filter criteria.
func (s *TradeStore) GetByFilter(filter TradeFilter) ([]model.Trade, error) {
	var trades []model.Trade
	err := s.withRetry(func() error {
		query := s.db.Model(&model.Trade{})
		if filter.Symbol != "" {
			query = query.Where("symbol = ?", filter.Symbol)
		}
		if filter.StrategyName != "" {
			query = query.Where("strategy_name = ?", filter.StrategyName)
		}
		query = TimeRangeQuery(query, "executed_at", filter.Start, filter.End)
		return query.Order("executed_at DESC").Find(&trades).Error
	})
	if err != nil {
		return nil, err
	}
	return trades, nil
}
