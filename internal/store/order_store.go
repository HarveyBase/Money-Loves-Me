package store

import (
	"money-loves-me/internal/model"
	"time"

	"gorm.io/gorm"
)

// OrderStore handles CRUD operations for orders.
type OrderStore struct {
	*Store
}

// NewOrderStore creates a new OrderStore.
func NewOrderStore(db *gorm.DB) *OrderStore {
	return &OrderStore{Store: NewStore(db)}
}

// Create inserts a new order record.
func (s *OrderStore) Create(order *model.Order) error {
	return s.withRetry(func() error {
		return s.db.Create(order).Error
	})
}

// GetByID retrieves an order by its primary key.
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

// Update saves changes to an existing order.
func (s *OrderStore) Update(order *model.Order) error {
	return s.withRetry(func() error {
		return s.db.Save(order).Error
	})
}

// OrderFilter defines filtering criteria for order queries.
type OrderFilter struct {
	Symbol string
	Status string
	Start  time.Time
	End    time.Time
}

// GetByFilter retrieves orders matching the given filter criteria.
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
