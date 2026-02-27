import { useState } from 'react'
import ChartPanel from '../components/ChartPanel'
import OrderPanel from '../components/OrderPanel'
import AccountPanel from '../components/AccountPanel'
import StrategyPanel from '../components/StrategyPanel'
import RiskPanel from '../components/RiskPanel'
import BacktestPanel from '../components/BacktestPanel'
import OptimizerPanel from '../components/OptimizerPanel'
import NotificationPanel from '../components/NotificationPanel'
import TradePanel from '../components/TradePanel'

const tabs = [
  { key: 'chart', label: 'K线图表' },
  { key: 'orders', label: '订单管理' },
  { key: 'account', label: '资产概览' },
  { key: 'strategy', label: '自动交易' },
  { key: 'risk', label: '风控设置' },
  { key: 'backtest', label: '回测结果' },
  { key: 'optimizer', label: '优化历史' },
  { key: 'notifications', label: '通知消息' },
  { key: 'trades', label: '交易记录' },
] as const

type TabKey = (typeof tabs)[number]['key']

export default function DashboardPage() {
  const [activeTab, setActiveTab] = useState<TabKey>('chart')

  const handleLogout = () => {
    localStorage.removeItem('token')
    window.location.href = '/login'
  }

  return (
    <div style={{ padding: 16 }}>
      <div style={{ display: 'flex', justifyContent: 'space-between', marginBottom: 16 }}>
        <h1 style={{ margin: 0 }}>币安交易系统</h1>
        <button onClick={handleLogout}>退出登录</button>
      </div>
      <nav style={{ marginBottom: 16, display: 'flex', gap: 8 }} role="tablist">
        {tabs.map((tab) => (
          <button key={tab.key} role="tab" aria-selected={activeTab === tab.key}
            onClick={() => setActiveTab(tab.key)}
            style={{ padding: '6px 12px', fontWeight: activeTab === tab.key ? 'bold' : 'normal' }}>
            {tab.label}
          </button>
        ))}
      </nav>
      <div role="tabpanel">
        {activeTab === 'chart' && <ChartPanel />}
        {activeTab === 'orders' && <OrderPanel />}
        {activeTab === 'account' && <AccountPanel />}
        {activeTab === 'strategy' && <StrategyPanel />}
        {activeTab === 'risk' && <RiskPanel />}
        {activeTab === 'backtest' && <BacktestPanel />}
        {activeTab === 'optimizer' && <OptimizerPanel />}
        {activeTab === 'notifications' && <NotificationPanel />}
        {activeTab === 'trades' && <TradePanel />}
      </div>
    </div>
  )
}
