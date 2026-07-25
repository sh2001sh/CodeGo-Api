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
import { useEffect, useMemo, useState } from 'react'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'
import { Skeleton } from '@/components/ui/skeleton'
import { updateBillingPreference } from '@/features/subscriptions/api'
import {
  getBillingPreferenceFromFundingSourceOrder,
  normalizeFundingSourceOrder,
} from '@/features/subscriptions/billing'
import { getSubscriptionPlanSubtitle } from '@/features/subscriptions/lib'
import type {
  FundingSource,
  PlanRecord,
  SelfSubscriptionData,
} from '@/features/subscriptions/types'
import { RedemptionCodePanel } from './redemption-code-panel'
import { WalletBillingOrderPanel } from './wallet-billing-order-panel'
import {
  getOrderedSubscriptions,
  type WalletPlanMeta,
} from './wallet-panel-utils'

const ALL_FUNDING_SOURCES: FundingSource[] = ['subscription', 'wallet']

interface WalletPagePanelsProps {
  plans: PlanRecord[]
  plansLoading?: boolean
  loading?: boolean
  topupLink?: string
  redemptionCode: string
  onRedemptionCodeChange: (code: string) => void
  onRedeem: () => void
  redeeming: boolean
  subscriptionData?: SelfSubscriptionData | null
  subscriptionLoading?: boolean
  onSubscriptionRefresh?: () => Promise<void>
  section: 'funding' | 'billing'
}

export function WalletPagePanels(props: WalletPagePanelsProps) {
  const { t } = useTranslation()
  const [draftFundingSourceOrder, setDraftFundingSourceOrder] = useState<
    FundingSource[]
  >(['subscription', 'wallet'])
  const [draftOrderIds, setDraftOrderIds] = useState<number[]>([])
  const [saving, setSaving] = useState(false)

  const activeSubscriptions = props.subscriptionData?.subscriptions ?? []
  const hasActiveSubscriptions = activeSubscriptions.length > 0

  useEffect(() => {
    if (!props.subscriptionData) return
    setDraftFundingSourceOrder(
      normalizeFundingSourceOrder(
        props.subscriptionData.funding_source_order,
        props.subscriptionData.billing_preference
      )
    )
    setDraftOrderIds(
      props.subscriptionData.subscription_order_ids?.length
        ? props.subscriptionData.subscription_order_ids
        : activeSubscriptions.map((item) => item.subscription.id)
    )
  }, [activeSubscriptions, props.subscriptionData])

  const planMetaMap = useMemo(() => {
    const map = new Map<number, WalletPlanMeta>()
    for (const item of props.plans) {
      if (!item?.plan?.id) continue
      map.set(item.plan.id, {
        title: item.plan.title || '',
        subtitle: getSubscriptionPlanSubtitle(item.plan),
        plan: item.plan,
      })
    }
    return map
  }, [props.plans])

  const orderedSubscriptions = useMemo(
    () => getOrderedSubscriptions(activeSubscriptions, draftOrderIds),
    [activeSubscriptions, draftOrderIds]
  )

  const disabledFundingSources = ALL_FUNDING_SOURCES.filter(
    (source) => !draftFundingSourceOrder.includes(source)
  )

  const handleSave = async () => {
    setSaving(true)
    try {
      const fundingSourceOrder = normalizeFundingSourceOrder(
        draftFundingSourceOrder,
        getBillingPreferenceFromFundingSourceOrder(draftFundingSourceOrder)
      )
      const response = await updateBillingPreference({
        billingPreference:
          getBillingPreferenceFromFundingSourceOrder(fundingSourceOrder),
        fundingSourceOrder,
        subscriptionOrderIds: hasActiveSubscriptions ? draftOrderIds : [],
      })
      if (!response.success) {
        toast.error(response.message || t('Failed to save billing priority.'))
        return
      }
      toast.success(t('Billing priority updated.'))
      await props.onSubscriptionRefresh?.()
    } catch {
      toast.error(t('Failed to save billing priority.'))
    } finally {
      setSaving(false)
    }
  }

  if (props.loading && props.section === 'billing') {
    return (
      <div className='grid gap-4 lg:grid-cols-2'>
        {Array.from({ length: 2 }).map((_, index) => (
          <div key={index} className='app-page-shell p-4'>
            <Skeleton className='h-5 w-28' />
            <Skeleton className='mt-3 h-10 w-full' />
            <Skeleton className='mt-3 h-10 w-full' />
          </div>
        ))}
      </div>
    )
  }

  if (props.section === 'funding') {
    return (
      <RedemptionCodePanel
        title={t('Redemption code')}
        description={t(
          'Redeem a code to add balance or activate a subscription.'
        )}
        topupLink={props.topupLink}
        redemptionCode={props.redemptionCode}
        onRedemptionCodeChange={props.onRedemptionCodeChange}
        onRedeem={props.onRedeem}
        redeeming={props.redeeming}
      />
    )
  }

  return (
    <WalletBillingOrderPanel
      draftFundingSourceOrder={draftFundingSourceOrder}
      disabledFundingSources={disabledFundingSources}
      subscriptionModeEnabled={draftFundingSourceOrder.includes('subscription')}
      hasActiveSubscriptions={hasActiveSubscriptions}
      orderedSubscriptions={orderedSubscriptions}
      planMetaMap={planMetaMap}
      saving={saving}
      isLoading={Boolean(
        props.loading || props.subscriptionLoading || props.plansLoading
      )}
      subscriptionLoading={props.subscriptionLoading ?? false}
      onRefresh={() => void props.onSubscriptionRefresh?.()}
      onSave={() => void handleSave()}
      onResetFundingSourceOrder={() =>
        setDraftFundingSourceOrder(['subscription', 'wallet'])
      }
      onResetSubscriptionOrder={() =>
        setDraftOrderIds(
          activeSubscriptions.map((item) => item.subscription.id)
        )
      }
      onToggleFundingSource={(source) => {
        setDraftFundingSourceOrder((current) => {
          if (current.includes(source)) {
            if (current.length === 1) return current
            return current.filter((item) => item !== source)
          }
          return [...current, source]
        })
      }}
      onMoveFundingSource={(source, direction) => {
        setDraftFundingSourceOrder((current) => {
          const index = current.indexOf(source)
          const target = index + direction
          if (index < 0 || target < 0 || target >= current.length)
            return current
          const next = [...current]
          ;[next[index], next[target]] = [next[target], next[index]]
          return next
        })
      }}
      onMoveSubscription={(id, direction) => {
        setDraftOrderIds((current) => {
          const index = current.indexOf(id)
          const target = index + direction
          if (index < 0 || target < 0 || target >= current.length)
            return current
          const next = [...current]
          ;[next[index], next[target]] = [next[target], next[index]]
          return next
        })
      }}
    />
  )
}
