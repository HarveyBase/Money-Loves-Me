import { useState, useEffect } from 'react'
import { accountAPI } from '../services/api'

export default function AccountPanel() {
  const [balances, setBalances] = useState<Record<string, unknown>[]>([])
  const [pnl, setPnl] = useState('')
  const [fees, setFees] = useState('')

  useEffect(() => {
    accountAPI.getBalances().then((r) => setBalances(r.data.balances || []))
    accountAPI.getPnL().then((r) => setPnl(r.data.pnl || '0'))
    accountAPI.getFees().then((r) => setFees(r.data.total_fees || '0'))
  }, [])

  return (
    <div>
      <h2>资产概览</h2>
      <div style={{ display: 'flex', gap: 24, marginBottom: 16 }}>
        <div><strong>持仓盈亏:</strong> {pnl} USDT</div>
        <div><strong>累计手续费:</strong> {fees} USDT</div>
      </div>
      <table style={{ width: '100%' }}><thead><tr>
        <th>币种</th><th>可用余额</th><th>冻结余额</th>
      </tr></thead><tbody>
        {balances.map((b, i) => (
          <tr key={i}><td>{String(b.asset)}</td><td>{String(b.free)}</td><td>{String(b.locked)}</td></tr>
        ))}
      </tbody></table>
    </div>
  )
}
