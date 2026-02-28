import { useState, useEffect } from 'react'
import { notificationAPI } from '../services/api'
import wsClient from '../services/websocket'

export default function NotificationPanel() {
  const [notifications, setNotifications] = useState<Record<string, unknown>[]>([])

  const load = () => notificationAPI.list().then((r) => setNotifications(r.data.notifications || []))

  useEffect(() => {
    load()
    const handler = () => load()
    wsClient.on('notification', handler)
    return () => wsClient.off('notification', handler)
  }, [])

  const markRead = async (id: string) => {
    await notificationAPI.markRead(id)
    load()
  }

  return (
    <div className="card">
      <div className="card-header">
        <span className="card-title">通知消息</span>
      </div>
      {notifications.length === 0 ? (
        <div className="empty-state">
          <div className="icon">🔔</div>
          <p>暂无通知消息</p>
        </div>
      ) : (
        <div>
          {notifications.map((n, i) => (
            <div key={i} className={`notification-item ${n.is_read ? '' : 'unread'}`}>
              <div className="notification-content">
                <div className="notification-type">{String(n.event_type)}</div>
                <div className="notification-title">{String(n.title)}</div>
                <div className="notification-desc">{String(n.description)}</div>
              </div>
              {!n.is_read && (
                <button className="btn btn-ghost btn-sm" onClick={() => markRead(String(n.id))}>
                  标记已读
                </button>
              )}
            </div>
          ))}
        </div>
      )}
    </div>
  )
}
