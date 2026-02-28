import { useState, useEffect } from 'react'
import { orderAPI } from '../services/api'

export default function OrderPanel() {
  const [orders, setOrders] = useState<Record<string, unknown>[]>([])
  const [symbol, setSymbol] = useState('BTCUSDT')
  const [side, setSide] = useState('BUY')
  const [orderType, setOrderType] = useState('LIMIT')
  const [quantity, setQuantity] = useState('')
  const [price, setPrice] = useState('')

  useEffect(() => { orderAPI.list().then((r) => setOrders(r.data.orders || [])) }, [])

  const handleSubmit = async () => {
    await orderAPI.create({ symbol, side, type: orderType, quantity, price })
    const r = await orderAPI.list()
    setOrders(r.data.orders || [])
  }

  return (
    <div className="card">
      <div className="card-header">
        <span className="card-title">订单管理</span>
        <button className="btn btn-ghost btn-sm" onClick={() => orderAPI.exportCSV()}>导出 CSV</button>
      </div>

      <div className="form-row">
        <input className="form-input-sm" placeholder="交易对" value={symbol}
          onChange={(e) => setSymbol(e.target.value)} />
        <select className="form-select" value={side} onChange={(e) => setSide(e.target.value)} aria-label="方向">
          <option value="BUY">买入</option>
          <option value="SELL">卖出</option>
        </select>
        <select className="form-select" value={orderType} onChange={(e) => setOrderType(e.target.value)} aria-label="类型">
          <option value="LIMIT">限价单</option>
          <option value="MARKET">市价单</option>
          <option value="STOP_LOSS_LIMIT">止损限价</option>
          <option value="TAKE_PROFIT_LIMIT">止盈限价</option>
        </select>
        <input className="form-input-sm" placeholder="数量" value={quantity}
          onChange={(e) => setQuantity(e.target.value)} />
        <input className="form-input-sm" placeholder="价格" value={price}
          onChange={(e) => setPrice(e.target.value)} />
        <button className={`btn btn-sm ${side === 'BUY' ? 'btn-green' : 'btn-red'}`}
          onClick={handleSubmit}>
          {side === 'BUY' ? '买入' : '卖出'} {symbol}
        </button>
      </div>

      <div className="table-wrap">
        <table>
          <thead>
            <tr>
              <th>交易对</th><th>方向</th><th>类型</th><th>数量</th><th>价格</th><th>状态</th><th>操作</th>
            </tr>
          </thead>
          <tbody>
            {orders.length === 0 ? (
              <tr><td colSpan={7} style={{ textAlign: 'center', padding: 32, color: 'var(--text-muted)' }}>暂无订单数据</td></tr>
            ) : orders.map((o, i) => (
              <tr key={i}>
                <td style={{ color: 'var(--text-primary)' }}>{String(o.symbol)}</td>
                <td className={String(o.side) === 'BUY' ? 'text-green' : 'text-red'}>{String(o.side) === 'BUY' ? '买入' : '卖出'}</td>
                <td>{String(o.type)}</td>
                <td>{String(o.quantity)}</td>
                <td>{String(o.price)}</td>
                <td><span className="badge badge-muted">{String(o.status)}</span></td>
                <td><button className="btn btn-ghost btn-sm" onClick={() => orderAPI.cancel(String(o.id))}>取消</button></td>
              </tr>
            ))}
          </tbody>
        </table>
      </div>
    </div>
  )
}
