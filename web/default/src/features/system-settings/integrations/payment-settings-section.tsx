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
import * as React from 'react'
import * as z from 'zod'
import { type Control, type FieldPath, useForm } from 'react-hook-form'
import { zodResolver } from '@hookform/resolvers/zod'
import { useMutation, useQueryClient } from '@tanstack/react-query'
import { Code2, Eye, ShieldAlert } from 'lucide-react'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'
import {
  Alert,
  AlertAction,
  AlertDescription,
  AlertTitle,
} from '@/components/ui/alert'
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
import { Separator } from '@/components/ui/separator'
import { Switch } from '@/components/ui/switch'
import { Textarea } from '@/components/ui/textarea'
import { RiskAcknowledgementDialog } from '@/components/risk-acknowledgement-dialog'
import { confirmPaymentCompliance } from '../api'
import { SettingsSection } from '../components/settings-section'
import { useUpdateOption } from '../hooks/use-update-option'
import { AmountDiscountVisualEditor } from './amount-discount-visual-editor'
import { AmountOptionsVisualEditor } from './amount-options-visual-editor'
import { CreemProductsVisualEditor } from './creem-products-visual-editor'
import {
  formatJsonForEditor,
  getJsonError,
  normalizeJsonForComparison,
  removeTrailingSlash,
} from './utils'

const paymentSchema = z.object({
  Price: z.coerce.number().min(0),
  MinTopUp: z.coerce.number().min(0),
  CustomCallbackAddress: z.string().refine((value) => {
    const trimmed = value.trim()
    return !trimmed || /^https?:\/\//.test(trimmed)
  }, 'Provide a valid URL starting with http:// or https://'),
  AmountOptions: z.string().superRefine((value, ctx) => {
    const error = getJsonError(value, (parsed) => Array.isArray(parsed))
    if (error) ctx.addIssue({ code: z.ZodIssueCode.custom, message: error })
  }),
  AmountDiscount: z.string().superRefine((value, ctx) => {
    const error = getJsonError(
      value,
      (parsed) =>
        !!parsed && typeof parsed === 'object' && !Array.isArray(parsed)
    )
    if (error) ctx.addIssue({ code: z.ZodIssueCode.custom, message: error })
  }),
  StripeApiSecret: z.string(),
  StripeWebhookSecret: z.string(),
  StripePriceId: z.string(),
  StripeUnitPrice: z.coerce.number().min(0),
  StripeMinTopUp: z.coerce.number().min(0),
  StripePromotionCodesEnabled: z.boolean(),
  CreemApiKey: z.string(),
  CreemWebhookSecret: z.string(),
  CreemTestMode: z.boolean(),
  CreemProducts: z.string().superRefine((value, ctx) => {
    const error = getJsonError(value, (parsed) => Array.isArray(parsed))
    if (error) ctx.addIssue({ code: z.ZodIssueCode.custom, message: error })
  }),
})

export type PaymentFormValues = z.infer<typeof paymentSchema>
type PaymentFormInput = z.input<typeof paymentSchema>

type PaymentComplianceDefaults = {
  confirmed: boolean
  termsVersion: string
  confirmedAt: number
  confirmedBy: number
}

type PaymentSettingsSectionProps = {
  defaultValues: PaymentFormValues
  complianceDefaults: PaymentComplianceDefaults
}

export function PaymentSettingsSection({
  defaultValues,
  complianceDefaults,
}: PaymentSettingsSectionProps) {
  const { t } = useTranslation()
  const queryClient = useQueryClient()
  const updateOption = useUpdateOption()
  const initialRef = React.useRef(defaultValues)
  const defaultsSignature = React.useMemo(
    () => JSON.stringify(defaultValues),
    [defaultValues]
  )
  const [amountOptionsVisualMode, setAmountOptionsVisualMode] =
    React.useState(true)
  const [amountDiscountVisualMode, setAmountDiscountVisualMode] =
    React.useState(true)
  const [creemProductsVisualMode, setCreemProductsVisualMode] =
    React.useState(true)
  const [showComplianceDialog, setShowComplianceDialog] = React.useState(false)

  const form = useForm<PaymentFormInput, undefined, PaymentFormValues>({
    resolver: zodResolver(paymentSchema),
    mode: 'onChange',
    defaultValues: {
      ...defaultValues,
      AmountOptions: formatJsonForEditor(defaultValues.AmountOptions),
      AmountDiscount: formatJsonForEditor(defaultValues.AmountDiscount),
      CreemProducts: formatJsonForEditor(defaultValues.CreemProducts),
    },
  })

  React.useEffect(() => {
    const parsedDefaults = JSON.parse(defaultsSignature) as PaymentFormValues
    initialRef.current = parsedDefaults
    form.reset({
      ...parsedDefaults,
      AmountOptions: formatJsonForEditor(parsedDefaults.AmountOptions),
      AmountDiscount: formatJsonForEditor(parsedDefaults.AmountDiscount),
      CreemProducts: formatJsonForEditor(parsedDefaults.CreemProducts),
    })
  }, [defaultsSignature, form])

  const complianceConfirmed =
    complianceDefaults.confirmed && complianceDefaults.termsVersion === 'v1'

  const confirmComplianceMutation = useMutation({
    mutationFn: confirmPaymentCompliance,
    onSuccess: (data) => {
      if (!data.success) {
        toast.error(data.message || t('Failed to confirm compliance'))
        return
      }
      toast.success(t('Compliance confirmed successfully'))
      setShowComplianceDialog(false)
      void queryClient.invalidateQueries({ queryKey: ['system-options'] })
    },
    onError: (error: Error) =>
      toast.error(error.message || t('Failed to confirm compliance')),
  })

  const onSubmit = async (values: PaymentFormValues) => {
    const current = initialRef.current
    const normalized = {
      ...values,
      CustomCallbackAddress: removeTrailingSlash(values.CustomCallbackAddress),
      AmountOptions: values.AmountOptions.trim(),
      AmountDiscount: values.AmountDiscount.trim(),
      CreemProducts: values.CreemProducts.trim(),
    }
    const previous = {
      ...current,
      CustomCallbackAddress: removeTrailingSlash(current.CustomCallbackAddress),
      AmountOptions: current.AmountOptions.trim(),
      AmountDiscount: current.AmountDiscount.trim(),
      CreemProducts: current.CreemProducts.trim(),
    }
    const updates: Array<{ key: string; value: string | number | boolean }> = []
    const add = (key: string, value: string | number | boolean) => {
      if (value !== previous[key as keyof typeof previous])
        updates.push({ key, value })
    }

    add('Price', normalized.Price)
    add('MinTopUp', normalized.MinTopUp)
    add('CustomCallbackAddress', normalized.CustomCallbackAddress)
    if (
      normalizeJsonForComparison(normalized.AmountOptions) !==
      normalizeJsonForComparison(previous.AmountOptions)
    ) {
      updates.push({
        key: 'payment_setting.amount_options',
        value: normalized.AmountOptions,
      })
    }
    if (
      normalizeJsonForComparison(normalized.AmountDiscount) !==
      normalizeJsonForComparison(previous.AmountDiscount)
    ) {
      updates.push({
        key: 'payment_setting.amount_discount',
        value: normalized.AmountDiscount,
      })
    }
    add('StripeApiSecret', normalized.StripeApiSecret)
    add('StripeWebhookSecret', normalized.StripeWebhookSecret)
    add('StripePriceId', normalized.StripePriceId)
    add('StripeUnitPrice', normalized.StripeUnitPrice)
    add('StripeMinTopUp', normalized.StripeMinTopUp)
    add('StripePromotionCodesEnabled', normalized.StripePromotionCodesEnabled)
    add('CreemApiKey', normalized.CreemApiKey)
    add('CreemWebhookSecret', normalized.CreemWebhookSecret)
    add('CreemTestMode', normalized.CreemTestMode)
    if (
      normalizeJsonForComparison(normalized.CreemProducts) !==
      normalizeJsonForComparison(previous.CreemProducts)
    ) {
      updates.push({ key: 'CreemProducts', value: normalized.CreemProducts })
    }

    try {
      for (const update of updates) await updateOption.mutateAsync(update)
      initialRef.current = normalized
      if (updates.length > 0) toast.success(t('Settings saved successfully'))
    } catch (error) {
      toast.error(
        error instanceof Error ? error.message : t('Failed to save settings')
      )
    }
  }

  return (
    <SettingsSection
      title={t('Payment Gateway')}
      description={t(
        'Configure recharge pricing and supported payment gateways'
      )}
    >
      {!complianceConfirmed ? (
        <Alert variant='destructive' className='mb-6'>
          <ShieldAlert className='h-4 w-4' />
          <AlertTitle>{t('Compliance confirmation required')}</AlertTitle>
          <AlertDescription>
            {t(
              'Confirm the deployment compliance terms before enabling billing actions.'
            )}
          </AlertDescription>
          <AlertAction>
            <Button
              type='button'
              size='sm'
              variant='destructive'
              onClick={() => setShowComplianceDialog(true)}
            >
              {t('Confirm compliance')}
            </Button>
          </AlertAction>
        </Alert>
      ) : null}

      <Form {...form}>
        <form className='space-y-6' onSubmit={form.handleSubmit(onSubmit)}>
          <section className='space-y-4'>
            <div>
              <h3 className='text-lg font-medium'>{t('General Settings')}</h3>
              <p className='text-muted-foreground text-sm'>
                {t('Shared configuration for the ordinary wallet')}
              </p>
            </div>
            <div className='grid gap-6 md:grid-cols-3'>
              <NumberField
                control={form.control}
                name='Price'
                label={t('Price (local currency / USD)')}
              />
              <NumberField
                control={form.control}
                name='MinTopUp'
                label={t('Minimum top-up (USD)')}
              />
              <FormField
                control={form.control}
                name='CustomCallbackAddress'
                render={({ field }) => (
                  <FormItem>
                    <FormLabel>{t('Callback URL override')}</FormLabel>
                    <FormControl>
                      <Input
                        placeholder='https://gateway.example.com'
                        {...field}
                      />
                    </FormControl>
                    <FormDescription>
                      {t(
                        'Optional callback override. Leave blank to use the server address'
                      )}
                    </FormDescription>
                    <FormMessage />
                  </FormItem>
                )}
              />
            </div>
            <JsonField
              control={form.control}
              name='AmountOptions'
              label={t('Top-up amount options')}
              visualMode={amountOptionsVisualMode}
              onToggle={() => setAmountOptionsVisualMode((value) => !value)}
              visualEditor={(value, onChange) => (
                <AmountOptionsVisualEditor value={value} onChange={onChange} />
              )}
            />
            <JsonField
              control={form.control}
              name='AmountDiscount'
              label={t('Amount discounts')}
              visualMode={amountDiscountVisualMode}
              onToggle={() => setAmountDiscountVisualMode((value) => !value)}
              visualEditor={(value, onChange) => (
                <AmountDiscountVisualEditor value={value} onChange={onChange} />
              )}
            />
          </section>

          <Separator />

          <PaymentProviderSection
            title='Stripe'
            description={t(
              'Configure Stripe Checkout for ordinary wallet top-ups'
            )}
          >
            <SecretField
              control={form.control}
              name='StripeApiSecret'
              label={t('API secret')}
            />
            <SecretField
              control={form.control}
              name='StripeWebhookSecret'
              label={t('Webhook secret')}
            />
            <FormField
              control={form.control}
              name='StripePriceId'
              render={({ field }) => (
                <FormItem>
                  <FormLabel>{t('Price ID')}</FormLabel>
                  <FormControl>
                    <Input placeholder='price_xxx' {...field} />
                  </FormControl>
                  <FormMessage />
                </FormItem>
              )}
            />
            <NumberField
              control={form.control}
              name='StripeUnitPrice'
              label={t('Unit price (local currency / USD)')}
            />
            <NumberField
              control={form.control}
              name='StripeMinTopUp'
              label={t('Minimum Stripe top-up (USD)')}
            />
            <FormField
              control={form.control}
              name='StripePromotionCodesEnabled'
              render={({ field }) => (
                <FormItem className='flex items-center justify-between rounded-lg border p-4 md:col-span-2'>
                  <div>
                    <FormLabel>{t('Promotion codes')}</FormLabel>
                    <FormDescription>
                      {t('Allow Stripe promotion codes during checkout')}
                    </FormDescription>
                  </div>
                  <FormControl>
                    <Switch
                      checked={field.value}
                      onCheckedChange={field.onChange}
                    />
                  </FormControl>
                </FormItem>
              )}
            />
          </PaymentProviderSection>

          <Separator />

          <PaymentProviderSection
            title='Creem'
            description={t(
              'Configure Creem Checkout products for ordinary wallet top-ups'
            )}
          >
            <SecretField
              control={form.control}
              name='CreemApiKey'
              label={t('API key')}
            />
            <SecretField
              control={form.control}
              name='CreemWebhookSecret'
              label={t('Webhook secret')}
            />
            <FormField
              control={form.control}
              name='CreemTestMode'
              render={({ field }) => (
                <FormItem className='flex items-center justify-between rounded-lg border p-4'>
                  <div>
                    <FormLabel>{t('Test mode')}</FormLabel>
                    <FormDescription>
                      {t('Use the Creem test API endpoint')}
                    </FormDescription>
                  </div>
                  <FormControl>
                    <Switch
                      checked={field.value}
                      onCheckedChange={field.onChange}
                    />
                  </FormControl>
                </FormItem>
              )}
            />
            <FormField
              control={form.control}
              name='CreemProducts'
              render={({ field }) => (
                <FormItem className='md:col-span-2'>
                  <div className='mb-2 flex items-center justify-between gap-2'>
                    <FormLabel>{t('Products')}</FormLabel>
                    <Button
                      type='button'
                      variant='outline'
                      size='sm'
                      onClick={() =>
                        setCreemProductsVisualMode((value) => !value)
                      }
                    >
                      {creemProductsVisualMode ? (
                        <>
                          <Code2 className='mr-2 size-3' />
                          {t('JSON Editor')}
                        </>
                      ) : (
                        <>
                          <Eye className='mr-2 size-3' />
                          {t('Visual Editor')}
                        </>
                      )}
                    </Button>
                  </div>
                  <FormControl>
                    {creemProductsVisualMode ? (
                      <CreemProductsVisualEditor
                        value={field.value}
                        onChange={field.onChange}
                      />
                    ) : (
                      <Textarea rows={5} {...field} />
                    )}
                  </FormControl>
                  <FormDescription>
                    {t('Configure Creem products as a JSON array')}
                  </FormDescription>
                  <FormMessage />
                </FormItem>
              )}
            />
          </PaymentProviderSection>

          <div className='flex justify-end'>
            <Button type='submit' disabled={updateOption.isPending}>
              {updateOption.isPending ? t('Saving...') : t('Save settings')}
            </Button>
          </div>
        </form>
      </Form>

      <RiskAcknowledgementDialog
        open={showComplianceDialog}
        onOpenChange={setShowComplianceDialog}
        title={t('Confirm payment compliance')}
        description={t(
          'Review the deployment responsibilities before enabling billing actions.'
        )}
        items={[
          t(
            'You have authorization for the connected model APIs, accounts, keys, and quotas.'
          ),
          t(
            'You are responsible for lawful deployment, operation, and charging behavior.'
          ),
          t(
            'You will protect user data and follow applicable platform and legal requirements.'
          ),
        ]}
        checklist={[
          t('I understand and accept these responsibilities.') as string,
        ]}
        confirmText={t('Confirm compliance')}
        cancelText={t('Cancel')}
        isLoading={confirmComplianceMutation.isPending}
        onConfirm={() => confirmComplianceMutation.mutate()}
      />
    </SettingsSection>
  )
}

function PaymentProviderSection(props: {
  title: string
  description: string
  children: React.ReactNode
}) {
  return (
    <section className='space-y-4'>
      <div>
        <h3 className='text-lg font-medium'>{props.title}</h3>
        <p className='text-muted-foreground text-sm'>{props.description}</p>
      </div>
      <div className='grid gap-6 md:grid-cols-2'>{props.children}</div>
    </section>
  )
}

type PaymentFieldProps = {
  control: Control<PaymentFormInput, undefined, PaymentFormValues>
  name: FieldPath<PaymentFormInput>
  label: string
}

function NumberField(props: PaymentFieldProps) {
  return (
    <FormField
      control={props.control}
      name={props.name}
      render={({ field }) => (
        <FormItem>
          <FormLabel>{props.label}</FormLabel>
          <FormControl>
            <Input
              type='number'
              min={0}
              step='0.01'
              value={(field.value ?? 0) as number}
              onChange={(event) => field.onChange(event.target.valueAsNumber)}
            />
          </FormControl>
          <FormMessage />
        </FormItem>
      )}
    />
  )
}

function SecretField(props: PaymentFieldProps) {
  return (
    <FormField
      control={props.control}
      name={props.name}
      render={({ field }) => (
        <FormItem>
          <FormLabel>{props.label}</FormLabel>
          <FormControl>
            <Input
              type='password'
              autoComplete='new-password'
              placeholder={props.label}
              {...field}
              value={String(field.value ?? '')}
            />
          </FormControl>
          <FormMessage />
        </FormItem>
      )}
    />
  )
}

function JsonField(
  props: PaymentFieldProps & {
    visualMode: boolean
    onToggle: () => void
    visualEditor: (
      value: string,
      onChange: (value: string) => void
    ) => React.ReactElement
  }
) {
  return (
    <FormField
      control={props.control}
      name={props.name}
      render={({ field }) => (
        <FormItem>
          <div className='mb-2 flex items-center justify-between gap-2'>
            <FormLabel>{props.label}</FormLabel>
            <Button
              type='button'
              variant='outline'
              size='sm'
              onClick={props.onToggle}
            >
              {props.visualMode ? (
                <>
                  <Code2 className='mr-2 size-3' />
                  JSON
                </>
              ) : (
                <>
                  <Eye className='mr-2 size-3' />
                  Visual
                </>
              )}
            </Button>
          </div>
          <FormControl>
            {props.visualMode ? (
              props.visualEditor(String(field.value ?? ''), field.onChange)
            ) : (
              <Textarea rows={4} {...field} value={String(field.value ?? '')} />
            )}
          </FormControl>
          <FormMessage />
        </FormItem>
      )}
    />
  )
}
