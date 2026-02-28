import { useState, useEffect } from 'react'
import { riskAPI } from '../services/api'

export default function RiskPanel() {
  const [config, setConfig] = useState({
    max_order_amount: '', max_daily_loss: '', stop_loss_percent: '', max_position_percent: ''
  })
  const [saved, setSaved] = useState(false)

  useEffect(() => { riskAPI.getConfig().then((r) => setConfig({ ...config, ...r.data.config })) }, [])

  const handleSave = async () => {
    await riskAPI.updateConfig(config)
    setSaved(true)
    setTimeout(() => setSaved(false), 2000)
  }

  return (
    <div className="card">
      <div className="card-header">
        <span className="card-title">风控参数配置</span>
        <button className="btn btn-accent btn-sm" onClick={handleSave}>
          {saved ? '✓ 已保存' : '保存配置'}
        </button>
      </div>

      <div className="risk-grid">
        <div className="form-group">
          <label>单笔最大金额 (USDT)</label>
          <input className="form-input" value={config.max_order_amount}
            onChange={(e) => setConfig({ ...config, max_order_amount: e.target.value })}
            placeholder="例如: 1000" />
        </div>
        <div className="form-group">
          <label>每日最大亏损 (USDT)</label>
          <input className="form-input" value={config.max_daily_loss}
            onChange={(e) => setConfig({ ...config, max_daily_loss: e.target.value })}
            placeholder="例如: 500" />
        </div>
        <div className="form-group">
          <label>止损百分比</label>
          <input className="form-input" value={config.stop_loss_percent}
            onChange={(e) => setConfig({ ...config, stop_loss_percent: e.target.value })}
            placeholder="例如: 0.05 (5%)" />
        </div>
        <div className="form-group">
          <label>最大持仓比例</label>
          <input className="form-input" value={config.max_position_percent}
            onChange={(e) => setConfig({ ...config, max_position_percent: e.target.value })}
            placeholder="例如: 0.3 (30%)" />
        </div>
      </div>
    </div>
  )
}
