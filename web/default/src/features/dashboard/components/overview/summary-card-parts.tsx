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
import { useState } from 'react'
import { formatQuota } from '@/lib/format'
import { Progress } from '@/components/ui/progress'

function clampPercent(used: number, total: number) {
  if (total <= 0) return 0
  return Math.max(0, Math.min(100, Math.round((used / total) * 100)))
}

type UsagePoint = {
  label: string
  value: number
}

export function UsageChart(props: { points: UsagePoint[] }) {
  const [hoveredIndex, setHoveredIndex] = useState<number | null>(null)
  const maxValue = Math.max(...props.points.map((point) => point.value), 1)
  const peakPoint =
    props.points.reduce<UsagePoint | null>((peak, point) => {
      if (!peak || point.value > peak.value) return point
      return peak
    }, null) ?? null
  const averageValue =
    props.points.length > 0
      ? props.points.reduce((sum, point) => sum + point.value, 0) /
        props.points.length
      : 0

  const displayPoint =
    hoveredIndex !== null ? props.points[hoveredIndex] : props.points.at(-1)

  return (
    <div className='overview-glass-card overview-panel-backdrop p-5 sm:p-6'>
      <div className='flex flex-wrap items-center justify-between gap-3'>
        <div className='min-w-0 flex-1'>
          <div className='text-muted-foreground text-[11px] font-medium tracking-[0.14em] uppercase'>
            用量概览
          </div>
          <div className='text-foreground mt-1 text-xl font-semibold tracking-tight'>
            最近 12 小时用量走势
          </div>
        </div>
        <div className='border-border/70 bg-background/80 text-muted-foreground rounded-full border px-3 py-1.5 text-xs font-medium'>
          按小时统计
        </div>
      </div>

      <div className='mt-5 grid gap-4 sm:grid-cols-3'>
        <div className='overview-soft-card px-4 py-3.5'>
          <div className='text-muted-foreground mb-1 text-xs font-medium'>
            {hoveredIndex !== null ? '悬停时段' : '当前时段'}
          </div>
          <div className='text-foreground text-2xl font-semibold tabular-nums'>
            {formatQuota(displayPoint?.value ?? 0)}
          </div>
          {displayPoint && (
            <div className='text-muted-foreground mt-1 text-xs'>
              {displayPoint.label}
            </div>
          )}
        </div>
        <DataMetric
          label='峰值时段'
          value={formatQuota(peakPoint?.value ?? 0)}
          hint={peakPoint?.label}
        />
        <DataMetric
          label='平均消耗'
          value={formatQuota(averageValue)}
          hint='12 小时平均'
        />
      </div>

      {props.points.length > 0 ? (
        <div
          className='overview-soft-card relative mt-5 p-5'
          onMouseLeave={() => setHoveredIndex(null)}
        >
          <div className='grid h-[200px] grid-cols-12 items-end gap-2.5 sm:gap-3'>
            {props.points.map((point, index) => {
              const isPeak =
                peakPoint?.label === point.label &&
                peakPoint.value === point.value
              const isCurrent = index === props.points.length - 1
              const isHovered = hoveredIndex === index
              const height = Math.max(
                8,
                Math.round((point.value / maxValue) * 100)
              )

              return (
                <div
                  key={`${point.label}-${index}`}
                  className='group relative flex h-full flex-col justify-end'
                  onMouseEnter={() => setHoveredIndex(index)}
                >
                  <div
                    className={`relative rounded-t-2xl transition-all duration-200 ${isHovered ? 'shadow-lg' : ''} ${
                      isPeak
                        ? 'from-primary/70 to-primary bg-gradient-to-b'
                        : isCurrent
                          ? 'from-muted-foreground/40 to-muted-foreground/70 bg-gradient-to-b'
                          : 'from-muted to-muted-foreground/20 bg-gradient-to-b'
                    } `}
                    style={{ height: `${height}%` }}
                  >
                    {isHovered && (
                      <div className='border-border bg-popover absolute -top-14 left-1/2 z-10 -translate-x-1/2 rounded-lg border px-3 py-2 text-xs whitespace-nowrap shadow-lg'>
                        <div className='text-popover-foreground font-semibold'>
                          {formatQuota(point.value)}
                        </div>
                        <div className='text-muted-foreground mt-0.5 text-[11px]'>
                          {point.label}
                        </div>
                      </div>
                    )}
                  </div>
                  <div className='text-muted-foreground mt-2 text-center text-[10px] font-medium'>
                    {point.label}
                  </div>
                </div>
              )
            })}
          </div>
        </div>
      ) : (
        <div className='border-border text-muted-foreground bg-background/70 mt-5 flex min-h-[240px] items-center justify-center rounded-2xl border border-dashed px-4 py-8 text-center text-sm'>
          最近 12 小时还没有可展示的用量数据。
        </div>
      )}
    </div>
  )
}

export function DataMetric(props: {
  label: string
  value: string
  hint?: string
}) {
  return (
    <div className='overview-soft-card px-3 py-3'>
      <div className='text-muted-foreground text-[11px] font-medium'>
        {props.label}
      </div>
      <div className='text-foreground mt-1 text-lg font-semibold'>
        {props.value}
      </div>
      {props.hint ? (
        <div className='text-muted-foreground mt-1 text-xs'>{props.hint}</div>
      ) : null}
    </div>
  )
}

export function ProgressBlock(props: {
  label: string
  used: number
  total: number
  remainingLabel: string
  hint: string
  className?: string
}) {
  const percent = clampPercent(props.used, props.total)

  return (
    <div className='app-subtle-panel p-3'>
      <div className='flex items-center justify-between gap-3 text-sm'>
        <div className='text-foreground font-medium'>{props.label}</div>
        <div className='text-muted-foreground text-xs'>
          {props.remainingLabel}
        </div>
      </div>
      <div className='mt-3'>
        <Progress className={props.className} value={percent} />
      </div>
      <div className='text-muted-foreground mt-2 text-xs'>{props.hint}</div>
    </div>
  )
}
