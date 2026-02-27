import { useState, useEffect } from 'react'
import { strategyAPI } from '../services/api'

export default function StrategyPanel() {
  const [running, setRunning] = useState(false)
  const [strategies, setStrategies] = useState<Record<string, unknown>[]>([])

  useEffect(() => {
    strategyAPI.status().then((r) => {
      setRunning(r.data.running)
      setStrategies(r.data.strategies || [])
    })
  }, [])

  const toggle = async () => {
    if (running) await strategyAPI.stop()
    else await strategyAPI.start()
    const r = await strategyAPI.status()
    setRunning(r.data.running)
  }

  return (
    <div>
      <h2>自动交易控制</h2>
      <button onClick={toggle} style={{ padding: '8px 24px', marginBottom: 16 }}>
        {running ? '停止自动交易' : '启动自动交易'}
      </button>
      <p>状态: {running ? '运行中' : '已停止'}</p>
      <h3>策略列表</h3>
      <ul>{strategies.map((s, i) => <li key={i}>{String(s.name)} - {String(s.status)}</li>)}</ul>
    </div>
  )
}
