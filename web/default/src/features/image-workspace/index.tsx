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
import { useEffect, useState } from 'react'
import dayjs from 'dayjs'
import { useQuery } from '@tanstack/react-query'
import { Link } from '@tanstack/react-router'
import {
  AlertCircle,
  Clock3,
  Download,
  History,
  ImagePlus,
  Images,
  PenSquare,
  RefreshCw,
  Sparkles,
  WandSparkles,
} from 'lucide-react'
import { nanoid } from 'nanoid'
import { toast } from 'sonner'
import { Alert, AlertDescription, AlertTitle } from '@/components/ui/alert'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import {
  Empty,
  EmptyDescription,
  EmptyHeader,
  EmptyMedia,
  EmptyTitle,
} from '@/components/ui/empty'
import { Label } from '@/components/ui/label'
import {
  Select,
  SelectContent,
  SelectGroup,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select'
import { Separator } from '@/components/ui/separator'
import { Skeleton } from '@/components/ui/skeleton'
import { Tabs, TabsContent, TabsList, TabsTrigger } from '@/components/ui/tabs'
import { Textarea } from '@/components/ui/textarea'
import { ImageDialog } from '@/features/usage-logs/components/dialogs/image-dialog'
import {
  fetchImageWorkspaceSourceFile,
  getImageWorkspaceGroups,
  getImageWorkspaceItems,
  getImageWorkspaceModels,
  submitImageEdit,
  submitImageGeneration,
} from './api'
import type {
  GroupOption,
  ImageWorkspaceFormState,
  ImageWorkspaceItem,
  ModelOption,
} from './types'

const SESSION_STORAGE_KEY = 'image_workspace_session_id'

const SIZE_OPTIONS = [
  '1024x1024',
  '1024x1536',
  '1536x1024',
  '1792x1024',
  '1024x1792',
]

const GPT_IMAGE_2_SIZE_OPTIONS = ['1024x1024', '1536x1024']

const QUALITY_OPTIONS = [
  { value: 'standard', label: '标准' },
  { value: 'hd', label: '高清' },
]

const COUNT_OPTIONS = ['1', '2', '3', '4']

function createSessionId() {
  return `imgs_${Date.now()}_${nanoid(8)}`
}

function getSizeOptionsForModel(model: string) {
  return model.toLowerCase().includes('gpt-image-2')
    ? GPT_IMAGE_2_SIZE_OPTIONS
    : SIZE_OPTIONS
}

function createBatchId() {
  return `imgb_${Date.now()}_${nanoid(8)}`
}

function loadSessionId() {
  if (typeof window === 'undefined') {
    return createSessionId()
  }
  const stored = window.localStorage.getItem(SESSION_STORAGE_KEY)
  if (stored) {
    return stored
  }
  const next = createSessionId()
  window.localStorage.setItem(SESSION_STORAGE_KEY, next)
  return next
}

function saveSessionId(sessionId: string) {
  if (typeof window !== 'undefined') {
    window.localStorage.setItem(SESSION_STORAGE_KEY, sessionId)
  }
}

function getStatusVariant(status: ImageWorkspaceItem['status']) {
  if (status === 'ready') return 'default'
  if (status === 'expired') return 'outline'
  return 'destructive'
}

function getStatusLabel(status: ImageWorkspaceItem['status']) {
  if (status === 'ready') return '已完成'
  if (status === 'expired') return '已过期'
  return '生成失败'
}

function dedupeItems(items: ImageWorkspaceItem[]) {
  const seen = new Set<number>()
  return items.filter((item) => {
    if (seen.has(item.id)) return false
    seen.add(item.id)
    return true
  })
}

function InfoPill(props: { label: string; value: string }) {
  return (
    <div className='border-border bg-muted/40 rounded-2xl border px-4 py-3'>
      <div className='text-muted-foreground text-xs'>{props.label}</div>
      <div className='text-foreground mt-1 text-sm font-semibold'>
        {props.value}
      </div>
    </div>
  )
}

function ImageWorkspaceSkeleton() {
  return (
    <div className='grid gap-4 xl:grid-cols-[minmax(0,1fr)_28rem]'>
      <Skeleton className='h-[46rem] rounded-[32px]' />
      <Skeleton className='h-[46rem] rounded-[32px]' />
    </div>
  )
}

export function ImageWorkspace() {
  const [sessionId, setSessionId] = useState(loadSessionId)
  const [galleryTab, setGalleryTab] = useState('session')
  const [selectedSourceId, setSelectedSourceId] = useState<number | null>(null)
  const [previewItem, setPreviewItem] = useState<ImageWorkspaceItem | null>(
    null
  )
  const [isSubmitting, setIsSubmitting] = useState(false)
  const [form, setForm] = useState<ImageWorkspaceFormState>({
    mode: 'generate',
    model: '',
    group: '',
    prompt: '',
    size: SIZE_OPTIONS[0],
    quality: QUALITY_OPTIONS[0].value,
    count: COUNT_OPTIONS[0],
  })

  const modelsQuery = useQuery({
    queryKey: ['image-workspace-models', form.group],
    queryFn: () => getImageWorkspaceModels(form.group),
    enabled: Boolean(form.group),
  })

  const groupsQuery = useQuery({
    queryKey: ['image-workspace-groups'],
    queryFn: getImageWorkspaceGroups,
  })

  const sessionItemsQuery = useQuery({
    queryKey: ['image-workspace-items', sessionId],
    queryFn: () =>
      getImageWorkspaceItems({
        sessionId,
        page: 1,
        pageSize: 60,
      }),
  })

  const recentItemsQuery = useQuery({
    queryKey: ['image-workspace-items-recent'],
    queryFn: () =>
      getImageWorkspaceItems({
        page: 1,
        pageSize: 60,
      }),
  })

  useEffect(() => {
    const models = modelsQuery.data ?? []
    const currentModelAvailable = models.some(
      (item) => item.value === form.model
    )
    if (models.length > 0 && !currentModelAvailable) {
      const preferredModel =
        models.find((item) => item.value === 'gpt-image-2')?.value ??
        models[0].value
      setForm((prev) => ({ ...prev, model: preferredModel }))
    } else if (models.length === 0 && form.model) {
      setForm((prev) => ({ ...prev, model: '' }))
    }
  }, [form.model, modelsQuery.data])

  useEffect(() => {
    const groups = groupsQuery.data ?? []
    if (!form.group && groups.length > 0) {
      const preferredGroup =
        groups.find((item) => item.value === 'default')?.value ??
        groups[0].value
      setForm((prev) => ({ ...prev, group: preferredGroup }))
    }
  }, [form.group, groupsQuery.data])

  const sessionItems = sessionItemsQuery.data?.items ?? []
  const recentItems = recentItemsQuery.data?.items ?? []
  const readySourceItems = dedupeItems(
    [...sessionItems, ...recentItems].filter(
      (item) => item.status === 'ready' && item.image_url
    )
  )
  const selectedSource =
    readySourceItems.find((item) => item.id === selectedSourceId) ?? null
  const selectedGroup =
    (groupsQuery.data ?? []).find((item) => item.value === form.group) ?? null
  const sizeOptions = getSizeOptionsForModel(form.model)

  useEffect(() => {
    if (form.mode !== 'edit') return
    if (selectedSource) return
    if (readySourceItems.length > 0) {
      setSelectedSourceId(readySourceItems[0].id)
    }
  }, [form.mode, readySourceItems, selectedSource])

  useEffect(() => {
    const nextSizeOptions = getSizeOptionsForModel(form.model)
    if (!nextSizeOptions.includes(form.size)) {
      setForm((prev) => ({ ...prev, size: nextSizeOptions[0] }))
    }
  }, [form.model, form.size])

  const updateForm = <K extends keyof ImageWorkspaceFormState>(
    key: K,
    value: ImageWorkspaceFormState[K]
  ) => {
    setForm((prev) => ({ ...prev, [key]: value }))
  }

  const handleNewSession = () => {
    const nextSessionId = createSessionId()
    setSessionId(nextSessionId)
    saveSessionId(nextSessionId)
    setGalleryTab('session')
    setSelectedSourceId(null)
  }

  const handleSubmit = async () => {
    if (!form.model) {
      toast.error('请先选择模型')
      return
    }
    if (!form.group) {
      toast.error('请先选择分组')
      return
    }
    if (!form.prompt.trim()) {
      toast.error('请先输入提示词')
      return
    }
    if (form.mode === 'edit' && !selectedSource) {
      toast.error('改图模式需要先选择一张来源图片')
      return
    }

    setIsSubmitting(true)
    try {
      const payload = {
        model: form.model,
        prompt: form.prompt.trim(),
        size: form.size,
        quality: form.quality,
        n: Number(form.count),
      }
      const batchId = createBatchId()

      if (form.mode === 'generate') {
        await submitImageGeneration(payload, {
          group: form.group,
          sessionId,
          batchId,
        })
      } else if (selectedSource) {
        const imageFile = await fetchImageWorkspaceSourceFile(selectedSource)
        await submitImageEdit(
          payload,
          {
            group: form.group,
            sessionId,
            batchId,
            sourceItemId: selectedSource.id,
          },
          imageFile
        )
      }

      await Promise.all([
        sessionItemsQuery.refetch(),
        recentItemsQuery.refetch(),
      ])
      setGalleryTab('session')
      toast.success(form.mode === 'generate' ? '图片生成完成' : '改图完成')
    } catch (error) {
      const err = error as {
        response?: { data?: { error?: { message?: string }; message?: string } }
        message?: string
      }
      const message =
        err?.response?.data?.error?.message ||
        err?.response?.data?.message ||
        err?.message ||
        '图片请求失败'
      toast.error(message)
    } finally {
      setIsSubmitting(false)
    }
  }

  const applyItemPrompt = (item: ImageWorkspaceItem) => {
    setForm((prev) => ({
      ...prev,
      prompt: item.revised_prompt || item.prompt,
    }))
  }

  const startEditFromItem = (item: ImageWorkspaceItem) => {
    setSelectedSourceId(item.id)
    setForm((prev) => ({
      ...prev,
      mode: 'edit',
      prompt: item.revised_prompt || item.prompt,
      model: item.model || prev.model,
    }))
  }

  const refreshGallery = async () => {
    await Promise.all([sessionItemsQuery.refetch(), recentItemsQuery.refetch()])
  }

  const isLoading =
    modelsQuery.isLoading ||
    groupsQuery.isLoading ||
    sessionItemsQuery.isLoading ||
    recentItemsQuery.isLoading

  if (isLoading && !modelsQuery.data && !groupsQuery.data) {
    return <ImageWorkspaceSkeleton />
  }

  const models = modelsQuery.data ?? []
  const groups = groupsQuery.data ?? []

  return (
    <>
      <div className='min-h-full bg-[radial-gradient(circle_at_top_left,rgba(14,165,233,0.16),transparent_26%),radial-gradient(circle_at_top_right,rgba(245,158,11,0.12),transparent_22%),linear-gradient(180deg,rgba(15,23,42,0.03),transparent_30%),var(--background)] p-4 md:p-6'>
        <div className='mx-auto max-w-[1580px] space-y-5'>
          {form.group && !modelsQuery.isLoading && models.length === 0 && (
            <Alert className='border-amber-500/40 bg-amber-500/5'>
              <AlertCircle className='text-amber-600' />
              <AlertTitle>当前分组没有可用的生图模型</AlertTitle>
              <AlertDescription>
                {form.group === 'auto'
                  ? '请检查后台 AutoGroups 是否包含生图模型所在分组、用户分组权限和模型计费配置。'
                  : '请检查该分组的渠道模型、用户权限和模型计费配置。'}
              </AlertDescription>
            </Alert>
          )}
          <div className='flex flex-wrap items-center justify-between gap-3 px-1'>
            <div>
              <div className='text-muted-foreground text-xs font-medium tracking-[0.28em]'>
                生图工作台
              </div>
              <h1 className='text-foreground mt-2 text-2xl font-semibold tracking-tight'>
                生成、编辑和管理图片
              </h1>
              <p className='text-muted-foreground mt-1 text-sm'>
                选择模型与参数，输入中文提示词即可开始创作。
              </p>
            </div>
            <div className='flex flex-wrap gap-2'>
              <Button variant='outline' onClick={handleNewSession}>
                <RefreshCw data-icon='inline-start' />
                新建会话
              </Button>
              <Button
                render={
                  <Link
                    to='/usage-logs/$section'
                    params={{ section: 'common' }}
                  />
                }
              >
                <Clock3 data-icon='inline-start' />
                查看日志
              </Button>
            </div>
          </div>

          <div className='grid gap-4 xl:grid-cols-[minmax(0,1fr)_28rem]'>
            <div className='border-border bg-card rounded-[30px] border p-5 shadow-sm'>
              <div className='text-foreground flex items-center gap-2 text-base font-semibold'>
                <Sparkles className='size-4' />
                创作对话
              </div>
              <div className='text-muted-foreground mt-1 text-sm'>
                用中文描述需求，系统会按你当前选择的模型、分组和参数发起生图。
              </div>

              <div className='mt-5 space-y-5'>
                <Tabs
                  value={form.mode}
                  onValueChange={(value) =>
                    updateForm('mode', value as ImageWorkspaceFormState['mode'])
                  }
                >
                  <TabsList className='grid w-full grid-cols-2'>
                    <TabsTrigger value='generate'>
                      <WandSparkles className='size-4' />
                      生图模式
                    </TabsTrigger>
                    <TabsTrigger value='edit'>
                      <PenSquare className='size-4' />
                      改图模式
                    </TabsTrigger>
                  </TabsList>
                </Tabs>

                <div className='grid gap-2'>
                  <Label>提示词</Label>
                  <Textarea
                    value={form.prompt}
                    onChange={(event) =>
                      updateForm('prompt', event.target.value)
                    }
                    placeholder='例如：生成一张电影感的雨夜街头场景，主角撑着透明雨伞站在霓虹灯下，镜头低角度，反光地面细节丰富，整体偏青橙色调。'
                    className='min-h-40 rounded-3xl'
                  />
                  <div className='text-muted-foreground text-xs leading-6'>
                    写得越具体，结果越稳定。建议写清主体、环境、风格、镜头、构图和想保留的关键细节。
                  </div>
                </div>

                <div className='grid gap-4 md:grid-cols-2'>
                  <div className='grid gap-2'>
                    <Label>模型</Label>
                    <Select
                      items={models.map((item: ModelOption) => ({
                        value: item.value,
                        label: item.label,
                      }))}
                      value={form.model}
                      onValueChange={(value) =>
                        updateForm('model', value ?? '')
                      }
                    >
                      <SelectTrigger className='w-full rounded-2xl'>
                        <SelectValue placeholder='选择模型' />
                      </SelectTrigger>
                      <SelectContent alignItemWithTrigger={false}>
                        <SelectGroup>
                          {models.map((item) => (
                            <SelectItem key={item.value} value={item.value}>
                              {item.label}
                            </SelectItem>
                          ))}
                        </SelectGroup>
                      </SelectContent>
                    </Select>
                  </div>

                  <div className='grid gap-2'>
                    <Label>分组</Label>
                    <Select
                      items={groups.map((item: GroupOption) => ({
                        value: item.value,
                        label: item.label,
                      }))}
                      value={form.group}
                      onValueChange={(value) =>
                        updateForm('group', value ?? '')
                      }
                    >
                      <SelectTrigger className='w-full rounded-2xl'>
                        <SelectValue placeholder='选择分组' />
                      </SelectTrigger>
                      <SelectContent alignItemWithTrigger={false}>
                        <SelectGroup>
                          {groups.map((item) => (
                            <SelectItem key={item.value} value={item.value}>
                              <div className='flex items-center gap-2'>
                                <span>{item.label}</span>
                                <span className='text-muted-foreground text-xs'>
                                  倍率 x{item.ratio}
                                </span>
                              </div>
                            </SelectItem>
                          ))}
                        </SelectGroup>
                      </SelectContent>
                    </Select>
                    {selectedGroup?.desc ? (
                      <div className='text-muted-foreground text-xs leading-6'>
                        {selectedGroup.desc}
                      </div>
                    ) : null}
                  </div>
                </div>

                <div className='grid gap-4 md:grid-cols-3'>
                  <div className='grid gap-2'>
                    <Label>尺寸</Label>
                    <Select
                      items={sizeOptions.map((value) => ({
                        value,
                        label: value,
                      }))}
                      value={form.size}
                      onValueChange={(value) =>
                        updateForm('size', value ?? sizeOptions[0])
                      }
                    >
                      <SelectTrigger className='w-full rounded-2xl'>
                        <SelectValue />
                      </SelectTrigger>
                      <SelectContent alignItemWithTrigger={false}>
                        <SelectGroup>
                          {sizeOptions.map((value) => (
                            <SelectItem key={value} value={value}>
                              {value}
                            </SelectItem>
                          ))}
                        </SelectGroup>
                      </SelectContent>
                    </Select>
                  </div>

                  <div className='grid gap-2'>
                    <Label>清晰度</Label>
                    <Select
                      items={QUALITY_OPTIONS}
                      value={form.quality}
                      onValueChange={(value) =>
                        updateForm('quality', value ?? QUALITY_OPTIONS[0].value)
                      }
                    >
                      <SelectTrigger className='w-full rounded-2xl'>
                        <SelectValue />
                      </SelectTrigger>
                      <SelectContent alignItemWithTrigger={false}>
                        <SelectGroup>
                          {QUALITY_OPTIONS.map((item) => (
                            <SelectItem key={item.value} value={item.value}>
                              {item.label}
                            </SelectItem>
                          ))}
                        </SelectGroup>
                      </SelectContent>
                    </Select>
                  </div>

                  <div className='grid gap-2'>
                    <Label>生成张数</Label>
                    <Select
                      items={COUNT_OPTIONS.map((value) => ({
                        value,
                        label: value,
                      }))}
                      value={form.count}
                      onValueChange={(value) =>
                        updateForm('count', value ?? COUNT_OPTIONS[0])
                      }
                    >
                      <SelectTrigger className='w-full rounded-2xl'>
                        <SelectValue />
                      </SelectTrigger>
                      <SelectContent alignItemWithTrigger={false}>
                        <SelectGroup>
                          {COUNT_OPTIONS.map((value) => (
                            <SelectItem key={value} value={value}>
                              {value}
                            </SelectItem>
                          ))}
                        </SelectGroup>
                      </SelectContent>
                    </Select>
                  </div>
                </div>

                {form.mode === 'edit' && (
                  <div className='border-border bg-muted/40 rounded-[28px] border p-4'>
                    <div className='flex items-center justify-between gap-3'>
                      <div>
                        <div className='text-foreground text-sm font-semibold'>
                          来源图片
                        </div>
                        <div className='text-muted-foreground text-xs leading-6'>
                          先从历史图片中选一张作为改图基础，再输入你要继续调整的方向。
                        </div>
                      </div>
                      <Badge variant='outline'>
                        {readySourceItems.length} 张可选
                      </Badge>
                    </div>

                    <div className='mt-4 space-y-3'>
                      {selectedSource ? (
                        <div className='grid gap-3 md:grid-cols-[7.5rem_minmax(0,1fr)]'>
                          <img
                            src={selectedSource.image_url}
                            alt={selectedSource.prompt}
                            className='h-28 w-full rounded-2xl border object-cover'
                          />
                          <div className='space-y-2'>
                            <div className='text-foreground line-clamp-3 text-sm leading-6'>
                              {selectedSource.revised_prompt ||
                                selectedSource.prompt}
                            </div>
                            <div className='text-muted-foreground text-xs'>
                              {selectedSource.model} ·{' '}
                              {dayjs
                                .unix(selectedSource.created_at)
                                .format('MM-DD HH:mm')}
                            </div>
                            <div className='flex flex-wrap gap-2'>
                              <Button
                                variant='outline'
                                size='sm'
                                onClick={() => setPreviewItem(selectedSource)}
                              >
                                预览
                              </Button>
                              <Button
                                variant='outline'
                                size='sm'
                                onClick={() => applyItemPrompt(selectedSource)}
                              >
                                复用提示词
                              </Button>
                            </div>
                          </div>
                        </div>
                      ) : (
                        <div className='text-muted-foreground text-sm'>
                          暂时没有可用来源图片。你可以先生成一张，再回来做改图。
                        </div>
                      )}

                      {readySourceItems.length > 0 ? (
                        <div className='grid grid-cols-3 gap-2 md:grid-cols-4'>
                          {readySourceItems.slice(0, 8).map((item) => (
                            <button
                              key={item.id}
                              type='button'
                              onClick={() => setSelectedSourceId(item.id)}
                              className={`overflow-hidden rounded-2xl border text-left transition ${
                                selectedSourceId === item.id
                                  ? 'border-emerald-500 ring-4 ring-emerald-500/15'
                                  : 'hover:border-slate-300'
                              }`}
                            >
                              <img
                                src={item.image_url}
                                alt={item.prompt}
                                className='aspect-square w-full object-cover'
                              />
                            </button>
                          ))}
                        </div>
                      ) : null}
                    </div>
                  </div>
                )}

                <Separator />

                <div className='flex flex-col gap-3 sm:flex-row'>
                  <Button
                    size='lg'
                    className='flex-1'
                    onClick={handleSubmit}
                    disabled={isSubmitting || !form.model || !form.group}
                  >
                    {isSubmitting ? (
                      <>
                        <RefreshCw className='size-4 animate-spin' />
                        正在处理
                      </>
                    ) : (
                      <>
                        {form.mode === 'generate' ? (
                          <WandSparkles className='size-4' />
                        ) : (
                          <ImagePlus className='size-4' />
                        )}
                        {form.mode === 'generate' ? '开始生图' : '开始改图'}
                      </>
                    )}
                  </Button>
                  <Button
                    variant='outline'
                    size='lg'
                    onClick={() => updateForm('prompt', '')}
                    disabled={isSubmitting}
                  >
                    清空提示词
                  </Button>
                </div>
              </div>
            </div>

            <div className='border-border bg-card rounded-[30px] border p-5 shadow-sm'>
              <div className='flex items-end justify-between gap-3'>
                <div>
                  <div className='text-foreground flex items-center gap-2 text-base font-semibold'>
                    <Images className='size-4' />
                    作品记录
                  </div>
                  <div className='text-muted-foreground mt-1 text-sm'>
                    当前会话结果和最近历史都会保存在这里，直到自动清理为止。
                  </div>
                </div>
                <Button variant='outline' size='sm' onClick={refreshGallery}>
                  <RefreshCw className='size-4' />
                  刷新
                </Button>
              </div>

              <Tabs value={galleryTab} onValueChange={setGalleryTab}>
                <TabsList className='mt-5 grid w-full grid-cols-2'>
                  <TabsTrigger value='session'>
                    <Sparkles className='size-4' />
                    当前会话
                  </TabsTrigger>
                  <TabsTrigger value='recent'>
                    <History className='size-4' />
                    最近历史
                  </TabsTrigger>
                </TabsList>

                <TabsContent value='session' className='mt-4'>
                  <ImageGrid
                    items={sessionItems}
                    emptyTitle='当前会话还没有图片'
                    emptyDescription='左侧输入中文提示词后，新的结果会优先出现在这里。'
                    onPreview={setPreviewItem}
                    onReusePrompt={applyItemPrompt}
                    onEditFromItem={startEditFromItem}
                  />
                </TabsContent>

                <TabsContent value='recent' className='mt-4'>
                  <ImageGrid
                    items={recentItems}
                    emptyTitle='最近还没有图片历史'
                    emptyDescription='你最近生成的图片会暂时保存在这里，过期后会被自动清理。'
                    onPreview={setPreviewItem}
                    onReusePrompt={applyItemPrompt}
                    onEditFromItem={startEditFromItem}
                  />
                </TabsContent>
              </Tabs>

              <div className='mt-5 grid gap-3'>
                <InfoPill
                  label='当前会话图片数'
                  value={String(sessionItems.length)}
                />
                <InfoPill
                  label='最近历史图片数'
                  value={String(recentItems.length)}
                />
                <InfoPill
                  label='当前浏览标签'
                  value={galleryTab === 'session' ? '当前会话' : '最近历史'}
                />
                <InfoPill
                  label='当前选中来源图'
                  value={selectedSource ? `#${selectedSource.id}` : '未选择'}
                />
              </div>
            </div>
          </div>
        </div>
      </div>

      <ImageDialog
        imageUrl={previewItem?.image_url ?? ''}
        taskId={previewItem ? String(previewItem.id) : undefined}
        open={Boolean(previewItem)}
        onOpenChange={(open) => {
          if (!open) {
            setPreviewItem(null)
          }
        }}
      />
    </>
  )
}

function ImageGrid(props: {
  items: ImageWorkspaceItem[]
  emptyTitle: string
  emptyDescription: string
  onPreview: (item: ImageWorkspaceItem) => void
  onReusePrompt: (item: ImageWorkspaceItem) => void
  onEditFromItem: (item: ImageWorkspaceItem) => void
}) {
  if (props.items.length === 0) {
    return (
      <Empty className='min-h-72 rounded-[28px] border'>
        <EmptyHeader>
          <EmptyMedia variant='icon'>
            <Images className='size-4' />
          </EmptyMedia>
          <EmptyTitle>{props.emptyTitle}</EmptyTitle>
          <EmptyDescription>{props.emptyDescription}</EmptyDescription>
        </EmptyHeader>
      </Empty>
    )
  }

  return (
    <div className='space-y-3'>
      {props.items.map((item) => {
        const isReady = item.status === 'ready' && !!item.image_url
        const isExpired = item.status === 'expired'

        return (
          <div
            key={item.id}
            className='border-border bg-muted/40 rounded-[28px] border p-4'
          >
            <div className='flex gap-4'>
              {isReady ? (
                <button
                  type='button'
                  onClick={() => props.onPreview(item)}
                  className='overflow-hidden rounded-2xl border'
                >
                  <img
                    src={item.image_url}
                    alt={item.prompt}
                    className='size-28 object-cover'
                  />
                </button>
              ) : (
                <div className='bg-muted flex size-28 items-center justify-center rounded-2xl border'>
                  <div className='text-muted-foreground flex items-center gap-2 text-sm'>
                    <AlertCircle className='size-4' />
                    {isExpired ? '已过期' : '不可用'}
                  </div>
                </div>
              )}

              <div className='min-w-0 flex-1 space-y-3'>
                <div className='flex items-start justify-between gap-3'>
                  <div className='min-w-0'>
                    <div className='text-foreground line-clamp-3 text-sm leading-6'>
                      {item.revised_prompt || item.prompt}
                    </div>
                    <div className='text-muted-foreground mt-2 flex flex-wrap items-center gap-2 text-xs'>
                      <span>{item.model}</span>
                      <span>·</span>
                      <span>
                        {dayjs.unix(item.created_at).format('MM-DD HH:mm')}
                      </span>
                      {item.expires_at > 0 && !isExpired ? (
                        <>
                          <span>·</span>
                          <span>
                            到期{' '}
                            {dayjs.unix(item.expires_at).format('MM-DD HH:mm')}
                          </span>
                        </>
                      ) : null}
                    </div>
                  </div>
                  <Badge variant={getStatusVariant(item.status)}>
                    {getStatusLabel(item.status)}
                  </Badge>
                </div>

                {item.error_message && item.status !== 'ready' ? (
                  <div className='rounded-2xl border border-dashed border-red-200 px-3 py-2 text-xs leading-5 text-red-600 dark:border-red-500/20 dark:text-red-300'>
                    {item.error_message}
                  </div>
                ) : null}

                <div className='flex flex-wrap gap-2'>
                  {isReady ? (
                    <Button
                      variant='outline'
                      size='sm'
                      onClick={() => props.onPreview(item)}
                    >
                      预览
                    </Button>
                  ) : null}
                  {isReady && item.download_url ? (
                    <Button
                      variant='outline'
                      size='sm'
                      render={
                        <a href={item.download_url}>
                          <Download className='size-4' />
                          下载
                        </a>
                      }
                    />
                  ) : null}
                  <Button
                    variant='outline'
                    size='sm'
                    onClick={() => props.onReusePrompt(item)}
                  >
                    复用提示词
                  </Button>
                  {isReady ? (
                    <Button
                      variant='outline'
                      size='sm'
                      onClick={() => props.onEditFromItem(item)}
                    >
                      基于这张继续改
                    </Button>
                  ) : null}
                </div>
              </div>
            </div>
          </div>
        )
      })}
    </div>
  )
}
