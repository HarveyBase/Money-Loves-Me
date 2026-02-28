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
    setStrategies(r.data.strategies || [])
  }

  return (
    <>
      <div className="strategy-status">
        <span className={`status-dot ${running ? 'running' : 'stopped'}`} />
        <span style={{ flex: 1, fontWeight: 600 }}>
          自动交易 {running ? '运行中' : '已停止'}
        </span>
        <button className={`btn btn-sm ${running ? 'btn-red' : 'btn-green'}`} onClick={toggle}>
          {running ? '停止交易' : '启动交易'}
        </button>
      </div>

      <div className="card">
        <div className="card-header">
          <span className="card-title">策略列表</span>
        </div>
        {strategies.length === 0 ? (
          <div className="empty-state">
            <div className="icon">🤖</div>
            <p>暂无策略配置</p>
          </div>
        ) : (
          <div className="table-wrap">
            <table>
              <thead>
                <tr><th>策略名称</th><th>状态</th><th>交易对</th></tr>
              </thead>
              <tbody>
                {strategies.map((s, i) => (
                  <tr key={i}>
                    <td style={{ color: 'var(--text-primary)', fontWeight: 600 }}>{String(s.name)}</td>
                    <td>
                      <span className={`badge ${String(s.status) === 'running' ? 'badge-green' : 'badge-muted'}`}>
                        {String(s.status)}
                      </span>
                    </td>
                    <td>{String(s.symbol || '-')}</td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        )}
      </div>
    </>
  )
}
