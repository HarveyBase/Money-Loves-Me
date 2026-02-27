package notification

import (
	"fmt"
	"sync"
	"time"

	"money-loves-me/internal/model"
)

// EventType represents the type of notification event.
type EventType string

const (
	EventOrderFilled      EventType = "ORDER_FILLED"
	EventSignalTriggered  EventType = "SIGNAL_TRIGGERED"
	EventRiskAlert        EventType = "RISK_ALERT"
	EventAPIDisconnect    EventType = "API_DISCONNECT"
	EventBacktestComplete EventType = "BACKTEST_COMPLETE"
	EventOptimizeComplete EventType = "OPTIMIZE_COMPLETE"
)

// AllEventTypes returns all defined event types.
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

// NotificationFilter defines filtering criteria for notification queries.
type NotificationFilter struct {
	EventType string
	IsRead    *bool
	Start     time.Time
	End       time.Time
}

// Store defines the persistence interface for notifications.
type Store interface {
	Create(notification *model.Notification) error
	GetByFilter(filter NotificationFilter) ([]model.Notification, error)
	MarkAsRead(id int64) error
}

// WSPusher defines the interface for pushing notifications via WebSocket.
type WSPusher interface {
	PushNotification(notification *model.Notification) error
}

// NotificationService manages notification creation, retrieval, and event filtering.
type NotificationService struct {
	store       Store
	wsPusher    WSPusher
	eventFilter map[EventType]bool
	mu          sync.RWMutex
}

// NewNotificationService creates a new NotificationService.
// All event types are enabled by default.
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

// Send creates a notification, stores it, and pushes via WebSocket
// if the event type passes the filter.
func (s *NotificationService) Send(eventType EventType, title, description string) error {
	if title == "" {
		return fmt.Errorf("notification title must not be empty")
	}

	s.mu.RLock()
	enabled, exists := s.eventFilter[eventType]
	s.mu.RUnlock()

	// If the event type is explicitly disabled, skip silently.
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

	// Push via WebSocket if pusher is available (best-effort).
	if s.wsPusher != nil {
		_ = s.wsPusher.PushNotification(notification)
	}

	return nil
}

// GetNotifications retrieves notifications matching the filter,
// ordered by creation time descending (as guaranteed by the store).
func (s *NotificationService) GetNotifications(filter NotificationFilter) ([]model.Notification, error) {
	return s.store.GetByFilter(filter)
}

// MarkAsRead marks a notification as read by its ID.
func (s *NotificationService) MarkAsRead(id int64) error {
	return s.store.MarkAsRead(id)
}

// SetEventFilter configures which event types are enabled for notifications.
// Events set to true will be delivered; events set to false will be silently dropped by Send.
func (s *NotificationService) SetEventFilter(filters map[EventType]bool) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	for et, enabled := range filters {
		s.eventFilter[et] = enabled
	}
	return nil
}

// GetEventFilter returns a copy of the current event filter configuration.
func (s *NotificationService) GetEventFilter() map[EventType]bool {
	s.mu.RLock()
	defer s.mu.RUnlock()

	result := make(map[EventType]bool, len(s.eventFilter))
	for k, v := range s.eventFilter {
		result[k] = v
	}
	return result
}
