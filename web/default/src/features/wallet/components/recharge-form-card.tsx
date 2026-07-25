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
import {
  CreditCard,
  ExternalLink,
  Gift,
  Loader2,
  Receipt,
  WalletCards,
} from 'lucide-react'
import { useTranslation } from 'react-i18next'
import { formatUsdAmount } from '@/lib/format'
import { cn } from '@/lib/utils'
import { Alert, AlertDescription } from '@/components/ui/alert'
import { Button } from '@/components/ui/button'
import { Card, CardContent, CardHeader } from '@/components/ui/card'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { Skeleton } from '@/components/ui/skeleton'
import { TitledCard } from '@/components/ui/titled-card'
import {
  calculatePresetPricing,
  formatPaymentAmount,
  getDiscountLabel,
} from '../lib'
import type { CreemProduct, PresetAmount, TopupInfo } from '../types'
import { CreemProductsSection } from './creem-products-section'

interface RechargeFormCardProps {
  topupInfo: TopupInfo | null
  presetAmounts: PresetAmount[]
  selectedPreset: number | null
  onSelectPreset: (preset: PresetAmount) => void
  topupAmount: number
  onTopupAmountChange: (amount: number) => void
  paymentAmount: number
  calculating: boolean
  onPaymentMethodSelect: () => void
  paymentLoading: boolean
  redemptionCode: string
  onRedemptionCodeChange: (code: string) => void
  onRedeem: () => void
  redeeming: boolean
  topupLink?: string
  loading?: boolean
  onOpenBilling?: () => void
  creemProducts?: CreemProduct[]
  enableCreemTopup?: boolean
  onCreemProductSelect?: (product: CreemProduct) => void
  showRedemptionSection?: boolean
  compact?: boolean
}

export function RechargeFormCard(props: RechargeFormCardProps) {
  const { t } = useTranslation()
  const [localAmount, setLocalAmount] = useState(String(props.topupAmount))
  const sectionLabelClassName = 'text-muted-foreground text-xs font-medium'

  useEffect(() => {
    setLocalAmount(String(props.topupAmount))
  }, [props.topupAmount])

  if (props.loading) {
    return (
      <Card className='gap-0 overflow-hidden py-0'>
        <CardHeader className='border-b p-3 sm:p-5'>
          <Skeleton className='h-6 w-32' />
          <Skeleton className='mt-2 h-4 w-48' />
        </CardHeader>
        <CardContent className='space-y-4 p-3 sm:p-5'>
          <Skeleton className='h-20 w-full' />
          <Skeleton className='h-10 w-full' />
          <Skeleton className='h-10 w-full' />
        </CardContent>
      </Card>
    )
  }

  const minTopup =
    props.topupInfo?.stripe_min_topup || props.topupInfo?.min_topup || 1
  const hasStripe = props.topupInfo?.enable_stripe_topup === true
  const hasCreem =
    props.enableCreemTopup && (props.creemProducts?.length ?? 0) > 0
  const hasRedemption = props.topupInfo?.enable_redemption !== false

  return (
    <TitledCard
      title={t('Balance top-up')}
      description={t('Add balance to the standard wallet for API usage')}
      icon={<WalletCards className='h-4 w-4' />}
      action={
        props.onOpenBilling ? (
          <Button variant='outline' size='sm' onClick={props.onOpenBilling}>
            <Receipt className='h-4 w-4' />
            {t('Billing History')}
          </Button>
        ) : null
      }
      className={props.compact ? 'rounded-xl' : undefined}
      contentClassName={props.compact ? 'space-y-4' : 'space-y-6'}
    >
      {hasStripe ? (
        <div className='space-y-4'>
          <div className='space-y-2'>
            <Label className={sectionLabelClassName}>
              {t('Recharge quota (USD)')}
            </Label>
            <div className='grid grid-cols-2 gap-2 sm:grid-cols-4'>
              {props.presetAmounts.map((preset) => {
                const discount =
                  preset.discount ||
                  props.topupInfo?.discount?.[preset.value] ||
                  1
                const pricing = calculatePresetPricing(
                  preset.value,
                  1,
                  discount,
                  1
                )
                return (
                  <Button
                    key={preset.value}
                    variant='outline'
                    onClick={() => props.onSelectPreset(preset)}
                    className={cn(
                      'h-auto min-h-16 flex-col items-start gap-1 px-3 py-2 text-left',
                      props.selectedPreset === preset.value &&
                        'border-foreground bg-foreground/5'
                    )}
                  >
                    <span className='text-base font-semibold'>
                      {formatUsdAmount(pricing.displayValue)}
                    </span>
                    <span className='text-muted-foreground text-xs'>
                      {pricing.hasDiscount
                        ? getDiscountLabel(discount)
                        : t('Pay {{amount}}', {
                            amount: formatPaymentAmount(pricing.actualPrice),
                          })}
                    </span>
                  </Button>
                )
              })}
            </div>
          </div>

          <div className='space-y-2'>
            <Label htmlFor='topup-amount' className={sectionLabelClassName}>
              {t('Custom amount')}
            </Label>
            <div className='grid gap-2 sm:grid-cols-[minmax(0,1fr)_220px]'>
              <Input
                id='topup-amount'
                type='number'
                min={minTopup}
                value={localAmount}
                onChange={(event) => {
                  setLocalAmount(event.target.value)
                  props.onTopupAmountChange(Number(event.target.value) || 0)
                }}
                placeholder={t('Minimum {{amount}} USD', { amount: minTopup })}
              />
              <div className='bg-muted/30 flex items-center justify-between rounded-md border px-3 text-sm'>
                <span className='text-muted-foreground'>
                  {t('Payment amount')}
                </span>
                {props.calculating ? (
                  <Skeleton className='h-5 w-16' />
                ) : (
                  <span className='font-semibold'>
                    {formatPaymentAmount(props.paymentAmount)}
                  </span>
                )}
              </div>
            </div>
          </div>

          <Button
            className='w-full sm:w-auto'
            onClick={props.onPaymentMethodSelect}
            disabled={props.paymentLoading || props.topupAmount < minTopup}
          >
            {props.paymentLoading ? (
              <Loader2 className='animate-spin' />
            ) : (
              <CreditCard />
            )}
            {props.paymentLoading
              ? t('Opening payment...')
              : t('Pay with Stripe')}
          </Button>
        </div>
      ) : (
        <Alert>
          <AlertDescription>
            {t('Online top-up is not enabled.')}
          </AlertDescription>
        </Alert>
      )}

      {hasCreem && props.onCreemProductSelect ? (
        <div className='space-y-3 border-t pt-4'>
          <Label className={sectionLabelClassName}>{t('Creem Payment')}</Label>
          <CreemProductsSection
            products={props.creemProducts ?? []}
            onProductSelect={props.onCreemProductSelect}
          />
        </div>
      ) : null}

      {props.showRedemptionSection !== false && hasRedemption ? (
        <div className='space-y-3 border-t pt-4'>
          <div className='flex items-center gap-2'>
            <Gift className='text-muted-foreground h-4 w-4' />
            <Label htmlFor='redemption-code' className={sectionLabelClassName}>
              {t('Have a Code?')}
            </Label>
          </div>
          <div className='grid grid-cols-[minmax(0,1fr)_auto] gap-2'>
            <Input
              id='redemption-code'
              value={props.redemptionCode}
              onChange={(event) =>
                props.onRedemptionCodeChange(event.target.value)
              }
              placeholder={t('Enter your redemption code')}
            />
            <Button
              onClick={props.onRedeem}
              disabled={props.redeeming}
              variant='outline'
            >
              {props.redeeming ? <Loader2 className='animate-spin' /> : null}
              {t('Redeem')}
            </Button>
          </div>
          {props.topupLink ? (
            <p className='text-muted-foreground text-xs'>
              {t('Need a redemption code?')}{' '}
              <a
                href={props.topupLink}
                target='_blank'
                rel='noopener noreferrer'
                className='inline-flex items-center gap-1 underline-offset-4 hover:underline'
              >
                {t('Get one here')} <ExternalLink className='h-3 w-3' />
              </a>
            </p>
          ) : null}
        </div>
      ) : null}
    </TitledCard>
  )
}
