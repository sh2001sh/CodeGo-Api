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
import { Activity, CreditCard, History, ReceiptText } from 'lucide-react'
import { useTranslation } from 'react-i18next'
import { formatUsdAmount, quotaUnitsToUsd } from '@/lib/format'
import { Button } from '@/components/ui/button'
import type { UserWalletData } from '../types'

export function WalletAccountOverview(props: {
  user: UserWalletData | null
  activeSubscriptionCount: number
  onSelectFunding: () => void
  onOpenBillingHistory: () => void
}) {
  const { t } = useTranslation()

  return (
    <section className='app-page-shell p-4 sm:p-5'>
      <div className='flex flex-col gap-4 lg:flex-row lg:items-center lg:justify-between'>
        <div className='grid min-w-0 gap-3 sm:grid-cols-2 lg:min-w-[28rem]'>
          <BalanceItem
            label={t('Wallet balance')}
            value={formatUsdAmount(quotaUnitsToUsd(props.user?.quota ?? 0))}
            description={t('Shared balance for all configured API providers')}
          />
          <BalanceItem
            label={t('Total spent')}
            value={formatUsdAmount(
              quotaUnitsToUsd(props.user?.used_quota ?? 0)
            )}
            description={t('Usage settled across your API requests')}
          />
        </div>

        <div className='flex flex-col gap-3 lg:items-end'>
          <div className='text-muted-foreground flex flex-wrap gap-x-4 gap-y-1 text-xs'>
            <span className='inline-flex items-center gap-1'>
              <Activity className='size-3.5' />
              {(props.user?.request_count ?? 0).toLocaleString()}{' '}
              {t('API requests')}
            </span>
            <span>
              {t('Active subscriptions')}: {props.activeSubscriptionCount}
            </span>
          </div>
          <div className='flex flex-wrap gap-2'>
            <Button type='button' onClick={props.onSelectFunding}>
              <CreditCard className='size-4' />
              {t('Top up')}
            </Button>
            <Button
              type='button'
              variant='outline'
              onClick={props.onOpenBillingHistory}
            >
              <History className='size-4' />
              {t('Records')}
              <ReceiptText className='size-3.5' />
            </Button>
          </div>
        </div>
      </div>
    </section>
  )
}

function BalanceItem(props: {
  label: string
  value: string
  description: string
}) {
  return (
    <div className='min-w-0'>
      <div className='text-muted-foreground text-xs font-medium'>
        {props.label}
      </div>
      <div className='text-foreground mt-1 truncate text-2xl font-semibold tracking-tight tabular-nums'>
        {props.value}
      </div>
      <div className='text-muted-foreground mt-0.5 text-xs'>
        {props.description}
      </div>
    </div>
  )
}
