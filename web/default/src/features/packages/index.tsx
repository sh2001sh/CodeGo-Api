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
import { useMemo, useState } from 'react'
import { Crown, RefreshCw } from 'lucide-react'
import { useTranslation } from 'react-i18next'
import { cn } from '@/lib/utils'
import { Button } from '@/components/ui/button'
import { TitledCard } from '@/components/ui/titled-card'
import {
  CardStaggerContainer,
  CardStaggerItem,
} from '@/components/page-transition'
import { SubscriptionFuelDialog } from '@/features/subscriptions/components/dialogs/subscription-fuel-dialog'
import { SubscriptionPurchaseDialog } from '@/features/subscriptions/components/dialogs/subscription-purchase-dialog'
import { PackageModelScopeNotice } from '@/features/subscriptions/components/package-model-scope-notice'
import type {
  PlanRecord,
  UserSubscriptionRecord,
} from '@/features/subscriptions/types'
import { WalletStatsCard } from '@/features/wallet/components/wallet-stats-card'
import { WalletWorkspaceShell } from '@/features/wallet/components/wallet-workspace-shell'
import { useWalletWorkspace } from '@/features/wallet/hooks/use-wallet-workspace'
import { CurrentPackagePanel, PlanZone } from './components'

type ZoneId = 'starter' | 'monthly' | 'shortterm'

function planRank(record: PlanRecord) {
  return -Number(record.plan?.sort_order || 0)
}

function getPlanZone(record: PlanRecord): ZoneId {
  const planType = record.plan?.plan_type
  if (planType === 'starter') return 'starter'
  if (planType === 'monthly') return 'monthly'
  return 'shortterm'
}

function useGroupedPlans(plans: PlanRecord[]) {
  return useMemo(() => {
    const grouped: Record<ZoneId, PlanRecord[]> = {
      starter: [],
      monthly: [],
      shortterm: [],
    }
    for (const record of plans) {
      if (!record.plan) continue
      grouped[getPlanZone(record)].push(record)
    }
    for (const value of Object.values(grouped)) {
      value.sort((a, b) => planRank(a) - planRank(b))
    }
    return grouped
  }, [plans])
}

export function PackagesPage() {
  const { t } = useTranslation()
  const workspace = useWalletWorkspace()
  const [selectedPlan, setSelectedPlan] = useState<PlanRecord | null>(null)
  const [purchaseOpen, setPurchaseOpen] = useState(false)
  const [refreshing, setRefreshing] = useState(false)
  const [fuelSubscription, setFuelSubscription] =
    useState<UserSubscriptionRecord | null>(null)
  const [fuelTitle, setFuelTitle] = useState('')
  const [fuelConfig, setFuelConfig] = useState({
    minimumQuota: 500_000,
    quotaStep: 500_000,
  })
  const groupedPlans = useGroupedPlans(workspace.publicPlans)
  const topupInfo = workspace.topupInfo

  const purchaseCountMap = useMemo(() => {
    const map = new Map<number, number>()
    for (const item of workspace.subscriptionData?.all_subscriptions ?? []) {
      const planId = item.subscription?.plan_id
      if (planId) map.set(planId, (map.get(planId) || 0) + 1)
    }
    return map
  }, [workspace.subscriptionData?.all_subscriptions])
  const currentSubscription = workspace.subscriptionData?.subscriptions[0]
  const shouldPrioritizeMonthlyPlans = Boolean(currentSubscription)
  const primaryPlanZones: Array<{
    id: 'starter' | 'monthly'
    title: string
    description: string
  }> = shouldPrioritizeMonthlyPlans
    ? [
        {
          id: 'monthly',
          title: t('Monthly plans'),
          description: t(
            'Suitable for ongoing development and team usage, with clear quota and validity.'
          ),
        },
        {
          id: 'starter',
          title: t('Starter plans'),
          description: t(
            'Short-term access with a defined quota and validity.'
          ),
        },
      ]
    : [
        {
          id: 'starter',
          title: t('Starter plans'),
          description: t(
            'Short-term access with a defined quota and validity.'
          ),
        },
        {
          id: 'monthly',
          title: t('Monthly plans'),
          description: t(
            'Suitable for ongoing development and team usage, with clear quota and validity.'
          ),
        },
      ]

  const openFuel = (
    subscription: UserSubscriptionRecord,
    title: string,
    config: { minimumQuota: number; quotaStep: number }
  ) => {
    setFuelSubscription(subscription)
    setFuelTitle(title)
    setFuelConfig(config)
  }

  const handleRefresh = async () => {
    setRefreshing(true)
    try {
      await Promise.all([
        workspace.fetchPublicPlans(),
        workspace.fetchSubscriptionData(),
      ])
    } finally {
      setRefreshing(false)
    }
  }

  const openPurchase = (record: PlanRecord) => {
    setSelectedPlan(record)
    setPurchaseOpen(true)
  }

  return (
    <>
      <WalletWorkspaceShell
        title={t('Plans')}
        description={t(
          'Choose a quota plan for predictable API usage across the models and routes you manage.'
        )}
        framedMain={false}
        main={
          <CardStaggerContainer className='space-y-4'>
            <CardStaggerItem>
              <CurrentPackagePanel
                subscriptions={workspace.subscriptionData?.subscriptions || []}
                plans={workspace.publicPlans}
                loading={workspace.subscriptionLoading}
                onFuel={openFuel}
              />
            </CardStaggerItem>

            <CardStaggerItem>
              <TitledCard
                title={t('Plan purchase')}
                description={t(
                  'Compare price, quota, model scope, and validity before subscribing.'
                )}
                icon={<Crown className='h-4 w-4' />}
                action={
                  <Button
                    variant='outline'
                    size='sm'
                    onClick={() => void handleRefresh()}
                    disabled={refreshing}
                  >
                    <RefreshCw
                      className={cn(
                        'mr-1 h-4 w-4',
                        refreshing && 'animate-spin'
                      )}
                    />
                    {t('Refresh')}
                  </Button>
                }
                contentClassName='space-y-5'
              >
                <PackageModelScopeNotice />
                {primaryPlanZones.map((zone) => {
                  if (
                    zone.id === 'monthly' &&
                    groupedPlans.monthly.length === 0
                  ) {
                    return null
                  }
                  return (
                    <PlanZone
                      key={zone.id}
                      title={zone.title}
                      description={zone.description}
                      plans={groupedPlans[zone.id]}
                      loading={workspace.publicPlansLoading}
                      onPurchase={openPurchase}
                      purchaseCountMap={purchaseCountMap}
                      currentSubscription={currentSubscription}
                      onFuel={openFuel}
                    />
                  )
                })}
                {groupedPlans.shortterm.length > 0 && (
                  <PlanZone
                    title={t('Short-term quota packs')}
                    description={t(
                      'Useful for short-term projects or temporary capacity increases.'
                    )}
                    plans={groupedPlans.shortterm}
                    loading={workspace.publicPlansLoading}
                    onPurchase={openPurchase}
                    purchaseCountMap={purchaseCountMap}
                    currentSubscription={currentSubscription}
                    onFuel={openFuel}
                  />
                )}
              </TitledCard>
            </CardStaggerItem>
          </CardStaggerContainer>
        }
        sidebar={
          <WalletStatsCard
            user={workspace.user}
            plans={workspace.publicPlans}
            loading={workspace.userLoading}
            subscriptionData={workspace.subscriptionData}
          />
        }
      />

      <SubscriptionPurchaseDialog
        open={purchaseOpen}
        onOpenChange={(open) => {
          setPurchaseOpen(open)
          if (!open) {
            void workspace.fetchPublicPlans()
            void workspace.fetchSubscriptionData()
          }
        }}
        plan={selectedPlan}
        enableStripe={!!topupInfo?.enable_stripe_topup}
        enableCreem={!!topupInfo?.enable_creem_topup}
        purchaseLimit={selectedPlan?.plan?.max_purchase_per_user || undefined}
        purchaseCount={
          selectedPlan?.plan?.id
            ? purchaseCountMap.get(selectedPlan.plan.id)
            : undefined
        }
      />
      {fuelSubscription ? (
        <SubscriptionFuelDialog
          open
          onOpenChange={(open) => {
            if (!open) setFuelSubscription(null)
          }}
          subscription={fuelSubscription.subscription}
          title={fuelTitle}
          minimumQuota={fuelConfig.minimumQuota}
          quotaStep={fuelConfig.quotaStep}
          paymentMethods={[]}
          enableStripe={!!topupInfo?.enable_stripe_topup}
          onCompleted={workspace.fetchSubscriptionData}
        />
      ) : null}
    </>
  )
}
