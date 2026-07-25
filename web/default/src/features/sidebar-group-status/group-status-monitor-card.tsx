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
import { cn } from '@/lib/utils'
import { Card, CardContent } from '@/components/ui/card'
import { HealthStrip } from './health-strip'
import { formatSampleWindowLabel, getStatusMeta } from './presentation'
import type { SidebarGroupModelStatusItem } from './types'

export function GroupStatusMonitorCard(props: {
  item: SidebarGroupModelStatusItem
}) {
  const meta = getStatusMeta(props.item.status)
  const seriesWindowLabel = formatSampleWindowLabel(
    props.item.series_window ?? props.item.sample_window
  )

  return (
    <Card
      size='sm'
      className={cn(
        'group bg-card relative overflow-hidden border py-0 shadow-sm transition-shadow hover:shadow-md',
        meta.border
      )}
    >
      <div
        className={cn(
          'absolute top-0 left-0 h-full w-1 transition-all group-hover:w-1.5',
          meta.accent
        )}
      />

      <CardContent className='px-4 py-3.5'>
        <div className='space-y-3'>
          {/* Header: Model name and status badge */}
          <div className='flex items-start justify-between gap-2'>
            <div className='flex min-w-0 flex-1 items-center gap-2'>
              <div className={cn('size-2 shrink-0 rounded-full', meta.dot)} />
              <h4
                className='text-foreground min-w-0 flex-1 truncate text-sm font-semibold'
                title={props.item.model}
              >
                {props.item.model}
              </h4>
            </div>
            <div
              className={cn(
                'shrink-0 rounded-md px-2 py-0.5 text-[10px] font-bold tracking-wide uppercase',
                meta.accentText,
                meta.badgeBg
              )}
            >
              {meta.label}
            </div>
          </div>

          {/* Success rate metric */}
          <div className='flex items-baseline justify-between'>
            <span className='text-muted-foreground text-xs'>成功率</span>
            <div className='flex items-baseline gap-1'>
              <span className='text-foreground font-mono text-2xl font-bold tabular-nums'>
                {props.item.success_rate == null
                  ? '--'
                  : Math.round(props.item.success_rate)}
              </span>
              <span className='text-muted-foreground text-sm font-medium'>
                %
              </span>
            </div>
          </div>

          {/* Time range label */}
          <div className='text-muted-foreground text-[11px]'>
            {seriesWindowLabel}
          </div>

          {/* Health strip */}
          <div className='pt-1'>
            <HealthStrip item={props.item} />
          </div>
        </div>
      </CardContent>
    </Card>
  )
}
