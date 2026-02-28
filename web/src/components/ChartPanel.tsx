import { useEffect, useRef } from 'react'
import { createChart, type UTCTimestamp } from 'lightweight-charts'
import wsClient from '../services/websocket'

export default function ChartPanel() {
  const chartRef = useRef<HTMLDivElement>(null)

  useEffect(() => {
    if (!chartRef.current) return
    const chart = createChart(chartRef.current, {
      width: chartRef.current.clientWidth,
      height: 520,
      layout: {
        background: { color: '#0b0e11' },
        textColor: '#848e9c',
        fontSize: 12,
      },
      grid: {
        vertLines: { color: '#1e2329' },
        horzLines: { color: '#1e2329' },
      },
      crosshair: {
        vertLine: { color: '#f0b90b', width: 1, style: 2, labelBackgroundColor: '#f0b90b' },
        horzLine: { color: '#f0b90b', width: 1, style: 2, labelBackgroundColor: '#f0b90b' },
      },
      rightPriceScale: {
        borderColor: '#2b3139',
      },
      timeScale: {
        borderColor: '#2b3139',
        timeVisible: true,
      },
    })

    const candleSeries = chart.addCandlestickSeries({
      upColor: '#0ecb81',
      downColor: '#f6465d',
      borderUpColor: '#0ecb81',
      borderDownColor: '#f6465d',
      wickUpColor: '#0ecb81',
      wickDownColor: '#f6465d',
    })

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
    <div className="card">
      <div className="card-header">
        <span className="card-title">BTCUSDT · K线图表</span>
      </div>
      <div ref={chartRef} style={{ width: '100%', borderRadius: 'var(--radius-sm)', overflow: 'hidden' }} />
    </div>
  )
}
