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
import { useMemo, useState } from 'react'
import { useQuery } from '@tanstack/react-query'
import { useTranslation } from 'react-i18next'
import { Badge } from '@/components/ui/badge'
import { Skeleton } from '@/components/ui/skeleton'
import { getPerfMetricsSummary } from '@/features/performance-metrics/api'
import { getPricing } from '@/features/pricing/api'
import type { PricingModel } from '@/features/pricing/types'

type Scene = 'coding' | 'chat' | 'long' | 'reasoning' | 'image' | 'video'
const MIN_SAMPLES = 20

function getSceneLabel(scene: Scene, t: (key: string) => string): string {
  const labels: Record<Scene, string> = {
    coding: t('Coding assistant'),
    chat: t('General chat'),
    long: t('Long context'),
    reasoning: t('Reasoning'),
    image: t('Image'),
    video: t('Video'),
  }
  return labels[scene]
}

function sceneMatches(model: PricingModel, scene: Scene): boolean {
  const text = `${model.model_name} ${model.tags ?? ''}`.toLowerCase()
  if (scene === 'image') return text.includes('image') || text.includes('dall')
  if (scene === 'video') return text.includes('video') || text.includes('sora')
  if (scene === 'reasoning') return /reason|o[134]|r1/.test(text)
  if (scene === 'long') return /long|gemini|claude|gpt-5/.test(text)
  if (scene === 'coding') return /code|coder|claude|gpt|deepseek/.test(text)
  return !sceneMatches(model, 'image') && !sceneMatches(model, 'video')
}

function standardCost(model: PricingModel): number {
  if (model.quota_type === 1) return Number(model.model_price ?? 0)
  return (
    Number(model.model_ratio || 0) *
    (0.8 + Number(model.completion_ratio || 1) * 0.2)
  )
}

export function ModelValueComparison() {
  const { t } = useTranslation()
  const [scene, setScene] = useState<Scene>('coding')
  const pricing = useQuery({
    queryKey: ['pricing', 'model-value'],
    queryFn: getPricing,
    staleTime: 300_000,
  })
  const performance = useQuery({
    queryKey: ['perf-metrics-summary', 168],
    queryFn: () => getPerfMetricsSummary(168),
    staleTime: 60_000,
  })
  const rows = useMemo(() => {
    const perfMap = new Map(
      (performance.data?.data.models ?? []).map((item) => [
        item.model_name,
        item,
      ])
    )
    return (pricing.data?.data ?? [])
      .filter((model) => sceneMatches(model, scene))
      .map((model) => {
        const perf = perfMap.get(model.model_name)
        return {
          model,
          cost: standardCost(model),
          ttft: Number(perf?.avg_ttft_ms ?? 0),
          latency: Number(perf?.avg_latency_ms ?? 0),
          speed: Number(perf?.avg_tps ?? 0),
          success: Number(perf?.success_rate ?? 0),
          samples: Number(perf?.request_count ?? 0),
        }
      })
      .filter((row) => row.cost > 0)
      .sort((a, b) => {
        const aScore =
          a.samples >= MIN_SAMPLES
            ? (a.cost * Math.max(a.latency, 1)) / Math.max(a.success, 1)
            : Number.MAX_VALUE
        const bScore =
          b.samples >= MIN_SAMPLES
            ? (b.cost * Math.max(b.latency, 1)) / Math.max(b.success, 1)
            : Number.MAX_VALUE
        return aScore - bScore
      })
      .slice(0, 8)
  }, [performance.data, pricing.data, scene])
  const loading = pricing.isLoading || performance.isLoading

  return (
    <section
      className='overflow-hidden rounded-lg border'
      aria-labelledby='model-value-title'
    >
      <header className='flex flex-wrap items-center justify-between gap-3 border-b px-4 py-3 sm:px-5'>
        <div>
          <h2 id='model-value-title' className='text-sm font-semibold'>
            {t('Model value comparison')}
          </h2>
          <p className='text-muted-foreground mt-0.5 text-xs'>
            {t(
              'Compare cost, speed, reliability, and sample confidence in the selected scenario.'
            )}
          </p>
        </div>
        <div className='flex flex-wrap gap-2'>
          <select
            aria-label={t('Scenario')}
            className='border-input bg-background h-8 rounded-md border px-2 text-xs'
            value={scene}
            onChange={(event) => setScene(event.target.value as Scene)}
          >
            <option value='coding'>{t('Coding assistant')}</option>
            <option value='chat'>{t('General chat')}</option>
            <option value='long'>{t('Long context')}</option>
            <option value='reasoning'>{t('Reasoning')}</option>
            <option value='image'>{t('Image')}</option>
            <option value='video'>{t('Video')}</option>
          </select>
        </div>
      </header>
      {loading ? (
        <div className='space-y-3 p-5'>
          <Skeleton className='h-56 w-full' />
        </div>
      ) : rows.length === 0 ? (
        <div className='text-muted-foreground px-5 py-14 text-center text-sm'>
          {t('No comparable model data is available for this scenario.')}
        </div>
      ) : (
        <div className='overflow-x-auto'>
          <table className='w-full min-w-[720px] text-sm'>
            <thead className='bg-muted/40 text-muted-foreground text-xs'>
              <tr>
                {[
                  t('Model'),
                  t('Standard cost'),
                  t('First token'),
                  t('Output speed'),
                  t('Success rate'),
                  t('Samples'),
                  t('Recommendation'),
                ].map((label) => (
                  <th key={label} className='px-4 py-2.5 text-left font-medium'>
                    {label}
                  </th>
                ))}
              </tr>
            </thead>
            <tbody className='divide-y'>
              {rows.map((row, index) => (
                <tr key={row.model.model_name}>
                  <td
                    className='max-w-72 truncate px-4 py-3 font-mono text-xs'
                    title={row.model.model_name}
                  >
                    {row.model.model_name}
                  </td>
                  <td className='px-4 py-3 tabular-nums'>
                    {row.cost.toFixed(3)}
                  </td>
                  <td className='px-4 py-3 tabular-nums'>
                    {row.ttft ? `${Math.round(row.ttft)} ms` : '--'}
                  </td>
                  <td className='px-4 py-3 tabular-nums'>
                    {row.speed ? `${row.speed.toFixed(1)} tok/s` : '--'}
                  </td>
                  <td className='px-4 py-3 tabular-nums'>
                    {row.success ? `${row.success.toFixed(2)}%` : '--'}
                  </td>
                  <td className='px-4 py-3 tabular-nums'>{row.samples}</td>
                  <td className='px-4 py-3'>
                    {row.samples < MIN_SAMPLES ? (
                      <Badge variant='outline'>{t('Insufficient data')}</Badge>
                    ) : index === 0 ? (
                      <Badge>{t('Recommended')}</Badge>
                    ) : (
                      t('Comparable')
                    )}
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      )}
      <footer className='border-t px-4 py-3 text-xs sm:px-5'>
        <span className='font-medium'>{t('Conclusion')}：</span>
        {rows[0]?.samples >= MIN_SAMPLES
          ? t(
              '{{model}} currently offers the strongest cost-performance balance for {{scene}} based on {{samples}} samples.',
              {
                model: rows[0].model.model_name,
                scene: getSceneLabel(scene, t),
                samples: rows[0].samples,
              }
            )
          : t(
              'There are not enough samples to produce a reliable ranking.'
            )}{' '}
        <span className='text-muted-foreground'>
          · {t('7-day window')} ·{' '}
          {pricing.data?.data[0]?.pricing_version ?? t('current pricing')}
        </span>
      </footer>
    </section>
  )
}
