package store

import (
	"money-loves-me/internal/model"
	"time"

	"gorm.io/gorm"
)

// OrderStore 处理订单的 CRUD 操作。
type OrderStore struct {
	*Store
}

// NewOrderStore 创建一个新的 OrderStore。
func NewOrderStore(db *gorm.DB) *OrderStore {
	return &OrderStore{Store: NewStore(db)}
}

// Create 插入一条新的订单记录。
func (s *OrderStore) Create(order *model.Order) error {
	return s.withRetry(func() error {
		return s.db.Create(order).Error
	})
}

// GetByID 根据主键获取订单。
func (s *OrderStore) GetByID(id int64) (*model.Order, error) {
	var order model.Order
	err := s.withRetry(func() error {
		return s.db.First(&order, id).Error
	})
	if err != nil {
		return nil, err
	}
	return &order, nil
}

// Update 保存对现有订单的更改。
func (s *OrderStore) Update(order *model.Order) error {
	return s.withRetry(func() error {
		return s.db.Save(order).Error
	})
}

// OrderFilter 定义订单查询的过滤条件。
type OrderFilter struct {
	Symbol string
	Status string
	Start  time.Time
	End    time.Time
}

// GetByFilter 获取符合给定过滤条件的订单。
func (s *OrderStore) GetByFilter(filter OrderFilter) ([]model.Order, error) {
	var orders []model.Order
	err := s.withRetry(func() error {
		query := s.db.Model(&model.Order{})
		if filter.Symbol != "" {
			query = query.Where("symbol = ?", filter.Symbol)
		}
		if filter.Status != "" {
			query = query.Where("status = ?", filter.Status)
		}
		query = TimeRangeQuery(query, "created_at", filter.Start, filter.End)
		return query.Order("created_at DESC").Find(&orders).Error
	})
	if err != nil {
		return nil, err
	}
	return orders, nil
}
