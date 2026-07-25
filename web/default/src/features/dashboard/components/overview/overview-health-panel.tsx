/*
Copyright (C) 2026 codego-api contributors

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as
published by the Free Software Foundation, either version 3 of the
License, or (at your option) any later version.

This program is distributed in the hope that it will be useful,
but WITHOUT ANY WARRANTY; without even the implied warranty of
MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
GNU Affero General Public License for more details.

You should have received a copy of the GNU Affero General Public License
along with this program. If not, see <https://www.gnu.org/licenses/>.

*/
import { useMemo } from 'react'
import { useQuery } from '@tanstack/react-query'
import { Activity, Gauge, HeartPulse, RadioTower, Timer } from 'lucide-react'
import { getUptimeStatus } from '@/features/dashboard/api'
import { getPerfMetricsSummary } from '@/features/performance-metrics/api'
import {
  formatLatency,
  formatThroughput,
  formatUptimePct,
} from '@/features/performance-metrics/lib/format'
import type { PerfModelSummary } from '@/features/performance-metrics/types'

const PERFORMANCE_WINDOW_HOURS = 24

type WeightedMetric = 'avg_latency_ms' | 'avg_tps' | 'success_rate'

function simpleAverage(
  rows: PerfModelSummary[],
  metric: WeightedMetric,
  isValid: (value: number) => boolean
): number {
  let total = 0
  let count = 0
  for (const row of rows) {
    const value = Number(row[metric])
    if (!isValid(value)) continue
    total += value
    count++
  }
  return count > 0 ? total / count : NaN
}

function resolveOverallStatus(successRate: number) {
  if (!Number.isFinite(successRate)) return '观测中'
  if (successRate >= 99.9) return '运行正常'
  if (successRate >= 99) return '轻微波动'
  return '需要关注'
}

export function OverviewHealthPanel() {
  const metricsQuery = useQuery({
    queryKey: ['perf-metrics-summary', PERFORMANCE_WINDOW_HOURS, 'overview'],
    queryFn: () => getPerfMetricsSummary(PERFORMANCE_WINDOW_HOURS),
    staleTime: 60 * 1000,
    retry: false,
  })

  const uptimeQuery = useQuery({
    queryKey: ['dashboard', 'uptime-status', 'overview'],
    queryFn: getUptimeStatus,
    staleTime: 60 * 1000,
  })

  const models = useMemo(
    () => metricsQuery.data?.data.models ?? [],
    [metricsQuery.data]
  )

  const summary = useMemo(() => {
    return {
      avgLatencyMs: Math.round(
        simpleAverage(
          models,
          'avg_latency_ms',
          (v) => Number.isFinite(v) && v > 0
        )
      ),
      avgTps: simpleAverage(
        models,
        'avg_tps',
        (v) => Number.isFinite(v) && v > 0
      ),
      successRate: simpleAverage(models, 'success_rate', Number.isFinite),
    }
  }, [models])

  const uptimeMonitors =
    uptimeQuery.data?.data?.flatMap((group) => group.monitors ?? []) ?? []
  const uptimeAverage =
    uptimeMonitors.length > 0
      ? uptimeMonitors.reduce(
          (sum, monitor) => sum + Number(monitor.uptime ?? 0),
          0
        ) / uptimeMonitors.length
      : NaN

  const rows = [
    {
      label: 'API 路由',
      value: uptimeMonitors.length > 0 ? '在线' : '等待配置',
      icon: RadioTower,
    },
    {
      label: '平均延迟',
      value: formatLatency(summary.avgLatencyMs),
      icon: Timer,
    },
    {
      label: '峰值吞吐',
      value: formatThroughput(summary.avgTps),
      icon: Gauge,
    },
    {
      label: '错误率',
      value: Number.isFinite(summary.successRate)
        ? `${(100 - summary.successRate).toFixed(2)}%`
        : '—',
      icon: HeartPulse,
    },
    {
      label: '可用率',
      value: Number.isFinite(uptimeAverage)
        ? formatUptimePct(uptimeAverage * 100)
        : '—',
      icon: Activity,
    },
  ]

  return (
    <section className='overview-glass-card p-5 sm:p-6'>
      <div className='flex items-start justify-between gap-3'>
        <div>
          <div className='text-muted-foreground text-[11px] font-medium tracking-[0.16em] uppercase'>
            平台健康
          </div>
          <div className='text-foreground mt-1 text-xl font-semibold tracking-tight'>
            今天的主要状态
          </div>
        </div>
        <div className='border-success/20 bg-success/10 text-success rounded-full border px-2.5 py-1 text-xs font-medium'>
          {resolveOverallStatus(summary.successRate)}
        </div>
      </div>

      <div className='mt-4 grid gap-2.5'>
        {rows.map((row) => {
          const Icon = row.icon
          return (
            <div
              key={row.label}
              className='overview-soft-card flex items-center justify-between gap-3 px-3 py-3'
            >
              <span className='flex min-w-0 items-center gap-2.5'>
                <span className='bg-primary/10 text-primary flex size-8 shrink-0 items-center justify-center rounded-xl'>
                  <Icon className='size-3.5' aria-hidden='true' />
                </span>
                <span className='text-foreground truncate text-sm font-medium'>
                  {row.label}
                </span>
              </span>
              <span className='text-foreground shrink-0 text-sm font-semibold tabular-nums'>
                {row.value}
              </span>
            </div>
          )
        })}
      </div>
    </section>
  )
}
