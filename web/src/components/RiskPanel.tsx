import { useState, useEffect } from 'react'
import { riskAPI } from '../services/api'

export default function RiskPanel() {
  const [config, setConfig] = useState({ max_order_amount: '', max_daily_loss: '', stop_loss_percent: '', max_position_percent: '' })

  useEffect(() => { riskAPI.getConfig().then((r) => setConfig({ ...config, ...r.data.config })) }, [])

  const handleSave = async () => {
    await riskAPI.updateConfig(config)
  }

  return (
    <div>
      <h2>风控设置</h2>
      <div style={{ display: 'grid', gap: 12, maxWidth: 400 }}>
        <label>单笔最大金额 (USDT)
          <input value={config.max_order_amount} onChange={(e) => setConfig({ ...config, max_order_amount: e.target.value })} />
        </label>
        <label>每日最大亏损 (USDT)
          <input value={config.max_daily_loss} onChange={(e) => setConfig({ ...config, max_daily_loss: e.target.value })} />
        </label>
        <label>止损百分比
          <input value={config.stop_loss_percent} onChange={(e) => setConfig({ ...config, stop_loss_percent: e.target.value })} />
        </label>
        <label>最大持仓比例
          <input value={config.max_position_percent} onChange={(e) => setConfig({ ...config, max_position_percent: e.target.value })} />
        </label>
        <button onClick={handleSave}>保存配置</button>
      </div>
    </div>
  )
}
