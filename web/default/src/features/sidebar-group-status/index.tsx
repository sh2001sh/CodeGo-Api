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
import { Link } from '@tanstack/react-router'
import {
  AlertTriangle,
  CheckCircle2,
  Layers3,
  RefreshCcw,
  Rows3,
} from 'lucide-react'
import { useTranslation } from 'react-i18next'
import { cn } from '@/lib/utils'
import { Button } from '@/components/ui/button'
import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from '@/components/ui/card'
import { Skeleton } from '@/components/ui/skeleton'
import { SectionPageLayout } from '@/components/layout'
import { GroupStatusMonitorCard } from './group-status-monitor-card'
import {
  formatSampleWindowLabel,
  sortItems,
  summarizeGroups,
} from './presentation'
import { useSidebarGroupStatus } from './use-sidebar-group-status'

export function SidebarGroupStatusPage() {
  const { t } = useTranslation()
  const query = useSidebarGroupStatus()
  const items = sortItems(query.data?.data ?? [])
  const summary = summarizeGroups(items)

  return (
    <SectionPageLayout>
      <SectionPageLayout.Title>{t('Group status')}</SectionPageLayout.Title>
      <SectionPageLayout.Description>
        查看各分组下模型的可用状态、最近请求成功率和对应时间段表现。
      </SectionPageLayout.Description>
      <SectionPageLayout.Actions>
        <Button
          variant='outline'
          size='sm'
          render={
            <Link to='/dashboard/$section' params={{ section: 'overview' }} />
          }
        >
          概览
        </Button>
        <Button
          variant='outline'
          size='sm'
          onClick={() => void query.refetch()}
          disabled={query.isFetching}
        >
          <RefreshCcw
            className={cn('size-3.5', query.isFetching && 'animate-spin')}
          />
          刷新
        </Button>
      </SectionPageLayout.Actions>
      <SectionPageLayout.Content>
        <div className='mx-auto flex w-full max-w-[1700px] flex-col gap-5'>
          <OverviewPanel summary={summary} loading={query.isLoading} />

          {query.isLoading ? (
            <BoardSkeleton />
          ) : query.isError ? (
            <ErrorPanel onRetry={() => void query.refetch()} />
          ) : items.length === 0 ? (
            <EmptyPanel />
          ) : (
            <div className='flex flex-col gap-5'>
              {items.map((group) => (
                <section key={group.group} className='app-page-shell p-4'>
                  <div className='mb-4 flex items-end justify-between gap-3'>
                    <div className='space-y-1'>
                      <h3 className='text-foreground text-xl font-semibold tracking-tight'>
                        {group.group}
                      </h3>
                      <p className='text-muted-foreground text-sm'>
                        {group.models.length} 个模型
                      </p>
                    </div>
                  </div>

                  <div className='grid gap-3 sm:grid-cols-2 lg:grid-cols-3 xl:grid-cols-4 2xl:grid-cols-5'>
                    {group.models.length === 0 ? (
                      <div className='text-muted-foreground rounded-2xl border border-dashed px-4 py-6 text-sm'>
                        当前分组下暂无可展示模型。
                      </div>
                    ) : (
                      group.models.map((model) => (
                        <GroupStatusMonitorCard
                          key={`${group.group}-${model.model}`}
                          item={model}
                        />
                      ))
                    )}
                  </div>
                </section>
              ))}
            </div>
          )}
        </div>
      </SectionPageLayout.Content>
    </SectionPageLayout>
  )
}

function OverviewPanel(props: {
  summary: ReturnType<typeof summarizeGroups>
  loading: boolean
}) {
  const metrics = [
    {
      label: '业务分组',
      value: String(props.summary.groups),
      hint: '当前可查看的分组数',
      icon: Layers3,
      tone: 'text-muted-foreground',
    },
    {
      label: '正常模型',
      value: String(props.summary.healthyModels),
      hint: `共 ${props.summary.models} 个模型`,
      icon: CheckCircle2,
      tone: 'text-success',
    },
    {
      label: '缓慢模型',
      value: String(props.summary.slowModels),
      hint:
        props.summary.sampleWindow == null
          ? '暂无采样窗口'
          : `${formatSampleWindowLabel(props.summary.sampleWindow)} 成功率窗口`,
      icon: AlertTriangle,
      tone: 'text-warning',
    },
    {
      label: '故障模型',
      value: String(props.summary.degradedModels),
      hint:
        props.summary.sampleWindow == null
          ? '暂无采样窗口'
          : `${formatSampleWindowLabel(props.summary.sampleWindow)} 成功率窗口`,
      icon: AlertTriangle,
      tone: 'text-destructive',
    },
    {
      label: '观测中模型',
      value: String(props.summary.unknownModels),
      hint: '暂无足够请求样本',
      icon: Rows3,
      tone: 'text-muted-foreground',
    },
  ]

  return (
    <Card className='border-border/70 from-background via-background to-primary/5 bg-gradient-to-br'>
      <CardHeader className='border-border/70 border-b'>
        <CardTitle className='flex items-center gap-2'>
          <Layers3 className='text-primary size-4' />
          分组模型状态
        </CardTitle>
        <CardDescription className='max-w-[72ch] leading-6'>
          快速查看哪些模型稳定可用，哪些模型最近出现波动，并定位问题出现的大致时间段。
        </CardDescription>
      </CardHeader>
      <CardContent className='grid gap-3 pt-4 md:grid-cols-2 xl:grid-cols-5'>
        {metrics.map((metric) => {
          const Icon = metric.icon

          return (
            <div
              key={metric.label}
              className='border-border/70 bg-background/88 dark:bg-background/70 rounded-2xl border px-4 py-3'
            >
              <div className='flex items-start justify-between gap-3'>
                <div className='space-y-1'>
                  <div className='text-muted-foreground text-xs font-medium'>
                    {metric.label}
                  </div>
                  {props.loading ? (
                    <Skeleton className='h-7 w-18 rounded-md' />
                  ) : (
                    <div className='text-2xl font-semibold tracking-tight tabular-nums'>
                      {metric.value}
                    </div>
                  )}
                  <div className='text-muted-foreground text-xs'>
                    {metric.hint}
                  </div>
                </div>
                <div
                  className={cn(
                    'bg-muted flex size-10 items-center justify-center rounded-2xl',
                    metric.tone
                  )}
                >
                  <Icon className='size-5' />
                </div>
              </div>
            </div>
          )
        })}
      </CardContent>
    </Card>
  )
}

function BoardSkeleton() {
  return (
    <div className='flex flex-col gap-5'>
      {Array.from({ length: 4 }).map((_, groupIndex) => (
        <Card key={groupIndex} className='bg-card/50 py-0'>
          <CardContent className='space-y-4 px-4 py-4'>
            <div className='space-y-2'>
              <Skeleton className='h-6 w-36 rounded-md' />
              <Skeleton className='h-4 w-24 rounded-md' />
            </div>
            <div className='grid gap-3 sm:grid-cols-2 lg:grid-cols-3 xl:grid-cols-4 2xl:grid-cols-5'>
              {Array.from({ length: 5 }).map((__, cardIndex) => (
                <Skeleton key={cardIndex} className='h-48 w-full rounded-2xl' />
              ))}
            </div>
          </CardContent>
        </Card>
      ))}
    </div>
  )
}

function ErrorPanel(props: { onRetry: () => void }) {
  return (
    <Card>
      <CardContent className='flex flex-col items-start gap-4 py-8'>
        <div className='space-y-1'>
          <div className='text-base font-semibold'>模型状态暂时不可用</div>
          <div className='text-muted-foreground text-sm leading-6'>
            当前无法获取分组下模型状态数据，请稍后刷新重试。
          </div>
        </div>
        <Button variant='outline' size='sm' onClick={props.onRetry}>
          <RefreshCcw className='size-3.5' />
          重新获取
        </Button>
      </CardContent>
    </Card>
  )
}

function EmptyPanel() {
  return (
    <Card>
      <CardContent className='py-8'>
        <div className='space-y-1'>
          <div className='text-base font-semibold'>暂无可展示的模型状态</div>
          <div className='text-muted-foreground text-sm leading-6'>
            当前用户还没有可用的业务分组模型，或暂未产生用于监测的请求样本。
          </div>
        </div>
      </CardContent>
    </Card>
  )
}
