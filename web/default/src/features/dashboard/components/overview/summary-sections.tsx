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
import type { ReactNode } from 'react'
import { Link } from '@tanstack/react-router'
import { ArrowRight, WalletCards } from 'lucide-react'
import { cn } from '@/lib/utils'
import { Button } from '@/components/ui/button'
import { Progress } from '@/components/ui/progress'
import { formatSubscriptionQuotaAmount } from '@/features/subscriptions/lib'
import { DataMetric } from './summary-card-parts'

export type MetricDef = {
  label: string
  value: string
  hint?: string
}

export type BalanceSegment = {
  label: string
  display: string
  value: number
  dot: string
  bar: string
}

export function BalanceWorkspace(props: {
  available: string
  segments: BalanceSegment[]
  metrics: MetricDef[]
}) {
  const total = props.segments.reduce(
    (sum, segment) => sum + Math.max(0, segment.value),
    0
  )
  const activeSegments = props.segments.filter((segment) => segment.value > 0)

  return (
    <section className='overview-glass-card p-5 xl:p-6'>
      <div className='grid gap-6 xl:grid-cols-[minmax(0,0.95fr)_minmax(0,1.05fr)] xl:items-stretch'>
        <div className='flex flex-col'>
          <div className='flex items-center gap-2'>
            <WalletCards className='text-primary size-4' />
            <span className='text-muted-foreground text-[11px] font-medium tracking-[0.16em] uppercase'>
              可用总额度
            </span>
            <span className='border-primary/30 bg-primary/10 text-primary ml-auto rounded-full border px-2.5 py-0.5 text-[11px] font-medium'>
              USD
            </span>
          </div>

          <div className='text-foreground mt-3 text-5xl font-semibold tracking-tight xl:text-6xl'>
            {props.available}
          </div>

          <div className='bg-border/50 mt-5 flex h-2.5 overflow-hidden rounded-full'>
            {total > 0 ? (
              activeSegments.map((segment) => (
                <div
                  key={segment.label}
                  className={cn('h-full', segment.bar)}
                  style={{ width: `${(segment.value / total) * 100}%` }}
                />
              ))
            ) : (
              <div className='bg-border h-full w-full' />
            )}
          </div>

          <div className='mt-3 flex flex-wrap gap-x-4 gap-y-1.5 text-xs'>
            {props.segments.map((segment) => (
              <div key={segment.label} className='flex items-center gap-1.5'>
                <span className={cn('size-2 rounded-full', segment.dot)} />
                <span className='text-muted-foreground'>{segment.label}</span>
                <span className='text-foreground font-medium tabular-nums'>
                  {segment.display}
                </span>
              </div>
            ))}
          </div>

          <div className='mt-5 grid gap-2.5 sm:grid-cols-3'>
            <Button
              variant='outline'
              className='justify-between rounded-2xl'
              render={<Link to='/wallet' />}
            >
              <span>钱包</span>
              <ArrowRight data-icon='inline-end' />
            </Button>
            <Button
              variant='outline'
              className='justify-between rounded-2xl'
              render={<Link to='/packages' />}
            >
              <span>套餐</span>
              <ArrowRight data-icon='inline-end' />
            </Button>
          </div>
        </div>

        <div className='grid gap-3 sm:grid-cols-2 xl:grid-cols-3'>
          {props.metrics.map((metric) => (
            <DataMetric
              key={metric.label}
              label={metric.label}
              value={metric.value}
              hint={metric.hint}
            />
          ))}
        </div>
      </div>
    </section>
  )
}

export function PackageStatusCard(props: {
  hasSubscription: boolean
  title: string
  subtitle: string
  remainingDays: number
  totalUsed: number
  totalAmount: number
  totalHint: string
  periodUsed?: number
  periodAmount?: number
  periodHint?: string
  children?: ReactNode
}) {
  const totalPercent =
    props.totalAmount > 0
      ? Math.min(100, Math.round((props.totalUsed / props.totalAmount) * 100))
      : 0
  const periodPercent =
    props.periodAmount && props.periodAmount > 0
      ? Math.min(
          100,
          Math.round(((props.periodUsed ?? 0) / props.periodAmount) * 100)
        )
      : 0

  return (
    <section className='overview-glass-card flex h-full flex-col gap-4 p-4 sm:p-5'>
      <div className='flex items-start justify-between gap-3'>
        <div>
          <div className='text-muted-foreground text-[11px] font-medium tracking-[0.16em] uppercase'>
            套餐状态
          </div>
          <div className='text-foreground mt-1 text-lg font-semibold tracking-tight'>
            {props.title}
          </div>
          <div className='text-muted-foreground mt-1 text-sm leading-6'>
            {props.subtitle}
          </div>
        </div>
        <div className='border-border/70 bg-background/80 text-foreground rounded-full border px-2.5 py-1 text-xs font-medium'>
          {props.hasSubscription ? `剩余 ${props.remainingDays} 天` : '未订阅'}
        </div>
      </div>

      {props.hasSubscription ? (
        <div className='grid gap-3'>
          <div className='overview-soft-card p-3'>
            <div className='flex items-center justify-between gap-3 text-sm'>
              <div className='text-foreground font-medium'>总额度进度</div>
              <div className='text-muted-foreground text-xs'>
                {formatSubscriptionQuotaAmount(
                  Math.max(0, props.totalAmount - props.totalUsed)
                )}{' '}
                / {formatSubscriptionQuotaAmount(props.totalAmount)}
              </div>
            </div>
            <div className='mt-3'>
              <Progress value={totalPercent} />
            </div>
            <div className='text-muted-foreground mt-2 text-xs'>
              {props.totalHint}
            </div>
          </div>

          {props.periodAmount != null && props.periodAmount > 0 ? (
            <div className='overview-soft-card p-3'>
              <div className='flex items-center justify-between gap-3 text-sm'>
                <div className='text-foreground font-medium'>周期额度</div>
                <div className='text-muted-foreground text-xs'>
                  {formatSubscriptionQuotaAmount(
                    Math.max(0, props.periodAmount - (props.periodUsed ?? 0))
                  )}{' '}
                  / {formatSubscriptionQuotaAmount(props.periodAmount)}
                </div>
              </div>
              <div className='mt-3'>
                <Progress value={periodPercent} />
              </div>
              {props.periodHint ? (
                <div className='text-muted-foreground mt-2 text-xs'>
                  {props.periodHint}
                </div>
              ) : null}
            </div>
          ) : null}

          <div className='grid gap-2 sm:grid-cols-2'>{props.children}</div>
        </div>
      ) : (
        <div className='border-border text-muted-foreground rounded-xl border border-dashed px-4 py-6 text-sm'>
          当前没有生效套餐。购买后这里会展示额度使用进度。
        </div>
      )}

      <Button
        className='mt-auto justify-between rounded-2xl'
        render={<Link to='/wallet' />}
      >
        <span>进入套餐与扣费管理</span>
        <ArrowRight data-icon='inline-end' />
      </Button>
    </section>
  )
}

export function StatusInfoCard(props: {
  label: string
  value: string
  hint: string
}) {
  return (
    <div className='overview-soft-card px-3 py-3'>
      <div className='text-muted-foreground text-[11px] font-medium tracking-[0.14em] uppercase'>
        {props.label}
      </div>
      <div className='text-foreground mt-1 text-lg font-semibold'>
        {props.value}
      </div>
      <div className='text-muted-foreground mt-1 text-xs leading-5'>
        {props.hint}
      </div>
    </div>
  )
}
