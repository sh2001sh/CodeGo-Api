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
import { CreditCard, SlidersHorizontal } from 'lucide-react'
import { useTranslation } from 'react-i18next'
import { Tabs, TabsList, TabsTrigger } from '@/components/ui/tabs'
import { BillingHistoryDialog } from './components/dialogs/billing-history-dialog'
import { CreemConfirmDialog } from './components/dialogs/creem-confirm-dialog'
import { PaymentConfirmDialog } from './components/dialogs/payment-confirm-dialog'
import { RechargeFormCard } from './components/recharge-form-card'
import { WalletAccountOverview } from './components/wallet-account-overview'
import { WalletPagePanels } from './components/wallet-page-panels'
import { WalletWorkspaceShell } from './components/wallet-workspace-shell'
import { useWalletWorkspace } from './hooks/use-wallet-workspace'

interface WalletProps {
  initialShowHistory?: boolean
}

export function Wallet(props: WalletProps) {
  const { t } = useTranslation()
  const workspace = useWalletWorkspace()
  const [activeSection, setActiveSection] = useState<'funding' | 'billing'>(
    'funding'
  )

  useEffect(() => {
    if (!props.initialShowHistory) return
    workspace.setBillingDialogOpen(true)
    if (typeof window !== 'undefined') {
      window.history.replaceState({}, '', window.location.pathname)
    }
  }, [props.initialShowHistory, workspace.setBillingDialogOpen])

  return (
    <>
      <WalletWorkspaceShell
        title={t('Wallet')}
        description={t(
          'Manage one shared API balance, top-ups, redemption codes, and billing records.'
        )}
        framedMain={false}
        main={
          <div className='flex min-w-0 flex-col gap-4'>
            <WalletAccountOverview
              user={workspace.user}
              activeSubscriptionCount={
                workspace.subscriptionData?.subscriptions?.length ?? 0
              }
              onSelectFunding={() => setActiveSection('funding')}
              onOpenBillingHistory={() => workspace.setBillingDialogOpen(true)}
            />

            <Tabs
              value={activeSection}
              onValueChange={(value) =>
                setActiveSection(value as 'funding' | 'billing')
              }
            >
              <TabsList className='grid w-full grid-cols-2'>
                <TabsTrigger value='funding'>
                  <CreditCard className='size-4' />
                  {t('Balance and redemption')}
                </TabsTrigger>
                <TabsTrigger value='billing'>
                  <SlidersHorizontal className='size-4' />
                  {t('Billing settings')}
                </TabsTrigger>
              </TabsList>
            </Tabs>

            {activeSection === 'funding' ? (
              <RechargeFormCard
                topupInfo={workspace.topupInfo}
                presetAmounts={workspace.presetAmounts}
                selectedPreset={workspace.selectedPreset}
                onSelectPreset={workspace.handleSelectPreset}
                topupAmount={workspace.topupAmount}
                onTopupAmountChange={workspace.handleTopupAmountChange}
                paymentAmount={workspace.paymentAmount}
                calculating={workspace.calculating}
                onPaymentMethodSelect={workspace.handlePaymentMethodSelect}
                paymentLoading={workspace.paymentLoading}
                redemptionCode={workspace.redemptionCode}
                onRedemptionCodeChange={workspace.setRedemptionCode}
                onRedeem={workspace.handleRedeem}
                redeeming={workspace.redeeming}
                loading={workspace.topupLoading}
                onOpenBilling={() => workspace.setBillingDialogOpen(true)}
                showRedemptionSection={false}
                creemProducts={workspace.topupInfo?.creem_products}
                enableCreemTopup={workspace.topupInfo?.enable_creem_topup}
                onCreemProductSelect={workspace.handleCreemProductSelect}
                compact
              />
            ) : null}

            <WalletPagePanels
              plans={workspace.publicPlans}
              plansLoading={workspace.publicPlansLoading}
              loading={workspace.userLoading}
              topupLink={workspace.topupInfo?.topup_link}
              redemptionCode={workspace.redemptionCode}
              onRedemptionCodeChange={workspace.setRedemptionCode}
              onRedeem={workspace.handleRedeem}
              redeeming={workspace.redeeming}
              subscriptionData={workspace.subscriptionData}
              subscriptionLoading={workspace.subscriptionLoading}
              onSubscriptionRefresh={workspace.fetchSubscriptionData}
              section={activeSection}
            />
          </div>
        }
      />

      <PaymentConfirmDialog
        open={workspace.confirmDialogOpen}
        onOpenChange={workspace.setConfirmDialogOpen}
        onConfirm={workspace.handlePaymentConfirm}
        topupAmount={workspace.topupAmount}
        paymentAmount={workspace.paymentAmount}
        paymentMethod={workspace.selectedPaymentMethod}
        calculating={workspace.calculating}
        processing={workspace.processing}
        discountRate={workspace.getDiscountRate()}
        usdExchangeRate={workspace.effectiveUsdExchangeRate}
      />

      <BillingHistoryDialog
        open={workspace.billingDialogOpen}
        onOpenChange={workspace.setBillingDialogOpen}
      />

      <CreemConfirmDialog
        open={workspace.creemDialogOpen}
        onOpenChange={workspace.setCreemDialogOpen}
        onConfirm={workspace.handleCreemConfirm}
        product={workspace.selectedCreemProduct}
        processing={workspace.creemProcessing}
      />
    </>
  )
}
