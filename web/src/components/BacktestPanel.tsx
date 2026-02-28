import { useState, useEffect } from 'react'
import { backtestAPI } from '../services/api'

export default function BacktestPanel() {
  const [results, setResults] = useState<Record<string, unknown>[]>([])

  useEffect(() => { backtestAPI.results().then((r) => setResults(r.data.results || [])) }, [])

  return (
    <div className="card">
      <div className="card-header">
        <span className="card-title">回测结果</span>
      </div>
      {results.length === 0 ? (
        <div className="empty-state">
          <div className="icon">📊</div>
          <p>暂无回测数据</p>
        </div>
      ) : (
        <div className="table-wrap">
          <table>
            <thead>
              <tr>
                <th>策略</th><th>交易对</th><th>总收益率</th><th>净收益</th>
                <th>最大回撤</th><th>胜率</th><th>手续费</th>
              </tr>
            </thead>
            <tbody>
              {results.map((r, i) => {
                const ret = parseFloat(String(r.total_return || '0'))
                return (
                  <tr key={i}>
                    <td style={{ color: 'var(--text-primary)', fontWeight: 600 }}>{String(r.strategy_name)}</td>
                    <td>{String(r.symbol)}</td>
                    <td className={ret >= 0 ? 'text-green' : 'text-red'}>
                      {ret >= 0 ? '+' : ''}{String(r.total_return)}%
                    </td>
                    <td className={parseFloat(String(r.net_profit || '0')) >= 0 ? 'text-green' : 'text-red'}>
                      {String(r.net_profit)}
                    </td>
                    <td className="text-red">{String(r.max_drawdown)}%</td>
                    <td>{String(r.win_rate)}%</td>
                    <td>{String(r.total_fees)}</td>
                  </tr>
                )
              })}
            </tbody>
          </table>
        </div>
      )}
    </div>
  )
}
