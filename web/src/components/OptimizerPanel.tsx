import { useState, useEffect } from 'react'
import { optimizerAPI } from '../services/api'

export default function OptimizerPanel() {
  const [history, setHistory] = useState<Record<string, unknown>[]>([])

  useEffect(() => { optimizerAPI.history().then((r) => setHistory(r.data.history || [])) }, [])

  return (
    <div>
      <h2>策略优化历史</h2>
      <table style={{ width: '100%' }}><thead><tr>
        <th>策略</th><th>旧参数</th><th>新参数</th><th>是否应用</th><th>分析结论</th><th>时间</th>
      </tr></thead><tbody>
        {history.map((h, i) => (
          <tr key={i}><td>{String(h.strategy_name)}</td>
            <td>{JSON.stringify(h.old_params)}</td><td>{JSON.stringify(h.new_params)}</td>
            <td>{h.applied ? '是' : '否'}</td><td>{String(h.analysis_notes)}</td>
            <td>{String(h.created_at)}</td></tr>
        ))}
      </tbody></table>
    </div>
  )
}
