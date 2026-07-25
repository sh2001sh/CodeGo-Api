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
import type { Dispatch, SetStateAction } from 'react'
import { Plus, Save, Trash2 } from 'lucide-react'
import { Button } from '@/components/ui/button'
import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from '@/components/ui/card'
import { Combobox } from '@/components/ui/combobox'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { Switch } from '@/components/ui/switch'
import { Textarea } from '@/components/ui/textarea'
import type { Channel } from '@/features/channels/types'
import type { RoutePoolDraft, RoutePoolMember } from '../types'

type RoutePoolEditorProps = {
  draft: RoutePoolDraft
  setDraft: Dispatch<SetStateAction<RoutePoolDraft>>
  channels: Channel[]
  channelsLoading: boolean
  saving: boolean
  deleting: boolean
  onSave: () => void
  onDelete: () => void
}

const createMember = (): RoutePoolMember => ({
  channel_id: 0,
  cost_multiplier: 1,
  model_cost_overrides: '{}',
  enabled: true,
})

export function RoutePoolEditor({
  draft,
  setDraft,
  channels,
  channelsLoading,
  saving,
  deleting,
  onSave,
  onDelete,
}: RoutePoolEditorProps) {
  const channelOptions = channels
    .filter(
      (channel) =>
        !draft.group ||
        (typeof channel.group === 'string' &&
          channel.group.split(',').includes(draft.group))
    )
    .map((channel) => ({
      value: String(channel.id),
      label: `#${channel.id} ${channel.name}`,
    }))

  const updateMember = (index: number, patch: Partial<RoutePoolMember>) => {
    setDraft((current) => ({
      ...current,
      members: current.members.map((member, memberIndex) =>
        memberIndex === index ? { ...member, ...patch } : member
      ),
    }))
  }

  return (
    <Card>
      <CardHeader>
        <CardTitle>{draft.id ? '编辑自动池' : '新建自动池'}</CardTitle>
        <CardDescription>
          每个分组只能有一个自动池。采购倍率只供 Root 的路由和报表使用。
        </CardDescription>
      </CardHeader>
      <CardContent className='space-y-4'>
        <div className='grid gap-3 sm:grid-cols-2'>
          <div className='space-y-1.5'>
            <Label htmlFor='pool-name'>名称</Label>
            <Input
              id='pool-name'
              value={draft.name}
              onChange={(event) =>
                setDraft({ ...draft, name: event.target.value })
              }
            />
          </div>
          <div className='space-y-1.5'>
            <Label htmlFor='pool-group'>分组</Label>
            <Input
              id='pool-group'
              value={draft.group}
              onChange={(event) =>
                setDraft({ ...draft, group: event.target.value })
              }
            />
          </div>
        </div>

        <div className='flex items-center justify-between border-y py-3'>
          <div>
            <p className='font-medium'>启用自动选择</p>
            <p className='text-muted-foreground text-xs'>
              启用后，此分组仅使用已启用的池成员进行自动路由。
            </p>
          </div>
          <Switch
            checked={draft.enabled}
            onCheckedChange={(enabled) => setDraft({ ...draft, enabled })}
          />
        </div>

        <div className='space-y-2'>
          <div className='flex items-center justify-between'>
            <Label>成员渠道</Label>
            <Button
              size='sm'
              variant='outline'
              disabled={channelsLoading}
              onClick={() =>
                setDraft({
                  ...draft,
                  members: [...draft.members, createMember()],
                })
              }
            >
              <Plus />
              添加渠道
            </Button>
          </div>
          {draft.members.map((member, index) => (
            <div
              key={`${member.channel_id}-${index}`}
              className='grid gap-2 rounded-lg border p-3 md:grid-cols-[112px_132px_minmax(0,1fr)_auto_auto]'
            >
              <Combobox
                aria-label='渠道'
                options={channelOptions}
                value={member.channel_id ? String(member.channel_id) : ''}
                placeholder={channelsLoading ? '正在加载渠道' : '选择渠道'}
                searchPlaceholder='按 ID 或名称搜索渠道'
                emptyText={draft.group ? '该分组没有可选渠道' : '没有可选渠道'}
                onValueChange={(value) =>
                  updateMember(index, { channel_id: Number(value) || 0 })
                }
              />
              <Input
                aria-label='采购倍率'
                type='number'
                min='0.0001'
                step='0.0001'
                value={member.cost_multiplier}
                onChange={(event) =>
                  updateMember(index, {
                    cost_multiplier: Number(event.target.value),
                  })
                }
              />
              <Textarea
                aria-label='模型覆盖倍率 JSON'
                className='min-h-8 resize-y py-1.5 text-xs'
                value={member.model_cost_overrides}
                onChange={(event) =>
                  updateMember(index, {
                    model_cost_overrides: event.target.value,
                  })
                }
              />
              <Switch
                aria-label='启用渠道成员'
                checked={member.enabled}
                onCheckedChange={(enabled) => updateMember(index, { enabled })}
              />
              <Button
                size='icon-sm'
                variant='ghost'
                aria-label='移除渠道成员'
                onClick={() =>
                  setDraft({
                    ...draft,
                    members: draft.members.filter(
                      (_, memberIndex) => memberIndex !== index
                    ),
                  })
                }
              >
                <Trash2 />
              </Button>
            </div>
          ))}
          {draft.members.length === 0 && (
            <p className='text-muted-foreground rounded-lg border border-dashed px-3 py-5 text-sm'>
              添加可用渠道后，系统才会接管该分组的请求选择。
            </p>
          )}
        </div>

        <div className='flex justify-end gap-2'>
          {draft.id > 0 && (
            <Button
              variant='destructive'
              onClick={onDelete}
              disabled={deleting}
            >
              <Trash2 />
              删除
            </Button>
          )}
          <Button onClick={onSave} disabled={saving}>
            <Save />
            保存自动池
          </Button>
        </div>
      </CardContent>
    </Card>
  )
}
