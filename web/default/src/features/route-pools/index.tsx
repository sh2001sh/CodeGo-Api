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
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { AlertCircle, RefreshCw } from 'lucide-react'
import { toast } from 'sonner'
import { api } from '@/lib/api'
import { Button } from '@/components/ui/button'
import { Card, CardContent } from '@/components/ui/card'
import { SectionPageLayout } from '@/components/layout'
import { RoutePoolGroupList } from './components/route-pool-group-list'
import { RoutePoolInsights } from './components/route-pool-insights'
import type { RoutePoolGroup, RoutePoolMetrics } from './types'

export function RoutePools() {
  return (
    <SectionPageLayout>
      <SectionPageLayout.Title>渠道与智能路由</SectionPageLayout.Title>
      <SectionPageLayout.Description>
        渠道分组来自渠道配置；启用算法后按成本、成功率、冷却和首字耗时自动选择。
      </SectionPageLayout.Description>
      <SectionPageLayout.Content>
        <RoutePoolsContent />
      </SectionPageLayout.Content>
    </SectionPageLayout>
  )
}

export function RoutePoolsContent() {
  const queryClient = useQueryClient()
  const [model, setModel] = useState('')
  const [selectedGroup, setSelectedGroup] = useState<string | null>(null)
  const groups = useQuery({
    queryKey: ['route-pool-groups'],
    queryFn: async () =>
      (
        await api.get<{ data: { items: RoutePoolGroup[] } }>(
          '/api/route-pools/groups'
        )
      ).data.data.items,
  })
  const selected = useMemo(
    () =>
      groups.data?.find((group) => group.group === selectedGroup) ??
      groups.data?.[0],
    [groups.data, selectedGroup]
  )
  const metrics = useQuery({
    queryKey: ['route-pool-metrics', selected?.pool_id, model],
    enabled: Boolean(selected?.pool_id && selected?.algorithm_active && model),
    queryFn: async () =>
      (
        await api.get<{ data: RoutePoolMetrics }>(
          `/api/route-pools/${selected?.pool_id}/metrics`,
          { params: { model } }
        )
      ).data.data,
  })
  const saveGroup = useMutation({
    mutationFn: (group: RoutePoolGroup) =>
      api.put('/api/route-pools/groups', {
        group: group.group,
        enabled: group.enabled,
        members: group.channels.map((channel) => ({
          channel_id: channel.channel_id,
          enabled: channel.enabled,
          cost_multiplier: channel.cost_multiplier,
          model_cost_overrides: channel.model_cost_overrides,
        })),
      }),
    onSuccess: () => {
      toast.success('自动路由配置已保存')
      void queryClient.invalidateQueries({ queryKey: ['route-pool-groups'] })
      void queryClient.invalidateQueries({ queryKey: ['route-pool-metrics'] })
    },
    onError: () => toast.error('自动路由配置保存失败'),
  })
  const refresh = () => {
    void groups.refetch()
    void metrics.refetch()
  }
  const queryFailed = groups.isError

  return (
    <div className='space-y-4'>
      <div className='flex flex-col justify-between gap-3 sm:flex-row sm:items-start'>
        <div>
          <h2 className='text-lg font-semibold'>自动路由</h2>
          <p className='text-muted-foreground mt-1 text-sm'>
            分组与候选渠道自动同步自渠道配置。开启后，未命中缓存粘性的请求按成本、健康度和首字时间评分选择。
          </p>
        </div>
        <Button variant='outline' onClick={refresh}>
          <RefreshCw />
          刷新
        </Button>
      </div>

      {queryFailed && (
        <Card className='border-destructive/40'>
          <CardContent className='flex items-center justify-between gap-3 py-4'>
            <div className='flex min-w-0 items-start gap-2 text-sm'>
              <AlertCircle className='text-destructive mt-0.5 size-4 shrink-0' />
              <div>
                <p className='font-medium'>路由数据暂时无法加载</p>
                <p className='text-muted-foreground mt-1'>
                  请刷新重试；现有渠道和运行中的路由不会被修改。
                </p>
              </div>
            </div>
            <Button size='sm' variant='outline' onClick={refresh}>
              重试
            </Button>
          </CardContent>
        </Card>
      )}

      <RoutePoolGroupList
        groups={groups.data ?? []}
        loading={groups.isLoading}
        savingGroup={saveGroup.variables?.group}
        onSelectGroup={setSelectedGroup}
        onSave={(group) => saveGroup.mutate(group)}
      />

      <div>
        <RoutePoolInsights
          groups={groups.data ?? []}
          model={model}
          selectedGroup={selected?.group ?? null}
          metrics={metrics.data}
          metricsLoading={metrics.isFetching}
          algorithmActive={selected?.algorithm_active ?? false}
          onModelChange={setModel}
          onGroupChange={setSelectedGroup}
        />
      </div>
    </div>
  )
}
