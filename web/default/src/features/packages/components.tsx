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
import { useMemo } from 'react'
import { Link } from '@tanstack/react-router'
import { Crown, Fuel } from 'lucide-react'
import { useTranslation } from 'react-i18next'
import { Button } from '@/components/ui/button'
import { Progress } from '@/components/ui/progress'
import { Skeleton } from '@/components/ui/skeleton'
import {
  getSubscriptionDisabledReasonText,
  subscriptionQuotaUnitsToUSD,
} from '@/features/subscriptions/lib'
import type {
  PlanRecord,
  UserSubscriptionRecord,
} from '@/features/subscriptions/types'
import { translatePlanTitle } from './lib/display'
import { PackagePlanCard } from './package-plan-card'

type FuelConfig = { minimumQuota: number; quotaStep: number }

export function PlanZone(props: {
  title: string
  description: string
  plans: PlanRecord[]
  loading: boolean
  purchaseCountMap: Map<number, number>
  onPurchase: (record: PlanRecord) => void
  currentSubscription?: UserSubscriptionRecord
  onFuel?: (
    subscription: UserSubscriptionRecord,
    title: string,
    config: FuelConfig
  ) => void
}) {
  const { t } = useTranslation()

  return (
    <section className='space-y-3'>
      <div>
        <h3 className='text-foreground text-base font-semibold'>
          {props.title}
        </h3>
        <p className='text-muted-foreground mt-1 text-sm leading-6'>
          {props.description}
        </p>
      </div>
      {props.loading ? (
        <div className='grid gap-4 sm:grid-cols-2 lg:grid-cols-3 xl:grid-cols-4'>
          {Array.from({ length: 4 }).map((_, index) => (
            <Skeleton key={index} className='h-[420px] rounded-xl' />
          ))}
        </div>
      ) : props.plans.length > 0 ? (
        <div className='grid gap-4 sm:grid-cols-2 lg:grid-cols-3 xl:grid-cols-4'>
          {props.plans.map((record) => (
            <PackagePlanCard
              key={record.plan.id}
              record={record}
              purchaseCount={props.purchaseCountMap.get(record.plan.id) || 0}
              onPurchase={() => props.onPurchase(record)}
              currentSubscription={props.currentSubscription}
              onFuel={props.onFuel}
            />
          ))}
        </div>
      ) : (
        <p className='text-muted-foreground border-border border-t pt-3 text-sm'>
          {t('No plans are currently available in this section.')}
        </p>
      )}
    </section>
  )
}

export function CurrentPackagePanel(props: {
  subscriptions: UserSubscriptionRecord[]
  plans: PlanRecord[]
  loading: boolean
  onFuel: (
    subscription: UserSubscriptionRecord,
    title: string,
    config: FuelConfig
  ) => void
}) {
  const { t } = useTranslation()
  const planMap = useMemo(() => {
    const map = new Map<number, PlanRecord>()
    for (const item of props.plans) map.set(item.plan.id, item)
    return map
  }, [props.plans])
  const current = props.subscriptions[0]
  const currentPlanRecord = current
    ? planMap.get(current.subscription.plan_id)
    : undefined
  const currentPlan = currentPlanRecord?.plan
  const currentTitle =
    translatePlanTitle(currentPlan?.title, t) ||
    (current ? t('Plan #{{id}}', { id: current.subscription.plan_id }) : '')
  const canFuel =
    Boolean(current) &&
    current?.subscription.status === 'active' &&
    currentPlan?.fuel_enabled === true &&
    (currentPlan?.fuel_min_quota || 0) > 0 &&
    (currentPlan?.fuel_quota_step || 0) > 0
  const renewalBlocked = currentPlanRecord?.action === 'disabled'
  const renewalBlockedReason = getSubscriptionDisabledReasonText(
    currentPlanRecord?.disabled_reason
  )
  if (!props.loading && !current) {
    return null
  }

  return (
    <section className='border-border bg-card flex flex-wrap items-center gap-x-4 gap-y-3 rounded-lg border px-4 py-3'>
      {props.loading ? (
        <Skeleton className='h-8 w-full sm:w-96' />
      ) : current ? (
        <>
          <div className='flex min-w-0 items-center gap-2'>
            <Crown className='text-primary size-4 shrink-0' />
            <span className='text-foreground truncate text-sm font-semibold'>
              {t('Current')}: {currentTitle}
            </span>
          </div>
          <div className='text-muted-foreground text-sm tabular-nums'>
            {t('Remaining')} $
            {Math.max(
              0,
              subscriptionQuotaUnitsToUSD(
                current.subscription.amount_total -
                  current.subscription.amount_used
              )
            ).toFixed(2)}
            /$
            {subscriptionQuotaUnitsToUSD(
              current.subscription.amount_total
            ).toFixed(2)}
          </div>
          <Progress
            className='order-last w-full sm:order-none sm:min-w-32 sm:flex-1'
            value={
              current.subscription.amount_total > 0
                ? Math.round(
                    (current.subscription.amount_used /
                      current.subscription.amount_total) *
                      100
                  )
                : 0
            }
          />
          <div className='ml-auto flex gap-2'>
            {canFuel ? (
              <Button
                size='sm'
                onClick={() =>
                  props.onFuel(current, currentTitle, {
                    minimumQuota: currentPlan?.fuel_min_quota || 0,
                    quotaStep: currentPlan?.fuel_quota_step || 0,
                  })
                }
              >
                <Fuel className='mr-1 size-4' />
                {t('Add quota')}
              </Button>
            ) : null}
            {renewalBlocked ? (
              <div className='flex max-w-64 flex-col items-end gap-1'>
                <Button size='sm' variant='outline' disabled>
                  {t('Renewal unavailable')}
                </Button>
                <span className='text-muted-foreground text-right text-xs leading-4'>
                  {renewalBlockedReason}
                </span>
              </div>
            ) : (
              <Button
                size='sm'
                variant='outline'
                render={<Link to='/packages' />}
              >
                {t('Renew')}
              </Button>
            )}
          </div>
        </>
      ) : null}
    </section>
  )
}
