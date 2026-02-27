package notification

import (
	"fmt"
	"sync"
	"time"

	"money-loves-me/internal/model"
)

// EventType 表示通知事件的类型。
type EventType string

const (
	EventOrderFilled      EventType = "ORDER_FILLED"
	EventSignalTriggered  EventType = "SIGNAL_TRIGGERED"
	EventRiskAlert        EventType = "RISK_ALERT"
	EventAPIDisconnect    EventType = "API_DISCONNECT"
	EventBacktestComplete EventType = "BACKTEST_COMPLETE"
	EventOptimizeComplete EventType = "OPTIMIZE_COMPLETE"
)

// AllEventTypes 返回所有已定义的事件类型。
func AllEventTypes() []EventType {
	return []EventType{
		EventOrderFilled,
		EventSignalTriggered,
		EventRiskAlert,
		EventAPIDisconnect,
		EventBacktestComplete,
		EventOptimizeComplete,
	}
}

// NotificationFilter 定义通知查询的过滤条件。
type NotificationFilter struct {
	EventType string
	IsRead    *bool
	Start     time.Time
	End       time.Time
}

// Store 定义通知的持久化接口。
type Store interface {
	Create(notification *model.Notification) error
	GetByFilter(filter NotificationFilter) ([]model.Notification, error)
	MarkAsRead(id int64) error
}

// WSPusher 定义通过 WebSocket 推送通知的接口。
type WSPusher interface {
	PushNotification(notification *model.Notification) error
}

// NotificationService 管理通知的创建、检索和事件过滤。
type NotificationService struct {
	store       Store
	wsPusher    WSPusher
	eventFilter map[EventType]bool
	mu          sync.RWMutex
}

// NewNotificationService 创建一个新的 NotificationService。
// 默认启用所有事件类型。
func NewNotificationService(store Store, wsPusher WSPusher) *NotificationService {
	filter := make(map[EventType]bool)
	for _, et := range AllEventTypes() {
		filter[et] = true
	}
	return &NotificationService{
		store:       store,
		wsPusher:    wsPusher,
		eventFilter: filter,
	}
}

// Send 创建一条通知，将其存储，并在事件类型通过过滤器时
// 通过 WebSocket 推送。
func (s *NotificationService) Send(eventType EventType, title, description string) error {
	if title == "" {
		return fmt.Errorf("notification title must not be empty")
	}

	s.mu.RLock()
	enabled, exists := s.eventFilter[eventType]
	s.mu.RUnlock()

	// 如果事件类型被明确禁用，则静默跳过。
	if exists && !enabled {
		return nil
	}

	desc := description
	notification := &model.Notification{
		EventType:   string(eventType),
		Title:       title,
		Description: &desc,
		IsRead:      false,
		CreatedAt:   time.Now(),
	}

	if err := s.store.Create(notification); err != nil {
		return fmt.Errorf("failed to store notification: %w", err)
	}

	// 如果推送器可用，则通过 WebSocket 推送（尽力而为）。
	if s.wsPusher != nil {
		_ = s.wsPusher.PushNotification(notification)
	}

	return nil
}

// GetNotifications 检索匹配过滤条件的通知，
// 按创建时间降序排列（由存储层保证）。
func (s *NotificationService) GetNotifications(filter NotificationFilter) ([]model.Notification, error) {
	return s.store.GetByFilter(filter)
}

// MarkAsRead 通过 ID 将通知标记为已读。
func (s *NotificationService) MarkAsRead(id int64) error {
	return s.store.MarkAsRead(id)
}

// SetEventFilter 配置哪些事件类型启用通知。
// 设置为 true 的事件将被投递；设置为 false 的事件将被 Send 静默丢弃。
func (s *NotificationService) SetEventFilter(filters map[EventType]bool) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	for et, enabled := range filters {
		s.eventFilter[et] = enabled
	}
	return nil
}

// GetEventFilter 返回当前事件过滤器配置的副本。
func (s *NotificationService) GetEventFilter() map[EventType]bool {
	s.mu.RLock()
	defer s.mu.RUnlock()

	result := make(map[EventType]bool, len(s.eventFilter))
	for k, v := range s.eventFilter {
		result[k] = v
	}
	return result
}
