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
import { useForm, type FieldErrors } from 'react-hook-form'
import { zodResolver } from '@hookform/resolvers/zod'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'
import { getCurrencyDisplay, getCurrencyLabel } from '@/lib/currency'
import { addTimeToDate } from '@/lib/time'
import { Button } from '@/components/ui/button'
import {
  Form,
  FormControl,
  FormDescription,
  FormField,
  FormItem,
  FormLabel,
  FormMessage,
} from '@/components/ui/form'
import { Input } from '@/components/ui/input'
import {
  Select,
  SelectContent,
  SelectGroup,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select'
import {
  Sheet,
  SheetClose,
  SheetContent,
  SheetDescription,
  SheetFooter,
  SheetHeader,
  SheetTitle,
} from '@/components/ui/sheet'
import { DateTimePicker } from '@/components/datetime-picker'
import { getAdminPlans } from '@/features/subscriptions/api'
import type { PlanRecord } from '@/features/subscriptions/types'
import { createRedemption, getRedemption, updateRedemption } from '../api'
import { REDEMPTION_TYPES, SUCCESS_MESSAGES } from '../constants'
import {
  REDEMPTION_FORM_DEFAULT_VALUES,
  downloadRedemptionTextFile,
  getRedemptionFormSchema,
  transformFormDataToPayload,
  transformRedemptionToFormDefaults,
  type RedemptionFormValues,
} from '../lib'
import { type Redemption } from '../types'
import { useRedemptions } from './redemptions-provider'

type RedemptionsMutateDrawerProps = {
  open: boolean
  onOpenChange: (open: boolean) => void
  currentRow?: Redemption
}

function getPlanCurrencyPrefix(currency?: string) {
  const normalized = (currency || '').toUpperCase()
  if (normalized === 'CNY') return 'RMB '
  if (normalized === 'EUR') return 'EUR '
  if (normalized === 'USD') return '$'
  return normalized ? `${normalized} ` : '$'
}

function trimToMaxRunes(value: string, maxRunes = 20) {
  return Array.from(value).slice(0, maxRunes).join('')
}

function formatCompactAmount(value: number) {
  const abs = Math.abs(value)
  if (abs === 0) return '0'
  if (abs >= 100) {
    return value.toFixed(Number.isInteger(value) ? 0 : 2).replace(/\.00$/, '')
  }
  if (abs >= 1) {
    return value
      .toFixed(2)
      .replace(/\.00$/, '')
      .replace(/(\.\d)0$/, '$1')
  }
  if (abs >= 0.01) {
    return value.toFixed(4).replace(/0+$/, '').replace(/\.$/, '')
  }
  return value.toFixed(6).replace(/0+$/, '').replace(/\.$/, '')
}

export function RedemptionsMutateDrawer({
  open,
  onOpenChange,
  currentRow,
}: RedemptionsMutateDrawerProps) {
  const { t } = useTranslation()
  const isUpdate = !!currentRow
  const { triggerRefresh } = useRedemptions()
  const [isSubmitting, setIsSubmitting] = useState(false)
  const [plans, setPlans] = useState<PlanRecord[]>([])

  const form = useForm<RedemptionFormValues>({
    resolver: zodResolver(getRedemptionFormSchema(t)),
    defaultValues: REDEMPTION_FORM_DEFAULT_VALUES,
  })

  const redeemType = form.watch('redeem_type')
  const selectedPlanId = form.watch('plan_id')

  useEffect(() => {
    if (!open) return

    getAdminPlans().then((result) => {
      if (result.success) {
        setPlans(result.data || [])
      }
    })

    if (isUpdate && currentRow) {
      getRedemption(currentRow.id).then((result) => {
        if (result.success && result.data) {
          form.reset(transformRedemptionToFormDefaults(result.data))
        }
      })
      return
    }

    form.reset(REDEMPTION_FORM_DEFAULT_VALUES)
  }, [open, isUpdate, currentRow, form])

  const onSubmit = async (data: RedemptionFormValues) => {
    setIsSubmitting(true)
    try {
      const basePayload = transformFormDataToPayload({
        ...data,
        name: resolveRedemptionName(data),
      })

      if (isUpdate && currentRow) {
        const result = await updateRedemption({
          ...basePayload,
          id: currentRow.id,
        })
        if (result.success) {
          toast.success(t(SUCCESS_MESSAGES.REDEMPTION_UPDATED))
          onOpenChange(false)
          triggerRefresh()
        }
      } else {
        const result = await createRedemption(basePayload)
        if (result.success) {
          const generatedCodes = result.data || []
          const count = generatedCodes.length
          toast.success(
            count > 1
              ? t('Successfully created {{count}} redemption codes', {
                  count,
                })
              : t(SUCCESS_MESSAGES.REDEMPTION_CREATED)
          )
          if (generatedCodes.length > 0) {
            downloadRedemptionTextFile(
              `${generatedCodes.join('\n')}\n`,
              basePayload.name || t('redemptions')
            )
          }
          onOpenChange(false)
          triggerRefresh()
        }
      }
    } catch (error) {
      toast.error(error instanceof Error ? error.message : t('Request failed'))
    } finally {
      setIsSubmitting(false)
    }
  }

  const onInvalid = (errors: FieldErrors<RedemptionFormValues>) => {
    const firstError = Object.values(errors)[0]
    const message =
      typeof firstError?.message === 'string'
        ? firstError.message
        : t('Please complete the required fields')
    toast.error(message)
  }

  const handleSetExpiry = (months: number, days: number, hours: number) => {
    const newDate = addTimeToDate(months, days, hours)
    form.setValue('expired_time', newDate)
  }

  const { meta: currencyMeta } = getCurrencyDisplay()
  const currencyLabel = getCurrencyLabel()
  const tokensOnly = currencyMeta.kind === 'tokens'
  const quotaLabel = t('Quota ({{currency}})', { currency: currencyLabel })
  const quotaPlaceholder = tokensOnly
    ? t('Enter quota in tokens')
    : t('Enter quota in {{currency}}', { currency: currencyLabel })

  const planOptions = useMemo(() => {
    const options = plans.map((record) => {
      const plan = record.plan
      return {
        value: String(plan.id),
        label: `${plan.title} (${getPlanCurrencyPrefix(plan.currency)}${Number(plan.price_amount || 0).toFixed(2)})`,
      }
    })

    if (
      selectedPlanId &&
      !options.some((option) => option.value === String(selectedPlanId))
    ) {
      options.push({
        value: String(selectedPlanId),
        label: t('Current Plan #{{id}}', { id: selectedPlanId }),
      })
    }

    return options
  }, [plans, selectedPlanId, t])

  const resolveRedemptionName = (data: RedemptionFormValues) => {
    const customName = String(data.name || '').trim()
    if (customName) {
      return trimToMaxRunes(customName)
    }

    if (data.redeem_type === REDEMPTION_TYPES.SUBSCRIPTION) {
      const selectedPlan = plans.find(
        (record) => record.plan.id === Number(data.plan_id || 0)
      )
      return trimToMaxRunes(
        selectedPlan?.plan.title || t('Subscription Redemption')
      )
    }

    const quotaText = tokensOnly
      ? `${t('Quota')} ${Math.round(Number(data.quota_dollars || 0))}`
      : `${t('Quota')} ${currencyLabel}${formatCompactAmount(Number(data.quota_dollars || 0))}`
    return trimToMaxRunes(quotaText)
  }

  return (
    <Sheet
      open={open}
      onOpenChange={(v) => {
        onOpenChange(v)
        if (!v) {
          form.reset(REDEMPTION_FORM_DEFAULT_VALUES)
        }
      }}
    >
      <SheetContent className='flex h-dvh w-full flex-col gap-0 overflow-hidden p-0 sm:max-w-[600px]'>
        <SheetHeader className='border-b px-4 py-3 text-start sm:px-6 sm:py-4'>
          <SheetTitle>
            {isUpdate
              ? t('Update Redemption Code')
              : t('Create Redemption Code')}
          </SheetTitle>
          <SheetDescription>
            {isUpdate
              ? t('Update the redemption code by providing necessary info.')
              : t(
                  'Add new redemption code(s) for quota top-up or subscription activation.'
                )}{' '}
            {t('Click save when you&apos;re done.')}
          </SheetDescription>
        </SheetHeader>
        <Form {...form}>
          <form
            id='redemption-form'
            onSubmit={form.handleSubmit(onSubmit, onInvalid)}
            className='flex-1 space-y-4 overflow-y-auto px-3 py-3 pb-4 sm:space-y-6 sm:px-4'
          >
            <FormField
              control={form.control}
              name='name'
              render={({ field }) => (
                <FormItem>
                  <FormLabel>{t('Name')}</FormLabel>
                  <FormControl>
                    <Input
                      {...field}
                      placeholder={t('Leave empty to auto-generate')}
                    />
                  </FormControl>
                  <FormDescription>
                    {t(
                      'Optional. Leave empty to auto-generate a name (max 20 characters)'
                    )}
                  </FormDescription>
                  <FormMessage />
                </FormItem>
              )}
            />

            <FormField
              control={form.control}
              name='redeem_type'
              render={({ field }) => (
                <FormItem>
                  <FormLabel>{t('Code Type')}</FormLabel>
                  <Select
                    items={[
                      {
                        value: REDEMPTION_TYPES.QUOTA,
                        label: t('Quota'),
                      },
                      {
                        value: REDEMPTION_TYPES.SUBSCRIPTION,
                        label: t('Subscription'),
                      },
                    ]}
                    value={field.value}
                    onValueChange={(value) => {
                      if (value === null) return
                      field.onChange(value)
                      if (value === REDEMPTION_TYPES.QUOTA) {
                        form.setValue('plan_id', undefined)
                      } else if (value === REDEMPTION_TYPES.SUBSCRIPTION) {
                        form.setValue('quota_dollars', 0)
                      }
                    }}
                  >
                    <FormControl>
                      <SelectTrigger>
                        <SelectValue />
                      </SelectTrigger>
                    </FormControl>
                    <SelectContent alignItemWithTrigger={false}>
                      <SelectGroup>
                        <SelectItem value={REDEMPTION_TYPES.QUOTA}>
                          {t('Quota')}
                        </SelectItem>
                        <SelectItem value={REDEMPTION_TYPES.SUBSCRIPTION}>
                          {t('Subscription')}
                        </SelectItem>
                      </SelectGroup>
                    </SelectContent>
                  </Select>
                  <FormDescription>
                    {t(
                      'Choose whether this code adds quota or activates a subscription plan.'
                    )}
                  </FormDescription>
                  <FormMessage />
                </FormItem>
              )}
            />

            {redeemType === REDEMPTION_TYPES.QUOTA ? (
              <FormField
                control={form.control}
                name='quota_dollars'
                render={({ field }) => (
                  <FormItem>
                    <FormLabel>{quotaLabel}</FormLabel>
                    <FormControl>
                      <Input
                        {...field}
                        type='number'
                        step={tokensOnly ? 1 : 0.01}
                        placeholder={quotaPlaceholder}
                        onChange={(e) =>
                          field.onChange(parseFloat(e.target.value) || 0)
                        }
                      />
                    </FormControl>
                    <FormDescription>
                      {tokensOnly
                        ? t('Enter the quota amount in tokens')
                        : t('Enter the quota amount in {{currency}}', {
                            currency: currencyLabel,
                          })}
                    </FormDescription>
                    <FormMessage />
                  </FormItem>
                )}
              />
            ) : redeemType === REDEMPTION_TYPES.SUBSCRIPTION ? (
              <FormField
                control={form.control}
                name='plan_id'
                render={({ field }) => (
                  <FormItem>
                    <FormLabel>{t('Subscription Plan')}</FormLabel>
                    <Select
                      items={planOptions}
                      value={field.value ? String(field.value) : ''}
                      onValueChange={(value) =>
                        field.onChange(value ? Number(value) : undefined)
                      }
                    >
                      <FormControl>
                        <SelectTrigger>
                          <SelectValue
                            placeholder={t('Select subscription plan')}
                          />
                        </SelectTrigger>
                      </FormControl>
                      <SelectContent alignItemWithTrigger={false}>
                        <SelectGroup>
                          {planOptions.map((option) => (
                            <SelectItem key={option.value} value={option.value}>
                              {option.label}
                            </SelectItem>
                          ))}
                        </SelectGroup>
                      </SelectContent>
                    </Select>
                    <FormDescription>
                      {t(
                        'The redeemed user will receive this subscription immediately'
                      )}
                    </FormDescription>
                    <FormMessage />
                  </FormItem>
                )}
              />
            ) : null}

            <FormField
              control={form.control}
              name='expired_time'
              render={({ field }) => (
                <FormItem>
                  <FormLabel>{t('Expiration Time')}</FormLabel>
                  <div className='space-y-2'>
                    <FormControl>
                      <DateTimePicker
                        value={field.value}
                        onChange={field.onChange}
                        placeholder={t('Never expires')}
                      />
                    </FormControl>
                    <div className='grid grid-cols-4 gap-1.5 sm:flex sm:gap-2'>
                      <Button
                        type='button'
                        variant='outline'
                        size='sm'
                        onClick={() => handleSetExpiry(0, 0, 0)}
                      >
                        {t('Never')}
                      </Button>
                      <Button
                        type='button'
                        variant='outline'
                        size='sm'
                        onClick={() => handleSetExpiry(1, 0, 0)}
                      >
                        {t('1M')}
                      </Button>
                      <Button
                        type='button'
                        variant='outline'
                        size='sm'
                        onClick={() => handleSetExpiry(0, 7, 0)}
                      >
                        {t('1W')}
                      </Button>
                      <Button
                        type='button'
                        variant='outline'
                        size='sm'
                        onClick={() => handleSetExpiry(0, 1, 0)}
                      >
                        {t('1 Day')}
                      </Button>
                    </div>
                  </div>
                  <FormDescription>
                    {t('Leave empty for never expires')}
                  </FormDescription>
                  <FormMessage />
                </FormItem>
              )}
            />

            {!isUpdate && (
              <FormField
                control={form.control}
                name='count'
                render={({ field }) => (
                  <FormItem>
                    <FormLabel>{t('Quantity')}</FormLabel>
                    <FormControl>
                      <Input
                        {...field}
                        type='number'
                        min='1'
                        max='100'
                        placeholder={t('Number of codes to create')}
                        onChange={(e) =>
                          field.onChange(parseInt(e.target.value, 10) || 1)
                        }
                      />
                    </FormControl>
                    <FormDescription>
                      {t('Create multiple redemption codes at once (1-100)')}
                    </FormDescription>
                    <FormMessage />
                  </FormItem>
                )}
              />
            )}
          </form>
        </Form>
        <SheetFooter className='grid grid-cols-2 gap-2 border-t px-4 py-3 sm:flex sm:px-6 sm:py-4'>
          <SheetClose render={<Button variant='outline' />}>
            {t('Close')}
          </SheetClose>
          <Button form='redemption-form' type='submit' disabled={isSubmitting}>
            {isSubmitting ? t('Saving...') : t('Save changes')}
          </Button>
        </SheetFooter>
      </SheetContent>
    </Sheet>
  )
}
