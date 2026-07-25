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
import { ArrowDown, CircleDollarSign, MousePointerClick } from 'lucide-react'
import { useTranslation } from 'react-i18next'
import { formatCompactNumber, formatNumber, formatQuota } from '@/lib/format'
import { Badge } from '@/components/ui/badge'
import { Skeleton } from '@/components/ui/skeleton'
import type { QuotaDataItem } from '../../types'

type ModelOperationsOverviewProps = {
  data: QuotaDataItem[]
  loading: boolean
}

type ModelAggregate = {
  name: string
  calls: number
  tokens: number
  quota: number
  callShare: number
  quotaShare: number
  averageQuota: number
}

function aggregateModels(data: QuotaDataItem[]): ModelAggregate[] {
  const modelMap = new Map<
    string,
    Omit<ModelAggregate, 'callShare' | 'quotaShare' | 'averageQuota'>
  >()

  data.forEach((item) => {
    const name = item.model_name?.trim() || 'unknown'
    const current = modelMap.get(name) ?? {
      name,
      calls: 0,
      tokens: 0,
      quota: 0,
    }
    current.calls += Number(item.count) || 0
    current.tokens += Number(item.token_used) || 0
    current.quota += Number(item.quota) || 0
    modelMap.set(name, current)
  })

  const rows = Array.from(modelMap.values())
  const totalCalls = rows.reduce((sum, row) => sum + row.calls, 0)
  const totalQuota = rows.reduce((sum, row) => sum + row.quota, 0)

  return rows
    .map((row) => ({
      ...row,
      callShare: totalCalls > 0 ? (row.calls / totalCalls) * 100 : 0,
      quotaShare: totalQuota > 0 ? (row.quota / totalQuota) * 100 : 0,
      averageQuota: row.calls > 0 ? row.quota / row.calls : 0,
    }))
    .sort((a, b) => b.quota - a.quota || b.calls - a.calls)
}

function OverviewSkeleton() {
  return (
    <div className='grid gap-3 lg:grid-cols-[minmax(0,0.9fr)_minmax(0,1.5fr)]'>
      <Skeleton className='h-80 w-full rounded-lg' />
      <Skeleton className='h-80 w-full rounded-lg' />
    </div>
  )
}

export function ModelOperationsOverview({
  data,
  loading,
}: ModelOperationsOverviewProps) {
  const { t } = useTranslation()
  const rows = useMemo(() => aggregateModels(data), [data])
  const totals = useMemo(
    () => ({
      calls: rows.reduce((sum, row) => sum + row.calls, 0),
      quota: rows.reduce((sum, row) => sum + row.quota, 0),
      tokens: rows.reduce((sum, row) => sum + row.tokens, 0),
    }),
    [rows]
  )

  if (loading) return <OverviewSkeleton />

  if (rows.length === 0) {
    return (
      <div className='text-muted-foreground rounded-lg border px-5 py-16 text-center text-sm'>
        {t('No model usage data is available for the selected period.')}
      </div>
    )
  }

  const topRows = rows.slice(0, 6)
  const topCallModel = [...rows].sort((a, b) => b.calls - a.calls)[0]
  const highestAverage = [...rows].sort(
    (a, b) => b.averageQuota - a.averageQuota
  )[0]

  return (
    <div className='space-y-3'>
      <div className='grid overflow-hidden rounded-lg border sm:grid-cols-3 sm:divide-x'>
        <div className='px-4 py-3'>
          <div className='text-muted-foreground flex items-center gap-2 text-xs'>
            <MousePointerClick className='size-3.5' />
            {t('Most called model')}
          </div>
          <div
            className='mt-1.5 truncate font-mono text-sm font-semibold'
            title={topCallModel.name}
          >
            {topCallModel.name}
          </div>
          <div className='text-muted-foreground mt-1 text-xs tabular-nums'>
            {formatNumber(topCallModel.calls)} ·{' '}
            {topCallModel.callShare.toFixed(1)}%
          </div>
        </div>
        <div className='border-t px-4 py-3 sm:border-t-0'>
          <div className='text-muted-foreground flex items-center gap-2 text-xs'>
            <CircleDollarSign className='size-3.5' />
            {t('Highest quota per call')}
          </div>
          <div
            className='mt-1.5 truncate font-mono text-sm font-semibold'
            title={highestAverage.name}
          >
            {highestAverage.name}
          </div>
          <div className='text-muted-foreground mt-1 text-xs tabular-nums'>
            {formatQuota(highestAverage.averageQuota)} / {t('call')}
          </div>
        </div>
        <div className='border-t px-4 py-3 sm:border-t-0'>
          <div className='text-muted-foreground flex items-center gap-2 text-xs'>
            <ArrowDown className='size-3.5' />
            {t('Top model quota share')}
          </div>
          <div className='mt-1.5 text-sm font-semibold tabular-nums'>
            {rows[0].quotaShare.toFixed(1)}%
          </div>
          <div
            className='text-muted-foreground mt-1 truncate text-xs'
            title={rows[0].name}
          >
            {rows[0].name}
          </div>
        </div>
      </div>

      <div className='grid gap-3 xl:grid-cols-[minmax(19rem,0.8fr)_minmax(0,1.7fr)]'>
        <section
          className='overflow-hidden rounded-lg border'
          aria-labelledby='quota-ranking-title'
        >
          <header className='border-b px-4 py-3'>
            <h2 id='quota-ranking-title' className='text-sm font-semibold'>
              {t('Quota consumption ranking')}
            </h2>
            <p className='text-muted-foreground mt-0.5 text-xs'>
              {t('Quickly identify the models driving total cost.')}
            </p>
          </header>
          <div className='space-y-3 p-4'>
            {topRows.map((row, index) => (
              <div key={row.name}>
                <div className='mb-1.5 flex items-center gap-2 text-xs'>
                  <span className='text-muted-foreground w-4 tabular-nums'>
                    {index + 1}
                  </span>
                  <span
                    className='min-w-0 flex-1 truncate font-mono'
                    title={row.name}
                  >
                    {row.name}
                  </span>
                  <span className='font-medium tabular-nums'>
                    {formatQuota(row.quota)}
                  </span>
                </div>
                <div className='bg-muted h-1.5 overflow-hidden rounded-full'>
                  <div
                    className='bg-primary h-full rounded-full'
                    style={{ width: `${Math.max(row.quotaShare, 1)}%` }}
                  />
                </div>
              </div>
            ))}
          </div>
        </section>

        <section
          className='overflow-hidden rounded-lg border'
          aria-labelledby='model-details-title'
        >
          <header className='flex items-center justify-between gap-3 border-b px-4 py-3'>
            <div>
              <h2 id='model-details-title' className='text-sm font-semibold'>
                {t('Model usage details')}
              </h2>
              <p className='text-muted-foreground mt-0.5 text-xs'>
                {t('{{models}} models · {{calls}} calls · {{tokens}} tokens', {
                  models: rows.length,
                  calls: formatCompactNumber(totals.calls),
                  tokens: formatCompactNumber(totals.tokens),
                })}
              </p>
            </div>
            <Badge variant='outline'>{formatQuota(totals.quota)}</Badge>
          </header>
          <div className='overflow-x-auto'>
            <table className='w-full min-w-[760px] text-sm'>
              <thead className='bg-muted/40 text-muted-foreground text-xs'>
                <tr>
                  {[
                    t('Model'),
                    t('Calls'),
                    t('Call share'),
                    t('Tokens'),
                    t('Quota'),
                    t('Quota share'),
                    t('Quota per call'),
                  ].map((label) => (
                    <th
                      key={label}
                      className='px-3 py-2.5 text-right font-medium first:text-left'
                    >
                      {label}
                    </th>
                  ))}
                </tr>
              </thead>
              <tbody className='divide-y'>
                {rows.map((row) => (
                  <tr key={row.name} className='hover:bg-muted/30'>
                    <td
                      className='max-w-64 truncate px-3 py-3 font-mono text-xs'
                      title={row.name}
                    >
                      {row.name}
                    </td>
                    <td className='px-3 py-3 text-right tabular-nums'>
                      {formatNumber(row.calls)}
                    </td>
                    <td className='px-3 py-3 text-right tabular-nums'>
                      {row.callShare.toFixed(1)}%
                    </td>
                    <td className='px-3 py-3 text-right tabular-nums'>
                      {formatCompactNumber(row.tokens)}
                    </td>
                    <td className='px-3 py-3 text-right font-medium tabular-nums'>
                      {formatQuota(row.quota)}
                    </td>
                    <td className='px-3 py-3 text-right tabular-nums'>
                      {row.quotaShare.toFixed(1)}%
                    </td>
                    <td className='px-3 py-3 text-right tabular-nums'>
                      {formatQuota(row.averageQuota)}
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        </section>
      </div>
    </div>
  )
}
