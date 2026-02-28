import { useState, useEffect } from 'react'
import { tradeAPI } from '../services/api'

export default function TradePanel() {
  const [trades, setTrades] = useState<Record<string, unknown>[]>([])
  const [symbol, setSymbol] = useState('')
  const [strategy, setStrategy] = useState('')

  const load = () => {
    const params: Record<string, string> = {}
    if (symbol) params.symbol = symbol
    if (strategy) params.strategy = strategy
    tradeAPI.list(params).then((r) => setTrades(r.data.trades || []))
  }

  useEffect(load, [])

  return (
    <div className="card">
      <div className="card-header">
        <span className="card-title">交易记录</span>
      </div>

      <div className="form-row">
        <input className="form-input-sm" placeholder="交易对筛选" value={symbol}
          onChange={(e) => setSymbol(e.target.value)} />
        <input className="form-input-sm" placeholder="策略名称筛选" value={strategy}
          onChange={(e) => setStrategy(e.target.value)} />
        <button className="btn btn-ghost btn-sm" onClick={load}>查询</button>
      </div>

      <div className="table-wrap">
        <table>
          <thead>
            <tr>
              <th>时间</th><th>交易对</th><th>方向</th><th>价格</th>
              <th>数量</th><th>金额</th><th>手续费</th><th>策略</th>
            </tr>
          </thead>
          <tbody>
            {trades.length === 0 ? (
              <tr><td colSpan={8} style={{ textAlign: 'center', padding: 32, color: 'var(--text-muted)' }}>暂无交易记录</td></tr>
            ) : trades.map((t, i) => (
              <tr key={i}>
                <td>{String(t.executed_at)}</td>
                <td style={{ color: 'var(--text-primary)', fontWeight: 600 }}>{String(t.symbol)}</td>
                <td className={String(t.side) === 'BUY' ? 'text-green' : 'text-red'}>
                  {String(t.side) === 'BUY' ? '买入' : '卖出'}
                </td>
                <td>{String(t.price)}</td>
                <td>{String(t.quantity)}</td>
                <td>{String(t.amount)}</td>
                <td>{String(t.fee)}</td>
                <td>{String(t.strategy_name)}</td>
              </tr>
            ))}
          </tbody>
        </table>
      </div>
    </div>
  )
}
