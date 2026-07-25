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
import { Sparkles } from 'lucide-react'
import { cn } from '@/lib/utils'
import { Badge } from '@/components/ui/badge'

export interface PricingMarketHighlightProps {
  totalCount: number
  freeCount: number
  visibleFreeCount: number
  activeGroupLabel?: string
  className?: string
}

function TinyMetric(props: { label: string; value: string; hint: string }) {
  return (
    <div className='border-border/70 bg-background/75 rounded-[18px] border p-4'>
      <div className='text-muted-foreground text-[11px] font-semibold tracking-[0.14em]'>
        {props.label}
      </div>
      <div className='text-foreground mt-2 text-2xl font-semibold tabular-nums'>
        {props.value}
      </div>
      <div className='text-muted-foreground mt-1 text-xs leading-5'>
        {props.hint}
      </div>
    </div>
  )
}

export function PricingMarketHighlight(props: PricingMarketHighlightProps) {
  const freeRatio =
    props.totalCount > 0
      ? Math.round((props.freeCount / props.totalCount) * 100)
      : 0

  return (
    <section
      className={cn(
        'overview-soft-card flex flex-col gap-4 p-4 sm:p-5',
        props.className
      )}
    >
      <div className='flex flex-wrap items-center gap-2'>
        <Badge className='bg-primary/12 text-primary hover:bg-primary/12 gap-1.5 rounded-full px-2.5'>
          <Sparkles className='size-3.5' />
          当前分组概览
        </Badge>
        {props.activeGroupLabel && (
          <Badge variant='secondary' className='rounded-full px-2.5'>
            {props.activeGroupLabel}
          </Badge>
        )}
      </div>

      <div className='grid gap-3 md:grid-cols-3'>
        <TinyMetric
          label='当前可用'
          value={props.totalCount.toString()}
          hint='当前公开模型广场中可查看的模型数量。'
        />
        <TinyMetric
          label='免费模型'
          value={props.freeCount.toString()}
          hint='按当前分组倍率为 0 的规则自动识别。'
        />
        <TinyMetric
          label='筛选后免费'
          value={props.visibleFreeCount.toString()}
          hint='应用当前筛选和搜索后仍可直接使用的免费模型。'
        />
      </div>

      <p className='text-muted-foreground text-sm leading-7'>
        当前分组下，免费可用模型约占公开模型目录的 {freeRatio}%。
      </p>
    </section>
  )
}
