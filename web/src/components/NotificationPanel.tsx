import { useState, useEffect } from 'react'
import { notificationAPI } from '../services/api'
import wsClient from '../services/websocket'

export default function NotificationPanel() {
  const [notifications, setNotifications] = useState<Record<string, unknown>[]>([])

  useEffect(() => {
    notificationAPI.list().then((r) => setNotifications(r.data.notifications || []))
    const handler = () => {
      notificationAPI.list().then((r) => setNotifications(r.data.notifications || []))
    }
    wsClient.on('notification', handler)
    return () => wsClient.off('notification', handler)
  }, [])

  const markRead = async (id: string) => {
    await notificationAPI.markRead(id)
    notificationAPI.list().then((r) => setNotifications(r.data.notifications || []))
  }

  return (
    <div>
      <h2>通知消息</h2>
      <ul style={{ listStyle: 'none', padding: 0 }}>
        {notifications.map((n, i) => (
          <li key={i} style={{ padding: 8, borderBottom: '1px solid #eee', opacity: n.is_read ? 0.6 : 1 }}>
            <strong>[{String(n.event_type)}]</strong> {String(n.title)} - {String(n.description)}
            {!n.is_read && <button onClick={() => markRead(String(n.id))} style={{ marginLeft: 8 }}>标记已读</button>}
          </li>
        ))}
      </ul>
    </div>
  )
}
