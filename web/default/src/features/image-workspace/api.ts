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
import { api } from '@/lib/api'
import type { GroupOption, ImageWorkspaceItem, ModelOption } from './types'

interface ImageWorkspaceItemsResponse {
  success: boolean
  message?: string
  data?: ImageWorkspaceItem[]
  total?: number
}

export async function getImageWorkspaceModels(
  group?: string
): Promise<ModelOption[]> {
  const res = await api.get('/api/user/image-workspace/models', {
    params: group ? { group } : undefined,
  })
  const { data } = res
  if (!data.success || !Array.isArray(data.data)) {
    return []
  }
  return data.data.map((model: string) => ({
    label: model,
    value: model,
  }))
}

export async function getImageWorkspaceGroups(): Promise<GroupOption[]> {
  const res = await api.get('/api/user/self/groups')
  const { data } = res
  if (!data.success || !data.data) {
    return []
  }

  const groupData = data.data as Record<string, { desc: string; ratio: number }>
  return Object.entries(groupData).map(([group, info]) => ({
    label: group,
    value: group,
    ratio: Number(info.ratio),
    desc: info.desc,
  }))
}

export async function getImageWorkspaceItems(params?: {
  sessionId?: string
  page?: number
  pageSize?: number
}): Promise<{ items: ImageWorkspaceItem[]; total: number }> {
  const res = await api.get('/api/user/image-workspace/items', {
    params: {
      p: params?.page ?? 1,
      page_size: params?.pageSize ?? 60,
      session_id: params?.sessionId ?? '',
    },
  })
  const data = res.data as ImageWorkspaceItemsResponse
  return {
    items: data.success && Array.isArray(data.data) ? data.data : [],
    total: data.success ? Number(data.total ?? 0) : 0,
  }
}

export async function submitImageGeneration(
  payload: {
    model: string
    prompt: string
    size: string
    quality: string
    n: number
  },
  meta: {
    group: string
    sessionId: string
    batchId: string
  }
) {
  const res = await api.post(
    `/pg/images/generations?group=${encodeURIComponent(meta.group)}&session_id=${encodeURIComponent(meta.sessionId)}&batch_id=${encodeURIComponent(meta.batchId)}`,
    payload,
    {
      skipErrorHandler: true,
    } as Record<string, unknown>
  )
  return res.data
}

export async function submitImageEdit(
  payload: {
    model: string
    prompt: string
    size: string
    quality: string
    n: number
  },
  meta: {
    group: string
    sessionId: string
    batchId: string
    sourceItemId: number
  },
  imageFile: File
) {
  const formData = new FormData()
  formData.append('model', payload.model)
  formData.append('prompt', payload.prompt)
  formData.append('size', payload.size)
  formData.append('quality', payload.quality)
  formData.append('n', String(payload.n))
  formData.append('image', imageFile)

  const res = await api.post(
    `/pg/images/edits?group=${encodeURIComponent(meta.group)}&session_id=${encodeURIComponent(meta.sessionId)}&batch_id=${encodeURIComponent(meta.batchId)}&source_item_id=${meta.sourceItemId}`,
    formData,
    {
      headers: {
        'Content-Type': 'multipart/form-data',
      },
      skipErrorHandler: true,
    } as Record<string, unknown>
  )
  return res.data
}

export async function fetchImageWorkspaceSourceFile(
  item: ImageWorkspaceItem
): Promise<File> {
  const response = await fetch(item.image_url, {
    credentials: 'include',
  })
  if (!response.ok) {
    throw new Error('Failed to load source image')
  }
  const blob = await response.blob()
  const fileExtension = blob.type.includes('webp')
    ? 'webp'
    : blob.type.includes('jpeg')
      ? 'jpg'
      : blob.type.includes('gif')
        ? 'gif'
        : 'png'
  return new File([blob], `image-workspace-${item.id}.${fileExtension}`, {
    type: blob.type || 'image/png',
  })
}
