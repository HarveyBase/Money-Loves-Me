import { useEffect, useRef } from 'react'
import { createChart, type UTCTimestamp } from 'lightweight-charts'
import wsClient from '../services/websocket'

export default function ChartPanel() {
  const chartRef = useRef<HTMLDivElement>(null)

  useEffect(() => {
    if (!chartRef.current) return
    const chart = createChart(chartRef.current, {
      width: chartRef.current.clientWidth,
      height: 500,
      layout: { background: { color: '#1a1a2e' }, textColor: '#e0e0e0' },
      grid: { vertLines: { color: '#2a2a3e' }, horzLines: { color: '#2a2a3e' } },
    })
    const candleSeries = chart.addCandlestickSeries()

    const handler = (data: unknown) => {
      const kline = data as { time: number; open: number; high: number; low: number; close: number }
      candleSeries.update({ ...kline, time: kline.time as UTCTimestamp })
    }
    wsClient.on('market', handler)

    const resizeObserver = new ResizeObserver(() => {
      if (chartRef.current) chart.applyOptions({ width: chartRef.current.clientWidth })
    })
    resizeObserver.observe(chartRef.current)

    return () => {
      wsClient.off('market', handler)
      resizeObserver.disconnect()
      chart.remove()
    }
  }, [])

  return (
    <div>
      <h2>K线图表</h2>
      <div ref={chartRef} style={{ width: '100%' }} />
    </div>
  )
}
