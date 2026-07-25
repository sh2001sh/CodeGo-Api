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
import { useEffect, useRef, useState, type ReactNode } from 'react'
import {
  CalendarClock,
  CheckCircle2,
  CircleSlash,
  Crown,
  ExternalLink,
  Loader2,
  XCircle,
} from 'lucide-react'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'
import { cn } from '@/lib/utils'
import { Alert, AlertDescription } from '@/components/ui/alert'
import { Button } from '@/components/ui/button'
import {
  Dialog,
  DialogContent,
  DialogHeader,
  DialogTitle,
} from '@/components/ui/dialog'
import {
  cancelSubscriptionOrder,
  getSubscriptionOrderStatus,
  paySubscriptionCreem,
  paySubscriptionStripe,
} from '../../api'
import {
  formatDuration,
  formatResetPeriod,
  formatSubscriptionPlanPrice,
  formatSubscriptionQuotaAmount,
  getSubscriptionDisabledReasonText,
  getSubscriptionPlanActionLabel,
  getSubscriptionPlanDetailText,
  getSubscriptionPlanSubtitle,
  isMonthlyCardPlan,
  normalizeSubscriptionText,
} from '../../lib'
import type {
  PlanRecord,
  SubscriptionOrderStatus,
  SubscriptionPayResponse,
} from '../../types'
import { PackageModelScopeNotice } from '../package-model-scope-notice'

interface Props {
  open: boolean
  onOpenChange: (open: boolean) => void
  plan: PlanRecord | null
  enableStripe?: boolean
  enableCreem?: boolean
  purchaseLimit?: number
  purchaseCount?: number
}

type PaymentStage = 'idle' | 'pending' | 'success' | 'failed' | 'cancelled'

interface PaymentTracker {
  stage: PaymentStage
  orderId: string
  externalUrl: string
  amountDue: number
  methodLabel: string
  message: string
}

const EMPTY_PAYMENT_TRACKER: PaymentTracker = {
  stage: 'idle',
  orderId: '',
  externalUrl: '',
  amountDue: 0,
  methodLabel: '',
  message: '',
}

function SummaryItem(props: { label: string; value: ReactNode }) {
  return (
    <div className='app-subtle-panel px-3 py-3'>
      <div className='text-muted-foreground text-[11px] font-medium tracking-wide'>
        {props.label}
      </div>
      <div className='text-foreground mt-1 text-sm font-medium'>
        {props.value}
      </div>
    </div>
  )
}

function StatusItem(props: { label: string; value: ReactNode }) {
  return (
    <div className='app-subtle-panel px-3 py-2.5'>
      <div className='text-muted-foreground text-[11px] font-medium tracking-wide'>
        {props.label}
      </div>
      <div className='text-foreground mt-1 text-sm font-medium'>
        {props.value}
      </div>
    </div>
  )
}

export function SubscriptionPurchaseDialog(props: Props) {
  const { t } = useTranslation()
  const [paying, setPaying] = useState(false)
  const [paymentTracker, setPaymentTracker] = useState<PaymentTracker>(
    EMPTY_PAYMENT_TRACKER
  )
  const successEventSent = useRef(false)
  const planRecord = props.plan
  const plan = planRecord?.plan

  useEffect(() => {
    if (props.open) return
    setPaymentTracker(EMPTY_PAYMENT_TRACKER)
    successEventSent.current = false
  }, [props.open])

  useEffect(() => {
    if (
      !props.open ||
      paymentTracker.stage !== 'pending' ||
      !paymentTracker.orderId
    ) {
      return
    }

    let active = true
    const poll = async () => {
      try {
        const response = await getSubscriptionOrderStatus(
          paymentTracker.orderId
        )
        if (!active || !response.success || !response.data) return
        const order = response.data as SubscriptionOrderStatus
        if (order.status === 'success') {
          setPaymentTracker((current) => ({
            ...current,
            stage: 'success',
            message: t('Payment completed and the plan is active.'),
          }))
          if (!successEventSent.current) {
            successEventSent.current = true
            window.dispatchEvent(new Event('subscription:changed'))
          }
        } else if (order.status === 'expired') {
          setPaymentTracker((current) => ({
            ...current,
            stage: 'failed',
            message: t('This order expired before payment was completed.'),
          }))
        }
      } catch {
        // The next poll can still recover from a transient status request error.
      }
    }

    void poll()
    const timer = window.setInterval(() => void poll(), 2000)
    return () => {
      active = false
      window.clearInterval(timer)
    }
  }, [paymentTracker.orderId, paymentTracker.stage, props.open, t])

  if (!plan || !planRecord) return null

  const effectiveAmount = Number(
    planRecord.amount_due ?? plan.price_amount ?? 0
  )
  const baseAmount = Number(
    planRecord.base_amount_due ?? plan.price_amount ?? effectiveAmount
  )
  const totalAmount = Number(plan.total_amount || 0)
  const periodAmount = Number(plan.period_amount || 0)
  const isMonthlyPlan = isMonthlyCardPlan(plan)
  const actionLabel = getSubscriptionPlanActionLabel(planRecord.action, t)
  const detailText = getSubscriptionPlanDetailText(
    plan,
    totalAmount,
    periodAmount,
    t
  )
  const limitReached =
    (props.purchaseLimit || 0) > 0 &&
    (props.purchaseCount || 0) >= (props.purchaseLimit || 0)
  const blockedByRule = planRecord.action === 'disabled'
  const blockedMessage =
    getSubscriptionDisabledReasonText(planRecord.disabled_reason) ||
    t('A higher active plan prevents this purchase.')
  const disablePurchase =
    paying ||
    limitReached ||
    blockedByRule ||
    paymentTracker.stage === 'pending'
  const summaryItems = [
    { label: t('Purchase type'), value: actionLabel },
    {
      label: t('Validity'),
      value: (
        <span className='flex items-center gap-1.5'>
          <CalendarClock className='h-3.5 w-3.5' />
          {formatDuration(plan, t)}
        </span>
      ),
    },
    {
      label: isMonthlyPlan
        ? t('Monthly quota')
        : periodAmount > 0
          ? t('Period quota')
          : t('Total quota'),
      value: formatSubscriptionQuotaAmount(
        !isMonthlyPlan && periodAmount > 0 ? periodAmount : totalAmount
      ),
    },
    ...(!isMonthlyPlan && periodAmount > 0
      ? [
          {
            label: t('Total quota'),
            value:
              totalAmount > 0
                ? formatSubscriptionQuotaAmount(totalAmount)
                : t('Unlimited'),
          },
        ]
      : []),
    ...(!isMonthlyPlan
      ? [
          {
            label: t('Quota reset'),
            value:
              formatResetPeriod(plan, t) === t('No Reset')
                ? t('No reset')
                : formatResetPeriod(plan, t),
          },
        ]
      : []),
    {
      label: t('Payment price'),
      value: formatSubscriptionPlanPrice(effectiveAmount, plan.currency),
    },
  ]

  const startPendingPayment = (
    response: SubscriptionPayResponse,
    methodLabel: string,
    externalUrl: string
  ) => {
    setPaymentTracker({
      stage: 'pending',
      orderId: String(response.data?.order_id || ''),
      externalUrl,
      amountDue: Number(response.data?.amount_due ?? effectiveAmount),
      methodLabel,
      message: t(
        'Complete payment in the new window. This dialog will refresh automatically.'
      ),
    })
    toast.success(t('Payment request created.'))
  }

  const handlePayment = async (method: 'Stripe' | 'Creem') => {
    setPaying(true)
    try {
      const response =
        method === 'Stripe'
          ? await paySubscriptionStripe({ plan_id: plan.id })
          : await paySubscriptionCreem({ plan_id: plan.id })
      const paymentUrl =
        response.data?.pay_link || response.data?.checkout_url || ''
      if (
        response.message === 'success' &&
        paymentUrl &&
        response.data?.order_id
      ) {
        window.open(paymentUrl, '_blank')
        startPendingPayment(response, method, paymentUrl)
        return
      }
      toast.error(response.message || t('Payment request failed.'))
    } catch {
      toast.error(t('Payment request failed.'))
    } finally {
      setPaying(false)
    }
  }

  const cancelPendingPayment = () => {
    if (paymentTracker.orderId) {
      void cancelSubscriptionOrder(paymentTracker.orderId)
    }
    setPaymentTracker((current) => ({
      ...current,
      stage: 'cancelled',
      message: t('The pending order was cancelled.'),
    }))
  }

  const handleOpenChange = (open: boolean) => {
    if (!open && paymentTracker.stage === 'pending') cancelPendingPayment()
    props.onOpenChange(open)
  }

  const renderPaymentStatus = () => {
    if (paymentTracker.stage === 'idle') return null
    const statusConfig = {
      pending: {
        icon: <Loader2 className='h-5 w-5 animate-spin' />,
        title: t('Waiting for payment'),
        tone: 'border-warning/20 bg-warning/5',
      },
      success: {
        icon: <CheckCircle2 className='text-success h-5 w-5' />,
        title: t('Payment completed'),
        tone: 'border-success/20 bg-success/5',
      },
      failed: {
        icon: <XCircle className='text-destructive h-5 w-5' />,
        title: t('Payment failed'),
        tone: 'border-destructive/20 bg-destructive/5',
      },
      cancelled: {
        icon: <CircleSlash className='text-muted-foreground h-5 w-5' />,
        title: t('Payment cancelled'),
        tone: 'border-border/70 bg-muted/40',
      },
      idle: { icon: null, title: '', tone: '' },
    }[paymentTracker.stage]

    return (
      <div
        className={cn('space-y-4 rounded-2xl border p-4', statusConfig.tone)}
      >
        <div className='flex items-start gap-3'>
          <div className='bg-background flex h-10 w-10 shrink-0 items-center justify-center rounded-full border'>
            {statusConfig.icon}
          </div>
          <div className='min-w-0'>
            <div className='text-foreground text-sm font-semibold'>
              {statusConfig.title}
            </div>
            <p className='text-muted-foreground mt-1 text-sm leading-6'>
              {paymentTracker.message}
            </p>
          </div>
        </div>
        <div className='grid gap-2 sm:grid-cols-2'>
          <StatusItem
            label={t('Payment method')}
            value={paymentTracker.methodLabel}
          />
          <StatusItem
            label={t('Amount due')}
            value={formatSubscriptionPlanPrice(
              paymentTracker.amountDue,
              plan.currency
            )}
          />
          <StatusItem
            label={t('Order')}
            value={paymentTracker.orderId || '-'}
          />
        </div>
        <div className='flex flex-wrap gap-2'>
          {paymentTracker.externalUrl && paymentTracker.stage === 'pending' ? (
            <Button
              variant='outline'
              onClick={() => window.open(paymentTracker.externalUrl, '_blank')}
            >
              <ExternalLink className='mr-1 h-4 w-4' />
              {t('Open payment page')}
            </Button>
          ) : null}
          {paymentTracker.stage === 'pending' ? (
            <Button variant='ghost' onClick={cancelPendingPayment}>
              {t('Cancel waiting')}
            </Button>
          ) : (
            <Button onClick={() => handleOpenChange(false)}>
              {t('Close')}
            </Button>
          )}
        </div>
      </div>
    )
  }

  return (
    <Dialog open={props.open} onOpenChange={handleOpenChange}>
      <DialogContent className='flex max-h-[calc(100vh-1rem)] w-[calc(100vw-1rem)] max-w-[calc(100vw-1rem)] flex-col gap-0 overflow-hidden p-0 sm:max-w-2xl'>
        <DialogHeader className='border-border/70 border-b px-4 py-4 sm:px-5'>
          <DialogTitle className='flex items-center gap-2 text-lg'>
            <Crown className='h-5 w-5' />
            {actionLabel}
          </DialogTitle>
        </DialogHeader>
        <div className='flex-1 overflow-y-auto px-4 pt-4 pb-4 sm:px-5 sm:pb-5'>
          <div className='space-y-4'>
            <div className='app-page-shell overflow-hidden'>
              <div className='border-border/70 border-b px-4 py-4 sm:px-5'>
                <div className='flex flex-col gap-3 sm:flex-row sm:items-end sm:justify-between'>
                  <div className='min-w-0'>
                    <p className='text-primary text-[11px] font-semibold tracking-[0.22em] uppercase'>
                      {getSubscriptionPlanSubtitle(plan)}
                    </p>
                    <h3 className='text-foreground mt-1 truncate text-xl font-semibold tracking-tight sm:text-2xl'>
                      {normalizeSubscriptionText(plan.title) || t('Plan name')}
                    </h3>
                    <p className='text-muted-foreground mt-2 text-sm leading-6'>
                      {detailText}
                    </p>
                  </div>
                  <div className='shrink-0 text-left sm:text-right'>
                    <div className='text-muted-foreground text-xs'>
                      {t('Payment price')}
                    </div>
                    <div className='text-primary mt-1 text-2xl font-bold tabular-nums'>
                      {formatSubscriptionPlanPrice(
                        effectiveAmount,
                        plan.currency
                      )}
                    </div>
                    {baseAmount !== effectiveAmount ? (
                      <div className='text-muted-foreground text-xs line-through'>
                        {formatSubscriptionPlanPrice(baseAmount, plan.currency)}
                      </div>
                    ) : null}
                  </div>
                </div>
              </div>
              <div className='grid gap-2 p-4 sm:grid-cols-2'>
                {summaryItems.map((item) => (
                  <SummaryItem
                    key={item.label}
                    label={item.label}
                    value={item.value}
                  />
                ))}
              </div>
            </div>

            <PackageModelScopeNotice />

            {limitReached ? (
              <Alert variant='destructive'>
                <AlertDescription>
                  {t('This plan has reached its purchase limit.')}
                </AlertDescription>
              </Alert>
            ) : null}
            {blockedByRule ? (
              <Alert variant='destructive'>
                <AlertDescription>{blockedMessage}</AlertDescription>
              </Alert>
            ) : null}

            {renderPaymentStatus()}

            {paymentTracker.stage === 'idle' ? (
              <div className='grid gap-3 sm:grid-cols-2'>
                {props.enableStripe && plan.stripe_price_id ? (
                  <Button
                    disabled={disablePurchase}
                    onClick={() => void handlePayment('Stripe')}
                  >
                    {paying ? <Loader2 className='animate-spin' /> : null}
                    {t('Pay with Stripe')}
                  </Button>
                ) : null}
                {props.enableCreem && plan.creem_product_id ? (
                  <Button
                    variant='outline'
                    disabled={disablePurchase}
                    onClick={() => void handlePayment('Creem')}
                  >
                    {paying ? <Loader2 className='animate-spin' /> : null}
                    {t('Pay with Creem')}
                  </Button>
                ) : null}
              </div>
            ) : null}
            {!props.enableStripe && !props.enableCreem ? (
              <Alert>
                <AlertDescription>
                  {t('No online payment provider is enabled.')}
                </AlertDescription>
              </Alert>
            ) : null}
          </div>
        </div>
      </DialogContent>
    </Dialog>
  )
}
