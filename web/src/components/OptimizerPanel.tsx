import { useState, useEffect } from 'react'
import { optimizerAPI } from '../services/api'

export default function OptimizerPanel() {
  const [history, setHistory] = useState<Record<string, unknown>[]>([])

  useEffect(() => { optimizerAPI.history().then((r) => setHistory(r.data.history || [])) }, [])

  return (
    <div className="card">
      <div className="card-header">
        <span className="card-title">策略优化历史</span>
      </div>
      {history.length === 0 ? (
        <div className="empty-state">
          <div className="icon">⚙️</div>
          <p>暂无优化记录</p>
        </div>
      ) : (
        <div className="table-wrap">
          <table>
            <thead>
              <tr>
                <th>策略</th><th>旧参数</th><th>新参数</th><th>状态</th><th>分析结论</th><th>时间</th>
              </tr>
            </thead>
            <tbody>
              {history.map((h, i) => (
                <tr key={i}>
                  <td style={{ color: 'var(--text-primary)', fontWeight: 600 }}>{String(h.strategy_name)}</td>
                  <td style={{ fontFamily: 'monospace', fontSize: 12 }}>{JSON.stringify(h.old_params)}</td>
                  <td style={{ fontFamily: 'monospace', fontSize: 12 }}>{JSON.stringify(h.new_params)}</td>
                  <td>
                    <span className={`badge ${h.applied ? 'badge-green' : 'badge-muted'}`}>
                      {h.applied ? '已应用' : '未应用'}
                    </span>
                  </td>
                  <td style={{ maxWidth: 200, overflow: 'hidden', textOverflow: 'ellipsis' }}>
                    {String(h.analysis_notes || '-')}
                  </td>
                  <td>{String(h.created_at)}</td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      )}
    </div>
  )
}
