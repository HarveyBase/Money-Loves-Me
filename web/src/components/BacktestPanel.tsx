import { useState, useEffect } from 'react'
import { backtestAPI } from '../services/api'

export default function BacktestPanel() {
  const [results, setResults] = useState<Record<string, unknown>[]>([])

  useEffect(() => { backtestAPI.results().then((r) => setResults(r.data.results || [])) }, [])

  return (
    <div>
      <h2>回测结果</h2>
      <table style={{ width: '100%' }}><thead><tr>
        <th>策略</th><th>交易对</th><th>总收益率</th><th>净收益</th><th>最大回撤</th><th>胜率</th><th>总手续费</th>
      </tr></thead><tbody>
        {results.map((r, i) => (
          <tr key={i}><td>{String(r.strategy_name)}</td><td>{String(r.symbol)}</td>
            <td>{String(r.total_return)}</td><td>{String(r.net_profit)}</td>
            <td>{String(r.max_drawdown)}</td><td>{String(r.win_rate)}</td><td>{String(r.total_fees)}</td></tr>
        ))}
      </tbody></table>
    </div>
  )
}
