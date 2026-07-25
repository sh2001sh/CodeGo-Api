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
import { useMutation } from '@tanstack/react-query'
import { AlertCircle, Loader2 } from 'lucide-react'
import { toast } from 'sonner'
import { Alert, AlertDescription } from '@/components/ui/alert'
import { Button } from '@/components/ui/button'
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from '@/components/ui/dialog'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select'
import {
  getSubscriptionOrderStatus,
  purchaseSubscriptionFuel,
  quoteSubscriptionFuel,
} from '../../api'
import type { SubscriptionFuelQuote, UserSubscription } from '../../types'
import {
  SubscriptionFuelPaymentResult,
  type FuelPaymentState,
} from './subscription-fuel-payment-result'

type PaymentMethod = { type: string; name?: string }

interface SubscriptionFuelDialogProps {
  open: boolean
  onOpenChange: (open: boolean) => void
  subscription: UserSubscription | null
  title: string
  minimumQuota: number
  quotaStep: number
  paymentMethods: PaymentMethod[]
  enableStripe?: boolean
  onCompleted?: () => Promise<void>
}

function formatDate(value: number): string {
  return new Date(value * 1000).toLocaleString()
}

function submitExternalPaymentForm(
  url: string,
  params: Record<string, unknown>,
  isSafari: boolean
) {
  const form = document.createElement('form')
  form.action = url
  form.method = 'POST'
  if (!isSafari) form.target = '_blank'

  for (const [key, value] of Object.entries(params)) {
    const input = document.createElement('input')
    input.type = 'hidden'
    input.name = key
    input.value = String(value)
    form.appendChild(input)
  }

  document.body.appendChild(form)
  form.submit()
  document.body.removeChild(form)
}

export function SubscriptionFuelDialog(props: SubscriptionFuelDialogProps) {
  const fallbackQuotaPerUnit = 500_000
  const configuredMinimumQuota = props.minimumQuota || fallbackQuotaPerUnit
  const configuredQuotaStep = props.quotaStep || fallbackQuotaPerUnit
  const minimumConfiguredAmount = configuredMinimumQuota / fallbackQuotaPerUnit
  const [amount, setAmount] = useState(minimumConfiguredAmount)
  const [quote, setQuote] = useState<SubscriptionFuelQuote | null>(null)
  const [paymentMethod, setPaymentMethod] = useState('')
  const [paymentState, setPaymentState] = useState<FuelPaymentState | null>(
    null
  )
  const paymentOptions = useMemo(
    () => [
      ...(props.enableStripe ? [{ type: 'stripe', name: 'Stripe' }] : []),
      ...props.paymentMethods,
    ],
    [props.enableStripe, props.paymentMethods]
  )
  const selectedPaymentMethod = paymentMethod || paymentOptions[0]?.type || ''
  const isSafari =
    typeof navigator !== 'undefined' &&
    /^((?!chrome|android).)*safari/i.test(navigator.userAgent)
  const paymentOrderId = paymentState?.orderId
  const paymentStage = paymentState?.stage
  const isOpen = props.open
  const onCompleted = props.onCompleted
  const quotaPerUnit = quote?.quota_per_unit ?? 500_000
  const minimumAmount =
    (quote?.min_quota ?? configuredMinimumQuota) / quotaPerUnit
  const amountStep = (quote?.quota_step ?? configuredQuotaStep) / quotaPerUnit
  const quota = Math.round(amount * quotaPerUnit)
  const quoteMutation = useMutation({
    mutationFn: quoteSubscriptionFuel,
    onSuccess: (response) => setQuote(response.data ?? null),
    onError: () => setQuote(null),
  })
  const requestQuote = quoteMutation.mutate
  const purchaseMutation = useMutation({
    mutationFn: purchaseSubscriptionFuel,
    onSuccess: (response) => {
      const data = response.data
      const orderId = String(data?.order_id || '')
      const formUrl = response.url || ''
      const form = data?.form || null
      const payUrl = data?.pay_url || ''
      const qrCodeUrl = data?.qrcode_url || ''
      const checkoutUrl = data?.pay_link || data?.checkout_url || ''

      if (checkoutUrl) {
        window.location.assign(checkoutUrl)
        return
      }

      if (!orderId) {
        toast.error('支付订单缺少订单号，请稍后重试')
        return
      }

      if (formUrl && form) {
        submitExternalPaymentForm(formUrl, form, isSafari)
      }

      setPaymentState({
        stage: 'pending',
        orderId,
        amountDue: Number(data?.amount_due ?? quote?.amount_due ?? 0),
        payUrl,
        qrCodeUrl,
        formUrl: formUrl && form ? formUrl : '',
        form: formUrl && form ? form : null,
        message:
          qrCodeUrl || payUrl
            ? '请使用下方二维码完成支付，支付结果会自动同步。'
            : '请在支付页面完成付款，支付结果会自动同步。',
      })
      toast.success('加油订单已创建')
    },
    onError: () => toast.error('创建加油订单失败，请稍后重试'),
  })

  useEffect(() => {
    if (!isOpen || !paymentOrderId || paymentStage !== 'pending') {
      return
    }

    let active = true
    const poll = async () => {
      try {
        const response = await getSubscriptionOrderStatus(paymentOrderId)
        if (!active || !response.success || !response.data) return

        if (response.data.status === 'success') {
          setPaymentState((current) =>
            current
              ? {
                  ...current,
                  stage: 'success',
                  message: '支付成功，加油额度已经到账。',
                }
              : current
          )
          void onCompleted?.()
          return
        }

        if (response.data.status === 'expired') {
          setPaymentState((current) =>
            current
              ? {
                  ...current,
                  stage: 'failed',
                  message: '订单已过期或支付未完成，请重新发起支付。',
                }
              : current
          )
        }
      } catch {
        // Keep polling while the payment provider is processing the order.
      }
    }

    void poll()
    const timer = window.setInterval(() => void poll(), 2000)
    return () => {
      active = false
      window.clearInterval(timer)
    }
  }, [isOpen, onCompleted, paymentOrderId, paymentStage])

  const openExternalPayment = () => {
    if (paymentState?.formUrl && paymentState.form) {
      submitExternalPaymentForm(paymentState.formUrl, paymentState.form, false)
      return
    }
    if (paymentState?.payUrl) window.open(paymentState.payUrl, '_blank')
  }

  useEffect(() => {
    if (!props.open || !props.subscription || amount <= 0) return
    const timer = window.setTimeout(() => {
      requestQuote({ subscriptionId: props.subscription!.id, quota })
    }, 250)
    return () => window.clearTimeout(timer)
  }, [amount, props.open, props.subscription, quota, requestQuote])

  return (
    <Dialog open={props.open} onOpenChange={props.onOpenChange}>
      <DialogContent className='sm:max-w-md'>
        <DialogHeader>
          <DialogTitle>为 {props.title} 加油</DialogTitle>
          <DialogDescription>
            额度直接追加到当前月卡，模型权限和到期时间保持不变。
          </DialogDescription>
        </DialogHeader>

        <div className='space-y-4 py-2'>
          {paymentState ? (
            <SubscriptionFuelPaymentResult
              payment={paymentState}
              onOpenExternal={openExternalPayment}
              onClose={() => props.onOpenChange(false)}
            />
          ) : (
            <>
              <div className='bg-muted/30 rounded-lg border px-3 py-2.5 text-sm'>
                <div className='font-medium'>月卡到期时间不变</div>
                <div className='text-muted-foreground mt-1 text-xs tabular-nums'>
                  {props.subscription
                    ? formatDate(props.subscription.end_time)
                    : '--'}
                </div>
              </div>

              <div className='space-y-2'>
                <Label htmlFor='subscription-fuel-amount'>补充额度（$）</Label>
                <Input
                  id='subscription-fuel-amount'
                  type='number'
                  min={minimumAmount}
                  step={amountStep}
                  value={Number.isFinite(amount) ? amount : ''}
                  onChange={(event) => setAmount(Number(event.target.value))}
                />
                <p className='text-muted-foreground text-xs'>
                  最低 ${minimumAmount}，每次递增 ${amountStep}。
                </p>
              </div>

              <div className='grid grid-cols-2 gap-3 text-sm'>
                <div className='rounded-lg border px-3 py-2.5'>
                  <div className='text-muted-foreground text-xs'>加油单价</div>
                  <div className='mt-1 font-mono font-semibold'>
                    ¥{quote?.unit_price.toFixed(3) ?? '--'} / $1
                  </div>
                </div>
                <div className='rounded-lg border px-3 py-2.5'>
                  <div className='text-muted-foreground text-xs'>应付金额</div>
                  <div className='mt-1 font-mono font-semibold tabular-nums'>
                    ¥{quote?.amount_due.toFixed(2) ?? '--'}
                  </div>
                </div>
              </div>

              <div className='space-y-2'>
                <Label>支付方式</Label>
                <Select
                  value={selectedPaymentMethod}
                  onValueChange={(value) => setPaymentMethod(value ?? '')}
                >
                  <SelectTrigger>
                    <SelectValue placeholder='选择支付方式' />
                  </SelectTrigger>
                  <SelectContent>
                    {paymentOptions.map((method) => (
                      <SelectItem key={method.type} value={method.type}>
                        {method.name || method.type}
                      </SelectItem>
                    ))}
                  </SelectContent>
                </Select>
              </div>

              <Alert>
                <AlertCircle className='size-4' />
                <AlertDescription>
                  加油不延长有效期，也不升级模型权限。
                </AlertDescription>
              </Alert>
            </>
          )}
        </div>

        {!paymentState ? (
          <DialogFooter>
            <Button variant='outline' onClick={() => props.onOpenChange(false)}>
              取消
            </Button>
            <Button
              disabled={
                !quote ||
                !selectedPaymentMethod ||
                quoteMutation.isPending ||
                purchaseMutation.isPending
              }
              onClick={() =>
                props.subscription &&
                purchaseMutation.mutate({
                  subscriptionId: props.subscription.id,
                  quota,
                  paymentMethod: selectedPaymentMethod,
                })
              }
            >
              {purchaseMutation.isPending ? (
                <Loader2 className='mr-1 size-4 animate-spin' />
              ) : null}
              确认支付
            </Button>
          </DialogFooter>
        ) : null}
      </DialogContent>
    </Dialog>
  )
}
