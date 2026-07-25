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
import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from '@/components/ui/card'
import { Input } from '@/components/ui/input'
import { Skeleton } from '@/components/ui/skeleton'
import type { RoutePoolGroup, RoutePoolMetrics } from '../types'

type RoutePoolInsightsProps = {
  groups: RoutePoolGroup[]
  model: string
  selectedGroup: string | null
  metrics?: RoutePoolMetrics
  metricsLoading: boolean
  algorithmActive: boolean
  onModelChange: (model: string) => void
  onGroupChange: (group: string) => void
}

export function RoutePoolInsights({
  groups,
  model,
  selectedGroup,
  metrics,
  metricsLoading,
  algorithmActive,
  onModelChange,
  onGroupChange,
}: RoutePoolInsightsProps) {
  return (
    <Card>
      <CardHeader>
        <CardTitle>路由算法验证</CardTitle>
        <CardDescription>
          这里显示服务端实际使用的候选评分。只有“算法已接管”的分组会忽略旧优先级和权重。
        </CardDescription>
      </CardHeader>
      <CardContent className='space-y-3'>
        <Input
          value={model}
          placeholder='输入模型，例如 gpt-5.6-luna'
          onChange={(event) => onModelChange(event.target.value)}
        />
        <div className='flex flex-wrap gap-2'>
          {groups.map((group) => (
            <button
              key={group.group}
              className={
                selectedGroup === group.group
                  ? 'bg-primary text-primary-foreground rounded-md px-3 py-1.5 text-sm'
                  : 'border-input hover:bg-accent rounded-md border px-3 py-1.5 text-sm'
              }
              onClick={() => onGroupChange(group.group)}
            >
              {group.group}
            </button>
          ))}
        </div>
        {!algorithmActive && selectedGroup && (
          <div className='rounded-md border border-amber-500/40 bg-amber-500/10 p-3 text-sm'>
            当前分组仍使用旧优先级 /
            权重路由。启用该分组的自动路由并保存后，才会使用成本、成功率、冷却和首字评分。
          </div>
        )}
        {algorithmActive && !model && (
          <p className='text-muted-foreground text-sm'>
            输入模型后查看该模型的实际候选、健康度和路由评分。
          </p>
        )}
        {metricsLoading && <Skeleton className='h-20' />}
        {metrics?.members.map((member) => (
          <div key={member.channel_id} className='border-b py-2 last:border-0'>
            <div className='flex justify-between gap-2'>
              <span className='truncate font-medium'>
                {member.channel_name || `渠道 #${member.channel_id}`}
              </span>
              <span className='font-mono text-xs'>
                {member.score.toFixed(3)}
              </span>
            </div>
            <div className='text-muted-foreground mt-1 flex flex-wrap gap-x-2 gap-y-1 text-xs'>
              <span>{member.eligible ? '候选可用' : '当前排除'}</span>
              <span>{member.health.state || 'unknown'}</span>
              <span>{member.health.success_rate_5m?.toFixed(1) ?? '0.0'}%</span>
              <span>P95 {Math.round(member.health.ttft_p95_ms || 0)}ms</span>
            </div>
          </div>
        ))}
      </CardContent>
    </Card>
  )
}
