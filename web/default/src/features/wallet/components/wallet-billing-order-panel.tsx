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
import { ArrowDown, ArrowUp, RefreshCw, Save } from 'lucide-react'
import { useTranslation } from 'react-i18next'
import { cn } from '@/lib/utils'
import { Button } from '@/components/ui/button'
import { Skeleton } from '@/components/ui/skeleton'
import {
  getFundingSourceDescription,
  getFundingSourceLabel,
} from '@/features/subscriptions/billing'
import type {
  FundingSource,
  UserSubscriptionRecord,
} from '@/features/subscriptions/types'
import {
  formatWalletDateTime,
  getSubscriptionUsageStatus,
  getWalletRemainingDays,
  type WalletPlanMeta,
} from './wallet-panel-utils'

interface WalletBillingOrderPanelProps {
  draftFundingSourceOrder: FundingSource[]
  disabledFundingSources: FundingSource[]
  subscriptionModeEnabled: boolean
  hasActiveSubscriptions: boolean
  orderedSubscriptions: UserSubscriptionRecord[]
  planMetaMap: Map<number, WalletPlanMeta>
  saving: boolean
  isLoading: boolean
  subscriptionLoading?: boolean
  onRefresh: () => void
  onSave: () => void
  onResetFundingSourceOrder: () => void
  onResetSubscriptionOrder: () => void
  onToggleFundingSource: (source: FundingSource) => void
  onMoveFundingSource: (source: FundingSource, direction: -1 | 1) => void
  onMoveSubscription: (id: number, direction: -1 | 1) => void
}

export function WalletBillingOrderPanel(props: WalletBillingOrderPanelProps) {
  const { t } = useTranslation()

  return (
    <div className='app-page-shell p-4'>
      <div className='flex flex-wrap items-start justify-between gap-3'>
        <div>
          <div className='text-foreground text-sm font-semibold'>
            {t('Billing priority')}
          </div>
          <div className='text-muted-foreground mt-1 text-xs leading-5'>
            {t(
              'Subscription quota and wallet balance share one priority order, which you can adjust below.'
            )}
          </div>
        </div>
        <div className='flex gap-2'>
          <Button
            variant='outline'
            size='icon'
            className='h-8 w-8'
            onClick={props.onRefresh}
            disabled={props.isLoading || props.saving}
          >
            <RefreshCw
              className={cn(
                'h-4 w-4',
                (props.subscriptionLoading || props.saving) && 'animate-spin'
              )}
            />
          </Button>
          <Button onClick={props.onSave} disabled={props.saving}>
            <Save className='mr-1 h-4 w-4' />
            {t('Save settings')}
          </Button>
        </div>
      </div>

      <div className='mt-4 grid gap-4 xl:grid-cols-[340px_minmax(0,1fr)]'>
        <div className='space-y-3'>
          <div className='app-section-kicker'>{t('Billing source order')}</div>
          {props.draftFundingSourceOrder.map((source, index) => (
            <div key={source} className='app-subtle-panel px-3 py-3'>
              <div className='flex items-start justify-between gap-3'>
                <div className='min-w-0'>
                  <div className='text-foreground truncate text-sm font-semibold'>
                    {index + 1}. {getFundingSourceLabel(source, t)}
                  </div>
                  <div className='text-muted-foreground mt-1 text-xs'>
                    {getFundingSourceDescription(source, t)}
                  </div>
                </div>
                <div className='flex shrink-0 items-center gap-1'>
                  <Button
                    variant='outline'
                    size='sm'
                    onClick={() => props.onToggleFundingSource(source)}
                    disabled={props.saving}
                  >
                    {t('Disable')}
                  </Button>
                  <Button
                    variant='outline'
                    size='icon'
                    className='h-8 w-8'
                    onClick={() => props.onMoveFundingSource(source, -1)}
                    disabled={index === 0 || props.saving}
                  >
                    <ArrowUp className='h-4 w-4' />
                  </Button>
                  <Button
                    variant='outline'
                    size='icon'
                    className='h-8 w-8'
                    onClick={() => props.onMoveFundingSource(source, 1)}
                    disabled={
                      index === props.draftFundingSourceOrder.length - 1 ||
                      props.saving
                    }
                  >
                    <ArrowDown className='h-4 w-4' />
                  </Button>
                </div>
              </div>
            </div>
          ))}

          {props.disabledFundingSources.length > 0 ? (
            <div className='border-border/70 bg-background/60 text-muted-foreground rounded-2xl border border-dashed px-3 py-4 text-xs'>
              <div className='text-foreground font-medium'>
                {t('Disabled sources')}
              </div>
              <div className='mt-2 flex flex-wrap gap-2'>
                {props.disabledFundingSources.map((source) => (
                  <Button
                    key={source}
                    variant='outline'
                    size='sm'
                    onClick={() => props.onToggleFundingSource(source)}
                    disabled={props.saving}
                  >
                    {t('Enable {{source}}', {
                      source: getFundingSourceLabel(source, t),
                    })}
                  </Button>
                ))}
              </div>
            </div>
          ) : null}

          <div className='flex flex-wrap gap-2'>
            <Button
              variant='outline'
              className='flex-1'
              onClick={props.onResetFundingSourceOrder}
              disabled={props.saving}
            >
              {t('Reset source order')}
            </Button>
            <Button
              variant='outline'
              className='flex-1'
              onClick={props.onResetSubscriptionOrder}
              disabled={!props.hasActiveSubscriptions || props.saving}
            >
              {t('Reset subscription order')}
            </Button>
          </div>
        </div>

        <div className='space-y-3'>
          <div className='app-section-kicker'>
            {t('Subscription billing order')}
          </div>
          {props.isLoading ? (
            <div className='space-y-2'>
              <Skeleton className='h-14 rounded-2xl' />
              <Skeleton className='h-14 rounded-2xl' />
            </div>
          ) : !props.subscriptionModeEnabled ? (
            <div className='border-border/70 bg-background/60 text-muted-foreground rounded-2xl border border-dashed px-3 py-4 text-xs'>
              {t(
                'Subscription billing is disabled; all subscriptions will be skipped during settlement.'
              )}
            </div>
          ) : !props.hasActiveSubscriptions ? (
            <div className='border-border/70 bg-background/60 text-muted-foreground rounded-2xl border border-dashed px-3 py-4 text-xs'>
              {t('No active subscriptions can be reordered.')}
            </div>
          ) : (
            <div className='space-y-2'>
              {props.orderedSubscriptions.map((record, index) => {
                const subscription = record.subscription
                const meta = props.planMetaMap.get(subscription.plan_id)
                const usageStatus = getSubscriptionUsageStatus(
                  record,
                  meta?.plan,
                  t
                )

                return (
                  <div
                    key={subscription.id}
                    className='app-subtle-panel px-3 py-3'
                  >
                    <div className='flex items-start justify-between gap-3'>
                      <div className='min-w-0'>
                        <div className='text-foreground truncate text-sm font-semibold'>
                          {index + 1}.{' '}
                          {meta?.title ||
                            t('Plan #{{id}}', { id: subscription.id })}
                        </div>
                        <div className='text-muted-foreground mt-1 text-xs'>
                          {meta?.subtitle || t('Subscription')} · ~{' '}
                          {getWalletRemainingDays(subscription.end_time)}{' '}
                          {t('days')}
                        </div>
                        <div className='text-warning mt-1 text-xs'>
                          {usageStatus.note || usageStatus.label}
                        </div>
                        <div className='text-muted-foreground mt-1 text-xs'>
                          {t('Expires at: {{time}}', {
                            time: formatWalletDateTime(subscription.end_time),
                          })}
                        </div>
                      </div>
                      <div className='flex shrink-0 items-center gap-1'>
                        <Button
                          variant='outline'
                          size='icon'
                          className='h-8 w-8'
                          onClick={() =>
                            props.onMoveSubscription(subscription.id, -1)
                          }
                          disabled={index === 0 || props.saving}
                        >
                          <ArrowUp className='h-4 w-4' />
                        </Button>
                        <Button
                          variant='outline'
                          size='icon'
                          className='h-8 w-8'
                          onClick={() =>
                            props.onMoveSubscription(subscription.id, 1)
                          }
                          disabled={
                            index === props.orderedSubscriptions.length - 1 ||
                            props.saving
                          }
                        >
                          <ArrowDown className='h-4 w-4' />
                        </Button>
                      </div>
                    </div>
                  </div>
                )
              })}
            </div>
          )}
        </div>
      </div>
    </div>
  )
}
