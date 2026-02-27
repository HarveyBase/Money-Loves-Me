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
    <div>
      <h2>交易记录</h2>
      <div style={{ display: 'flex', gap: 8, marginBottom: 16 }}>
        <input placeholder="交易对" value={symbol} onChange={(e) => setSymbol(e.target.value)} />
        <input placeholder="策略名称" value={strategy} onChange={(e) => setStrategy(e.target.value)} />
        <button onClick={load}>查询</button>
      </div>
      <table style={{ width: '100%' }}><thead><tr>
        <th>时间</th><th>交易对</th><th>方向</th><th>价格</th><th>数量</th><th>金额</th><th>手续费</th><th>策略</th><th>决策依据</th>
      </tr></thead><tbody>
        {trades.map((t, i) => (
          <tr key={i}><td>{String(t.executed_at)}</td><td>{String(t.symbol)}</td><td>{String(t.side)}</td>
            <td>{String(t.price)}</td><td>{String(t.quantity)}</td><td>{String(t.amount)}</td>
            <td>{String(t.fee)}</td><td>{String(t.strategy_name)}</td>
            <td><details><summary>详情</summary><pre>{JSON.stringify(t.decision_reason, null, 2)}</pre></details></td></tr>
        ))}
      </tbody></table>
    </div>
  )
}
