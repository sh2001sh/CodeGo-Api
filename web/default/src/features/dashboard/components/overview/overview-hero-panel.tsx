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
import { KeyRound, LinkIcon, Package } from 'lucide-react'
import { getConfiguredServerAddress } from '@/lib/server-url'
import { Button } from '@/components/ui/button'
import { CopyButton } from '@/components/copy-button'

function EndpointRow(props: {
  label: string
  value: string
  copyLabel: string
}) {
  return (
    <div className='bg-background/72 flex min-w-0 items-center gap-2 rounded-xl border border-white/60 px-3 py-2 dark:border-white/10'>
      <span className='text-muted-foreground shrink-0 text-[11px] font-medium'>
        {props.label}
      </span>
      <code
        className='text-foreground min-w-0 flex-1 truncate font-mono text-[11px]'
        title={props.value}
      >
        {props.value}
      </code>
      <CopyButton
        value={props.value}
        variant='ghost'
        size='sm'
        className='h-6 px-2 text-[11px]'
        tooltip={props.copyLabel}
        successTooltip='已复制'
        aria-label={props.copyLabel}
      >
        复制
      </CopyButton>
    </div>
  )
}

export function OverviewHeroPanel() {
  const serverAddress = getConfiguredServerAddress()
  const openAIEndpoint = `${serverAddress}/v1`

  return (
    <section className='overview-hero-card p-5 sm:p-6 xl:p-7'>
      <div className='grid gap-5 xl:grid-cols-[minmax(0,1fr)_minmax(380px,420px)] xl:items-start'>
        <div className='flex min-w-0 flex-col gap-5'>
          <div className='space-y-3'>
            <div className='text-primary text-xs font-semibold tracking-[0.16em] uppercase'>
              API 统一管理平台
            </div>
            <h2 className='max-w-xl text-3xl font-semibold tracking-tight text-balance sm:text-4xl xl:text-5xl'>
              统一管理模型、渠道与调用额度
            </h2>
            <p className='text-muted-foreground max-w-xl text-sm leading-7 sm:text-[15px]'>
              在一个控制台中维护 API
              密钥、模型路由、渠道状态和用量记录，让团队接入与运营保持清晰可追踪。
            </p>
          </div>

          <div className='flex flex-wrap items-center gap-2'>
            <Button variant='outline' render={<Link to='/wallet' />}>
              <KeyRound data-icon='inline-start' />
              查看钱包
            </Button>
            <Button variant='outline' render={<Link to='/packages' />}>
              <Package data-icon='inline-start' />
              查看套餐
            </Button>
          </div>
        </div>

        <div className='overview-soft-card flex min-w-0 flex-col gap-4 p-5'>
          <div className='flex items-center gap-2.5'>
            <span className='bg-primary/10 text-primary flex size-10 shrink-0 items-center justify-center rounded-xl'>
              <LinkIcon className='size-5' aria-hidden='true' />
            </span>
            <div className='min-w-0'>
              <div className='text-base font-semibold'>API 请求地址</div>
              <div className='text-muted-foreground text-xs'>
                创建 API 密钥后即可开始请求
              </div>
            </div>
          </div>

          <div className='space-y-3'>
            <EndpointRow
              label='OpenAI'
              value={openAIEndpoint}
              copyLabel='复制 OpenAI 格式地址'
            />
            <EndpointRow
              label='Anthropic'
              value={serverAddress}
              copyLabel='复制 Anthropic 格式地址'
            />
          </div>

          <Button variant='outline' render={<Link to='/keys' />}>
            <KeyRound data-icon='inline-start' />
            管理 API 密钥
          </Button>
        </div>
      </div>
    </section>
  )
}
