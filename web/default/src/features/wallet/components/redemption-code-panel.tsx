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
import { ExternalLink, Gift, Loader2 } from 'lucide-react'
import { useTranslation } from 'react-i18next'
import { cn } from '@/lib/utils'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'

interface RedemptionCodePanelProps {
  title?: string
  description?: string
  topupLink?: string
  redemptionCode: string
  onRedemptionCodeChange: (code: string) => void
  onRedeem: () => void
  redeeming: boolean
  className?: string
  compact?: boolean
}

export function RedemptionCodePanel(props: RedemptionCodePanelProps) {
  const { t } = useTranslation()

  return (
    <div className={cn('app-page-shell p-4', props.className)}>
      <div className='flex flex-wrap items-start justify-between gap-3'>
        <div className='min-w-0'>
          <div className='text-foreground flex items-center gap-2 text-sm font-semibold'>
            <Gift className='text-primary h-4 w-4' />
            {props.title || t('Redemption code')}
          </div>
          <div className='text-muted-foreground mt-1 max-w-2xl text-xs leading-5'>
            {props.description ||
              t(
                'Enter a code issued by an administrator or purchased offline. Successful redemption immediately adds the associated balance, plan, or benefit.'
              )}
          </div>
        </div>
        {props.topupLink ? (
          <Button
            variant='outline'
            size='sm'
            render={
              <a
                href={props.topupLink}
                target='_blank'
                rel='noopener noreferrer'
              />
            }
          >
            {t('Get a redemption code')}
            <ExternalLink data-icon='inline-end' />
          </Button>
        ) : null}
      </div>

      <div
        className={cn(
          'mt-4 grid gap-2',
          props.compact
            ? 'grid-cols-[minmax(0,1fr)_auto]'
            : 'sm:grid-cols-[minmax(0,1fr)_auto]'
        )}
      >
        <Input
          value={props.redemptionCode}
          onChange={(event) => props.onRedemptionCodeChange(event.target.value)}
          placeholder={t('Enter redemption code')}
          className='h-10 min-w-0'
        />
        <Button
          onClick={props.onRedeem}
          disabled={props.redeeming}
          className='h-10 px-5'
        >
          {props.redeeming ? (
            <Loader2 className='h-4 w-4 animate-spin' />
          ) : (
            t('Redeem now')
          )}
        </Button>
      </div>
    </div>
  )
}
