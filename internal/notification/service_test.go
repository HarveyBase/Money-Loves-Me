package notification

import (
	"fmt"
	"sort"
	"sync"
	"testing"
	"time"

	"money-loves-me/internal/model"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"pgregory.net/rapid"
)

// --- 模拟存储 ---

type mockStore struct {
	mu            sync.Mutex
	notifications []model.Notification
	nextID        int64
	createErr     error
	filterErr     error
	markReadErr   error
}

func newMockStore() *mockStore {
	return &mockStore{nextID: 1}
}

func (m *mockStore) Create(n *model.Notification) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.createErr != nil {
		return m.createErr
	}
	n.ID = m.nextID
	m.nextID++
	m.notifications = append(m.notifications, *n)
	return nil
}

func (m *mockStore) GetByFilter(filter NotificationFilter) ([]model.Notification, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.filterErr != nil {
		return nil, m.filterErr
	}
	var result []model.Notification
	for _, n := range m.notifications {
		if filter.EventType != "" && n.EventType != filter.EventType {
			continue
		}
		if filter.IsRead != nil && n.IsRead != *filter.IsRead {
			continue
		}
		if !filter.Start.IsZero() && n.CreatedAt.Before(filter.Start) {
			continue
		}
		if !filter.End.IsZero() && n.CreatedAt.After(filter.End) {
			continue
		}
		result = append(result, n)
	}
	// 按时间倒序返回。
	sort.Slice(result, func(i, j int) bool {
		return result[i].CreatedAt.After(result[j].CreatedAt)
	})
	return result, nil
}

func (m *mockStore) MarkAsRead(id int64) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.markReadErr != nil {
		return m.markReadErr
	}
	for i := range m.notifications {
		if m.notifications[i].ID == id {
			m.notifications[i].IsRead = true
			return nil
		}
	}
	return fmt.Errorf("notification %d not found", id)
}

// --- 模拟 WebSocket 推送器 ---

type mockWSPusher struct {
	mu      sync.Mutex
	pushed  []*model.Notification
	pushErr error
}

func newMockWSPusher() *mockWSPusher {
	return &mockWSPusher{}
}

func (m *mockWSPusher) PushNotification(n *model.Notification) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.pushErr != nil {
		return m.pushErr
	}
	m.pushed = append(m.pushed, n)
	return nil
}

// --- 测试 ---

func TestNewNotificationService_AllEventsEnabledByDefault(t *testing.T) {
	svc := NewNotificationService(newMockStore(), nil)
	filter := svc.GetEventFilter()

	for _, et := range AllEventTypes() {
		assert.True(t, filter[et], "event type %s should be enabled by default", et)
	}
}

func TestSend_StoresAndPushesNotification(t *testing.T) {
	store := newMockStore()
	ws := newMockWSPusher()
	svc := NewNotificationService(store, ws)

	err := svc.Send(EventOrderFilled, "Order Filled", "BTC/USDT order filled at 50000")
	require.NoError(t, err)

	// 验证已存储。
	assert.Len(t, store.notifications, 1)
	n := store.notifications[0]
	assert.Equal(t, string(EventOrderFilled), n.EventType)
	assert.Equal(t, "Order Filled", n.Title)
	assert.Equal(t, "BTC/USDT order filled at 50000", *n.Description)
	assert.False(t, n.IsRead)
	assert.False(t, n.CreatedAt.IsZero())

	// 验证已通过 WebSocket 推送。
	assert.Len(t, ws.pushed, 1)
}

func TestSend_EmptyTitleReturnsError(t *testing.T) {
	svc := NewNotificationService(newMockStore(), nil)
	err := svc.Send(EventRiskAlert, "", "some description")
	assert.Error(t, err)
}

func TestSend_DisabledEventTypeSkipped(t *testing.T) {
	store := newMockStore()
	ws := newMockWSPusher()
	svc := NewNotificationService(store, ws)

	// 禁用 ORDER_FILLED 事件。
	err := svc.SetEventFilter(map[EventType]bool{EventOrderFilled: false})
	require.NoError(t, err)

	err = svc.Send(EventOrderFilled, "Order Filled", "should be skipped")
	require.NoError(t, err)

	assert.Empty(t, store.notifications)
	assert.Empty(t, ws.pushed)
}

func TestSend_StoreErrorPropagated(t *testing.T) {
	store := newMockStore()
	store.createErr = fmt.Errorf("db connection lost")
	svc := NewNotificationService(store, nil)

	err := svc.Send(EventRiskAlert, "Alert", "desc")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to store notification")
}

func TestSend_WSPushErrorDoesNotFail(t *testing.T) {
	store := newMockStore()
	ws := newMockWSPusher()
	ws.pushErr = fmt.Errorf("ws connection closed")
	svc := NewNotificationService(store, ws)

	err := svc.Send(EventRiskAlert, "Alert", "desc")
	require.NoError(t, err)
	assert.Len(t, store.notifications, 1)
}

func TestSend_NilWSPusherDoesNotPanic(t *testing.T) {
	svc := NewNotificationService(newMockStore(), nil)
	err := svc.Send(EventBacktestComplete, "Done", "backtest finished")
	require.NoError(t, err)
}

func TestGetNotifications_ReturnsInReverseChronologicalOrder(t *testing.T) {
	store := newMockStore()
	svc := NewNotificationService(store, nil)

	base := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)
	for i := 0; i < 5; i++ {
		n := &model.Notification{
			EventType: string(EventOrderFilled),
			Title:     fmt.Sprintf("Notification %d", i),
			CreatedAt: base.Add(time.Duration(i) * time.Hour),
		}
		require.NoError(t, store.Create(n))
	}

	results, err := svc.GetNotifications(NotificationFilter{})
	require.NoError(t, err)
	require.Len(t, results, 5)

	// 验证按时间倒序排列。
	for i := 1; i < len(results); i++ {
		assert.True(t, results[i-1].CreatedAt.After(results[i].CreatedAt) ||
			results[i-1].CreatedAt.Equal(results[i].CreatedAt))
	}
}

func TestGetNotifications_FilterByEventType(t *testing.T) {
	store := newMockStore()
	svc := NewNotificationService(store, nil)

	require.NoError(t, store.Create(&model.Notification{EventType: string(EventOrderFilled), Title: "a", CreatedAt: time.Now()}))
	require.NoError(t, store.Create(&model.Notification{EventType: string(EventRiskAlert), Title: "b", CreatedAt: time.Now()}))
	require.NoError(t, store.Create(&model.Notification{EventType: string(EventOrderFilled), Title: "c", CreatedAt: time.Now()}))

	results, err := svc.GetNotifications(NotificationFilter{EventType: string(EventOrderFilled)})
	require.NoError(t, err)
	assert.Len(t, results, 2)
	for _, n := range results {
		assert.Equal(t, string(EventOrderFilled), n.EventType)
	}
}

func TestMarkAsRead(t *testing.T) {
	store := newMockStore()
	svc := NewNotificationService(store, nil)

	require.NoError(t, store.Create(&model.Notification{EventType: string(EventRiskAlert), Title: "alert", CreatedAt: time.Now()}))
	assert.False(t, store.notifications[0].IsRead)

	err := svc.MarkAsRead(1)
	require.NoError(t, err)
	assert.True(t, store.notifications[0].IsRead)
}

func TestSetEventFilter_PartialUpdate(t *testing.T) {
	svc := NewNotificationService(newMockStore(), nil)

	// 禁用一个事件类型。
	err := svc.SetEventFilter(map[EventType]bool{EventRiskAlert: false})
	require.NoError(t, err)

	filter := svc.GetEventFilter()
	assert.False(t, filter[EventRiskAlert])
	// 其他事件类型保持启用。
	assert.True(t, filter[EventOrderFilled])
	assert.True(t, filter[EventSignalTriggered])
}

func TestAllEventTypes_ContainsSixTypes(t *testing.T) {
	types := AllEventTypes()
	assert.Len(t, types, 6)
	expected := map[EventType]bool{
		EventOrderFilled:      true,
		EventSignalTriggered:  true,
		EventRiskAlert:        true,
		EventAPIDisconnect:    true,
		EventBacktestComplete: true,
		EventOptimizeComplete: true,
	}
	for _, et := range types {
		assert.True(t, expected[et], "unexpected event type: %s", et)
	}
}

func TestSend_EachEventTypeContainsTimestampAndDescription(t *testing.T) {
	store := newMockStore()
	svc := NewNotificationService(store, nil)

	for _, et := range AllEventTypes() {
		err := svc.Send(et, "Title for "+string(et), "Description for "+string(et))
		require.NoError(t, err)
	}

	assert.Len(t, store.notifications, 6)
	for _, n := range store.notifications {
		assert.NotEmpty(t, n.EventType)
		assert.NotEmpty(t, n.Title)
		assert.NotNil(t, n.Description)
		assert.NotEmpty(t, *n.Description)
		assert.False(t, n.CreatedAt.IsZero())
	}
}

// --- 属性测试 ---

// Feature: binance-trading-system, Property 17: 通知时间倒序和事件过滤
// Validates: Requirements 8.4, 8.5
func TestProperty17_NotificationOrderAndEventFilter(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		store := newMockStore()
		svc := NewNotificationService(store, nil)

		allTypes := AllEventTypes()

		// 生成随机数量的通知（2..20），具有不同的时间戳和事件类型。
		count := rapid.IntRange(2, 20).Draw(t, "count")
		baseTime := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)

		for i := 0; i < count; i++ {
			// 随机偏移秒数以创建不同的时间戳。
			offsetSec := rapid.IntRange(0, 100000).Draw(t, fmt.Sprintf("offset_%d", i))
			eventIdx := rapid.IntRange(0, len(allTypes)-1).Draw(t, fmt.Sprintf("eventIdx_%d", i))
			et := allTypes[eventIdx]

			n := &model.Notification{
				EventType: string(et),
				Title:     fmt.Sprintf("Notification %d", i),
				CreatedAt: baseTime.Add(time.Duration(offsetSec) * time.Second),
			}
			if err := store.Create(n); err != nil {
				t.Fatalf("failed to create notification: %v", err)
			}
		}

		// 第一部分：验证 GetNotifications 返回严格按时间倒序排列的结果。
		results, err := svc.GetNotifications(NotificationFilter{})
		if err != nil {
			t.Fatalf("GetNotifications failed: %v", err)
		}
		if len(results) != count {
			t.Fatalf("expected %d notifications, got %d", count, len(results))
		}

		for i := 1; i < len(results); i++ {
			if results[i-1].CreatedAt.Before(results[i].CreatedAt) {
				t.Fatalf("notifications not in reverse chronological order: index %d (%v) < index %d (%v)",
					i-1, results[i-1].CreatedAt, i, results[i].CreatedAt)
			}
		}

		// 第二部分：随机选择一个事件类型进行过滤，验证只返回匹配的通知。
		filterIdx := rapid.IntRange(0, len(allTypes)-1).Draw(t, "filterEventIdx")
		filterType := allTypes[filterIdx]

		filtered, err := svc.GetNotifications(NotificationFilter{EventType: string(filterType)})
		if err != nil {
			t.Fatalf("GetNotifications with filter failed: %v", err)
		}

		// 所有返回的通知必须匹配过滤的事件类型。
		for _, n := range filtered {
			if n.EventType != string(filterType) {
				t.Fatalf("filtered results should only contain event type %s, got %s", filterType, n.EventType)
			}
		}

		// 过滤后的结果也必须按时间倒序排列。
		for i := 1; i < len(filtered); i++ {
			if filtered[i-1].CreatedAt.Before(filtered[i].CreatedAt) {
				t.Fatalf("filtered notifications not in reverse chronological order: index %d (%v) < index %d (%v)",
					i-1, filtered[i-1].CreatedAt, i, filtered[i].CreatedAt)
			}
		}

		// 过滤结果的数量必须与存储中该事件类型的实际数量一致。
		expectedCount := 0
		for _, n := range store.notifications {
			if n.EventType == string(filterType) {
				expectedCount++
			}
		}
		if len(filtered) != expectedCount {
			t.Fatalf("expected %d notifications of type %s, got %d", expectedCount, filterType, len(filtered))
		}
	})
}

// Feature: binance-trading-system, Property 18: 事件触发通知生成
// Validates: Requirements 8.1, 8.3
func TestProperty18_EventTriggersNotificationGeneration(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		store := newMockStore()
		svc := NewNotificationService(store, nil)

		allTypes := AllEventTypes()

		// 从所有已定义的类型中随机选择一个事件类型。
		eventIdx := rapid.IntRange(0, len(allTypes)-1).Draw(t, "eventIdx")
		eventType := allTypes[eventIdx]

		// 生成随机的非空标题和描述。
		title := rapid.StringMatching(`[A-Za-z][A-Za-z0-9 ]{0,49}`).Draw(t, "title")
		description := rapid.StringMatching(`[A-Za-z][A-Za-z0-9 .,!]{0,99}`).Draw(t, "description")

		beforeSend := time.Now()

		// 通过调用 Send 触发事件。
		err := svc.Send(eventType, title, description)
		if err != nil {
			t.Fatalf("Send failed: %v", err)
		}

		// 验证恰好存储了一条通知。
		if len(store.notifications) != 1 {
			t.Fatalf("expected 1 notification, got %d", len(store.notifications))
		}

		n := store.notifications[0]

		// 验证通知具有非零时间戳。
		if n.CreatedAt.IsZero() {
			t.Fatal("notification timestamp must not be zero")
		}

		// 验证时间戳合理（不早于调用 Send 的时间）。
		if n.CreatedAt.Before(beforeSend.Add(-time.Second)) {
			t.Fatalf("notification timestamp %v is before send time %v", n.CreatedAt, beforeSend)
		}

		// 验证事件类型匹配。
		if n.EventType != string(eventType) {
			t.Fatalf("expected event type %s, got %s", eventType, n.EventType)
		}

		// 验证描述非空。
		if n.Description == nil || *n.Description == "" {
			t.Fatal("notification description must not be empty")
		}

		// 验证描述与发送的内容一致。
		if *n.Description != description {
			t.Fatalf("expected description %q, got %q", description, *n.Description)
		}

		// 验证标题匹配。
		if n.Title != title {
			t.Fatalf("expected title %q, got %q", title, n.Title)
		}
	})
}
