import { useState, useEffect } from 'react'
import { accountAPI } from '../services/api'

export default function AccountPanel() {
  const [balances, setBalances] = useState<Record<string, unknown>[]>([])
  const [pnl, setPnl] = useState('0')
  const [fees, setFees] = useState('0')

  useEffect(() => {
    accountAPI.getBalances().then((r) => setBalances(r.data.balances || []))
    accountAPI.getPnL().then((r) => setPnl(r.data.pnl || '0'))
    accountAPI.getFees().then((r) => setFees(r.data.total_fees || '0'))
  }, [])

  const pnlNum = parseFloat(pnl)

  return (
    <>
      <div className="stats-grid">
        <div className="stat-card">
          <div className="stat-label">持仓盈亏</div>
          <div className={`stat-value ${pnlNum >= 0 ? 'green' : 'red'}`}>
            {pnlNum >= 0 ? '+' : ''}{pnl} USDT
          </div>
        </div>
        <div className="stat-card">
          <div className="stat-label">累计手续费</div>
          <div className="stat-value">{fees} USDT</div>
        </div>
        <div className="stat-card">
          <div className="stat-label">持仓币种</div>
          <div className="stat-value">{balances.length}</div>
        </div>
      </div>

      <div className="card">
        <div className="card-header">
          <span className="card-title">资产明细</span>
        </div>
        <div className="table-wrap">
          <table>
            <thead>
              <tr><th>币种</th><th>可用余额</th><th>冻结余额</th></tr>
            </thead>
            <tbody>
              {balances.length === 0 ? (
                <tr><td colSpan={3} style={{ textAlign: 'center', padding: 32, color: 'var(--text-muted)' }}>暂无资产数据</td></tr>
              ) : balances.map((b, i) => (
                <tr key={i}>
                  <td style={{ color: 'var(--text-primary)', fontWeight: 600 }}>{String(b.asset)}</td>
                  <td>{String(b.free)}</td>
                  <td>{String(b.locked)}</td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      </div>
    </>
  )
}
