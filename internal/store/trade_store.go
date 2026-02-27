package store

import (
	"time"

	"money-loves-me/internal/model"

	"gorm.io/gorm"
)

// TradeStore 处理交易记录的 CRUD 操作。
type TradeStore struct {
	*Store
}

// NewTradeStore 创建一个新的 TradeStore。
func NewTradeStore(db *gorm.DB) *TradeStore {
	return &TradeStore{Store: NewStore(db)}
}

// Create 插入一条新的交易记录。
func (s *TradeStore) Create(trade *model.Trade) error {
	return s.withRetry(func() error {
		return s.db.Create(trade).Error
	})
}

// GetByOrderID 获取指定订单 ID 的所有交易记录。
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

// TradeFilter 定义交易查询的过滤条件。
type TradeFilter struct {
	Symbol       string
	StrategyName string
	Start        time.Time
	End          time.Time
}

// GetByFilter 获取符合给定过滤条件的交易记录。
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
