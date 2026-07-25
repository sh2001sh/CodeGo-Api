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
import { useState } from 'react'
import { ArrowRight, ChevronDown } from 'lucide-react'
import { useTranslation } from 'react-i18next'
import { cn } from '@/lib/utils'
import { Button } from '@/components/ui/button'
import { Card, CardContent } from '@/components/ui/card'
import {
  formatDuration,
  formatSubscriptionPlanPrice,
  formatSubscriptionQuotaAmount,
} from '@/features/subscriptions/lib'
import type {
  PlanRecord,
  UserSubscriptionRecord,
} from '@/features/subscriptions/types'
import {
  translateDisabledReason,
  translatePlanAction,
  translatePlanSubtitle,
  translatePlanTitle,
} from './lib/display'

export function PackagePlanCard(props: {
  record: PlanRecord
  purchaseCount: number
  onPurchase: () => void
  currentSubscription?: UserSubscriptionRecord
  onFuel?: (
    subscription: UserSubscriptionRecord,
    title: string,
    config: { minimumQuota: number; quotaStep: number }
  ) => void
}) {
  const { t } = useTranslation()
  const [showDetails, setShowDetails] = useState(false)
  const plan = props.record.plan
  const title = translatePlanTitle(plan.title, t)
  const limit = Number(plan.max_purchase_per_user || 0)
  const limitReached = limit > 0 && props.purchaseCount >= limit
  const actionLabel = translatePlanAction(props.record.action, t)
  const effectiveAmount =
    props.record.action === 'disabled'
      ? plan.price_amount
      : (props.record.amount_due ?? plan.price_amount)
  const baseQuota = Number(plan.total_amount || 0)
  const blockedReason =
    translateDisabledReason(props.record.disabled_reason, t) ||
    t('A higher active plan with remaining quota prevents downgrading.')
  const isCurrentPlan =
    props.currentSubscription?.subscription.plan_id === plan.id
  const canFuel =
    isCurrentPlan &&
    props.currentSubscription?.subscription.status === 'active' &&
    plan.fuel_enabled === true &&
    Number(plan.fuel_min_quota || 0) > 0 &&
    Number(plan.fuel_quota_step || 0) > 0

  return (
    <Card className='border-border bg-card relative h-full overflow-hidden shadow-sm transition-all hover:shadow-md'>
      <CardContent className='flex h-full flex-col gap-3 p-4'>
        <div className='text-center'>
          <div className='text-muted-foreground text-xs font-medium'>
            {translatePlanSubtitle(plan, t)}
          </div>
          <h4 className='text-foreground mt-1 text-lg font-bold'>{title}</h4>
          {isCurrentPlan && (
            <span className='border-primary/25 bg-primary/10 text-primary mt-2 inline-flex rounded-md border px-2 py-1 text-xs font-semibold'>
              {t('Currently active')}
            </span>
          )}
        </div>

        <div className='text-center'>
          <div className='text-muted-foreground text-xs font-medium'>
            {t('Payment price')}
          </div>
          <div className='text-primary flex items-baseline justify-center gap-1 text-3xl font-bold'>
            {formatSubscriptionPlanPrice(effectiveAmount, plan.currency)}
          </div>
          {effectiveAmount !== plan.price_amount && (
            <div className='text-muted-foreground mt-1 text-xs line-through'>
              {formatSubscriptionPlanPrice(plan.price_amount, plan.currency)}
            </div>
          )}
        </div>

        <div className='space-y-2'>
          <div className='flex items-center justify-between text-sm'>
            <span className='text-muted-foreground'>
              {t('Base quota (USD)')}
            </span>
            <span className='text-foreground font-semibold'>
              {formatSubscriptionQuotaAmount(baseQuota)}
            </span>
          </div>
          <div className='flex items-center justify-between text-sm'>
            <span className='text-muted-foreground'>{t('Validity')}</span>
            <span className='text-foreground font-semibold'>
              {formatDuration(plan, t)}
            </span>
          </div>
        </div>

        {showDetails && (
          <div className='border-border space-y-2 rounded-lg border p-3'>
            <div className='text-muted-foreground text-xs leading-relaxed'>
              {t(
                'The quota is available after payment confirmation and can be used with the models allowed by this plan.'
              )}
            </div>
          </div>
        )}

        <button
          onClick={() => setShowDetails(!showDetails)}
          className='text-primary hover:text-primary/80 -mx-4 flex items-center justify-center gap-1 py-1 text-xs font-medium transition-colors'
        >
          {t(showDetails ? 'Hide details' : 'View details')}
          <ChevronDown
            className={cn(
              'h-3.5 w-3.5 transition-transform',
              showDetails && 'rotate-180'
            )}
          />
        </button>

        <div className='mt-auto space-y-2'>
          {canFuel && props.currentSubscription && props.onFuel && (
            <Button
              className='w-full'
              onClick={() =>
                props.onFuel?.(props.currentSubscription!, title, {
                  minimumQuota: Number(plan.fuel_min_quota || 0),
                  quotaStep: Number(plan.fuel_quota_step || 0),
                })
              }
            >
              {t('Add quota to current plan')}
            </Button>
          )}
          <Button
            className='w-full'
            disabled={limitReached || props.record.action === 'disabled'}
            onClick={props.onPurchase}
          >
            {limitReached ? t('Purchase limit reached') : actionLabel}
            {!limitReached && <ArrowRight className='ml-1 h-4 w-4' />}
          </Button>
          {limitReached && (
            <div className='text-muted-foreground text-center text-xs'>
              {t('Limit reached ({{current}}/{{limit}})', {
                current: props.purchaseCount,
                limit,
              })}
            </div>
          )}
          {props.record.action === 'disabled' && (
            <div className='text-muted-foreground text-center text-xs leading-relaxed'>
              {blockedReason}
            </div>
          )}
        </div>
      </CardContent>
    </Card>
  )
}
