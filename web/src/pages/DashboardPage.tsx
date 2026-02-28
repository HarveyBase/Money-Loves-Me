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
  { key: 'chart', label: '📈 行情', icon: '' },
  { key: 'orders', label: '📋 订单', icon: '' },
  { key: 'trades', label: '💱 成交', icon: '' },
  { key: 'account', label: '💰 资产', icon: '' },
  { key: 'strategy', label: '🤖 策略', icon: '' },
  { key: 'risk', label: '🛡️ 风控', icon: '' },
  { key: 'backtest', label: '📊 回测', icon: '' },
  { key: 'optimizer', label: '⚙️ 优化', icon: '' },
  { key: 'notifications', label: '🔔 通知', icon: '' },
] as const

type TabKey = (typeof tabs)[number]['key']

export default function DashboardPage() {
  const [activeTab, setActiveTab] = useState<TabKey>('chart')

  const handleLogout = () => {
    localStorage.removeItem('token')
    window.location.href = '/login'
  }

  return (
    <div className="dashboard">
      <header className="topbar">
        <div className="topbar-brand">
          <span>◈ Money Loves Me</span>
          <span className="dot" />
        </div>
        <div className="topbar-right">
          <button className="btn btn-ghost btn-sm" onClick={handleLogout}>退出登录</button>
        </div>
      </header>

      <nav className="tab-nav" role="tablist">
        {tabs.map((tab) => (
          <button key={tab.key} role="tab" aria-selected={activeTab === tab.key}
            className={`tab-btn ${activeTab === tab.key ? 'active' : ''}`}
            onClick={() => setActiveTab(tab.key)}>
            {tab.label}
          </button>
        ))}
      </nav>

      <main className="content" role="tabpanel">
        {activeTab === 'chart' && <ChartPanel />}
        {activeTab === 'orders' && <OrderPanel />}
        {activeTab === 'trades' && <TradePanel />}
        {activeTab === 'account' && <AccountPanel />}
        {activeTab === 'strategy' && <StrategyPanel />}
        {activeTab === 'risk' && <RiskPanel />}
        {activeTab === 'backtest' && <BacktestPanel />}
        {activeTab === 'optimizer' && <OptimizerPanel />}
        {activeTab === 'notifications' && <NotificationPanel />}
      </main>
    </div>
  )
}
