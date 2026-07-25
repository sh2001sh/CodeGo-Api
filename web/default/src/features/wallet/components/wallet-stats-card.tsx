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
import { Activity, WalletCards } from 'lucide-react'
import { useTranslation } from 'react-i18next'
import { formatUsdAmount, quotaUnitsToUsd } from '@/lib/format'
import { Skeleton } from '@/components/ui/skeleton'
import type {
  PlanRecord,
  SelfSubscriptionData,
} from '@/features/subscriptions/types'
import type { UserWalletData } from '../types'
import { WalletStatItem } from './wallet-panel-primitives'

interface WalletStatsCardProps {
  user: UserWalletData | null
  plans: PlanRecord[]
  loading?: boolean
  subscriptionData?: SelfSubscriptionData | null
}

export function WalletStatsCard(props: WalletStatsCardProps) {
  const { t } = useTranslation()
  const activeSubscriptions = props.subscriptionData?.subscriptions || []

  if (props.loading) {
    return (
      <aside className='space-y-4 lg:sticky lg:top-4'>
        <div className='app-page-shell p-4'>
          <Skeleton className='h-5 w-28' />
          <Skeleton className='mt-3 h-10 w-full' />
          <Skeleton className='mt-3 h-10 w-full' />
        </div>
      </aside>
    )
  }

  return (
    <aside className='space-y-4 lg:sticky lg:top-4'>
      <div className='app-page-shell p-4'>
        <div className='text-foreground flex items-center gap-2 text-sm font-semibold'>
          <WalletCards className='text-primary h-4 w-4' />
          {t('Wallet balance')}
        </div>
        <div className='text-foreground mt-3 font-mono text-3xl font-bold tracking-tight tabular-nums'>
          {formatUsdAmount(quotaUnitsToUsd(props.user?.quota ?? 0))}
        </div>
        <div className='mt-4 grid gap-2'>
          <WalletStatItem
            label={t('Total spent')}
            value={formatUsdAmount(
              quotaUnitsToUsd(props.user?.used_quota ?? 0)
            )}
          />
          <WalletStatItem
            label={t('API requests')}
            value={(props.user?.request_count ?? 0).toLocaleString()}
            icon={<Activity className='text-muted-foreground h-4 w-4' />}
          />
          <WalletStatItem
            label={t('Active subscriptions')}
            value={`${activeSubscriptions.length}`}
          />
          <WalletStatItem
            label={t('Available plans')}
            value={`${props.plans.length}`}
          />
        </div>
      </div>
    </aside>
  )
}
