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
import { ExternalLink, Loader2, QrCode } from 'lucide-react'
import { QRCodeCanvas } from 'qrcode.react'
import { Button } from '@/components/ui/button'

export type FuelPaymentStage = 'pending' | 'success' | 'failed'

export interface FuelPaymentState {
  stage: FuelPaymentStage
  orderId: string
  amountDue: number
  payUrl: string
  qrCodeUrl: string
  formUrl: string
  form: Record<string, unknown> | null
  message: string
}

export function SubscriptionFuelPaymentResult(props: {
  payment: FuelPaymentState
  onOpenExternal: () => void
  onClose: () => void
}) {
  const isPending = props.payment.stage === 'pending'

  return (
    <div className='space-y-4 rounded-xl border p-4'>
      <div className='flex items-start gap-3'>
        <div className='bg-muted flex size-10 shrink-0 items-center justify-center rounded-full'>
          {getFuelPaymentStatusIcon(props.payment.stage)}
        </div>
        <div className='min-w-0'>
          <div className='font-semibold'>
            {getFuelPaymentStatusTitle(props.payment.stage)}
          </div>
          <p className='text-muted-foreground mt-1 text-sm leading-6'>
            {props.payment.message}
          </p>
        </div>
      </div>

      <div className='grid grid-cols-2 gap-3 text-sm'>
        <div className='rounded-lg border px-3 py-2.5'>
          <div className='text-muted-foreground text-xs'>应付金额</div>
          <div className='mt-1 font-mono font-semibold'>
            ¥{props.payment.amountDue.toFixed(2)}
          </div>
        </div>
        <div className='rounded-lg border px-3 py-2.5'>
          <div className='text-muted-foreground text-xs'>订单号</div>
          <div className='mt-1 font-mono text-xs break-all'>
            {props.payment.orderId}
          </div>
        </div>
      </div>

      {isPending ? <PaymentQrPanel payment={props.payment} /> : null}

      <div className='flex flex-wrap justify-end gap-2'>
        {isPending &&
        (props.payment.payUrl ||
          (props.payment.formUrl && props.payment.form)) ? (
          <Button variant='outline' onClick={props.onOpenExternal}>
            <ExternalLink className='mr-1 size-4' />
            打开支付页面
          </Button>
        ) : null}
        <Button onClick={props.onClose}>关闭</Button>
      </div>
    </div>
  )
}

function PaymentQrPanel(props: { payment: FuelPaymentState }) {
  return (
    <div className='space-y-3 rounded-xl border bg-white p-3 dark:bg-slate-950'>
      {props.payment.qrCodeUrl ? (
        <img
          src={props.payment.qrCodeUrl}
          alt='payment-qrcode'
          className='mx-auto size-48 object-contain'
        />
      ) : props.payment.payUrl ? (
        <QRCodeCanvas
          value={props.payment.payUrl}
          size={192}
          className='mx-auto'
        />
      ) : (
        <div className='text-muted-foreground rounded-lg border border-dashed px-4 py-8 text-center text-sm'>
          请点击下方按钮打开支付页面。
        </div>
      )}
      {props.payment.qrCodeUrl || props.payment.payUrl ? (
        <div className='text-muted-foreground flex items-center justify-center gap-2 text-center text-xs'>
          <QrCode className='size-4' />
          请使用微信扫码完成支付
        </div>
      ) : null}
    </div>
  )
}

function getFuelPaymentStatusTitle(stage: FuelPaymentStage): string {
  if (stage === 'pending') return '订单已创建，等待支付'
  if (stage === 'success') return '支付成功'
  return '支付未完成'
}

function getFuelPaymentStatusIcon(stage: FuelPaymentStage) {
  if (stage === 'pending') return <Loader2 className='size-5 animate-spin' />
  if (stage === 'success')
    return <span className='text-success text-lg'>✓</span>
  return <span className='text-destructive text-lg'>!</span>
}
