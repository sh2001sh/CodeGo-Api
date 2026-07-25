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
import { useCallback, useEffect, useMemo, useState } from 'react'
import { Pencil, Plus, RotateCcw } from 'lucide-react'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'
import { Alert, AlertDescription } from '@/components/ui/alert'
import { Button } from '@/components/ui/button'
import {
  Dialog,
  DialogContent,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from '@/components/ui/dialog'
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
  SheetContent,
  SheetDescription,
  SheetHeader,
  SheetTitle,
} from '@/components/ui/sheet'
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from '@/components/ui/table'
import { Textarea } from '@/components/ui/textarea'
import { ConfirmDialog } from '@/components/confirm-dialog'
import { StatusBadge } from '@/components/status-badge'
import {
  createUserSubscription,
  deleteUserSubscription,
  getAdminPlans,
  getUserSubscriptions,
  invalidateUserSubscription,
  resetUserSubscriptionQuota,
  updateUserSubscription,
} from '../../api'
import {
  formatSubscriptionQuotaAmount,
  formatTimestamp,
  parseSubscriptionQuotaUSDToUnits,
  subscriptionQuotaUnitsToUSD,
} from '../../lib'
import type { PlanRecord, UserSubscriptionRecord } from '../../types'

function toDateTimeLocal(unixSeconds: number) {
  if (!unixSeconds) return ''
  const date = new Date(unixSeconds * 1000)
  const offset = date.getTimezoneOffset()
  const local = new Date(date.getTime() - offset * 60 * 1000)
  return local.toISOString().slice(0, 16)
}

function fromDateTimeLocal(value: string) {
  if (!value) return 0
  return Math.floor(new Date(value).getTime() / 1000)
}

function getCurrencySymbol(currency?: string) {
  const normalized = (currency || '').toUpperCase()
  if (normalized === 'CNY') return 'RMB '
  if (normalized === 'EUR') return 'EUR '
  return '$'
}

interface Props {
  open: boolean
  onOpenChange: (open: boolean) => void
  user: { id: number; username?: string } | null
  onSuccess?: () => void
}

function SubscriptionStatusBadge(props: {
  sub: UserSubscriptionRecord['subscription']
  t: (key: string) => string
}) {
  const now = Date.now() / 1000
  const isExpired = (props.sub.end_time || 0) > 0 && props.sub.end_time < now
  const isActive = props.sub.status === 'active' && !isExpired

  if (isActive) {
    return (
      <StatusBadge
        label={props.t('Active')}
        variant='success'
        copyable={false}
      />
    )
  }

  if (props.sub.status === 'cancelled') {
    return (
      <StatusBadge
        label={props.t('Invalidated')}
        variant='neutral'
        copyable={false}
      />
    )
  }

  return (
    <StatusBadge
      label={props.t('Expired')}
      variant='neutral'
      copyable={false}
    />
  )
}

export function UserSubscriptionsDialog(props: Props) {
  const { t } = useTranslation()
  const [loading, setLoading] = useState(false)
  const [creating, setCreating] = useState(false)
  const [plans, setPlans] = useState<PlanRecord[]>([])
  const [subs, setSubs] = useState<UserSubscriptionRecord[]>([])
  const [selectedPlanId, setSelectedPlanId] = useState('')
  const [confirmAction, setConfirmAction] = useState<{
    type: 'invalidate' | 'delete' | 'reset'
    subId: number
  } | null>(null)
  const [editingSub, setEditingSub] = useState<
    UserSubscriptionRecord['subscription'] | null
  >(null)
  const [editValues, setEditValues] = useState({
    start_time: '',
    end_time: '',
    status: 'active',
    amount_total: '0',
    amount_used: '0',
    period_amount: '0',
    period_used: '0',
    model_limits: '',
  })

  const planTitleMap = useMemo(() => {
    const map = new Map<number, string>()
    plans.forEach((p) => {
      if (p.plan.id) {
        map.set(p.plan.id, p.plan.title || `#${p.plan.id}`)
      }
    })
    return map
  }, [plans])

  const loadData = useCallback(async () => {
    if (!props.user?.id) return

    setLoading(true)
    try {
      const [plansRes, subsRes] = await Promise.all([
        getAdminPlans(),
        getUserSubscriptions(props.user.id),
      ])

      if (plansRes.success) {
        setPlans(plansRes.data || [])
      }
      if (subsRes.success) {
        setSubs(subsRes.data || [])
      }
    } catch {
      toast.error(t('Loading failed'))
    } finally {
      setLoading(false)
    }
  }, [props.user?.id, t])

  useEffect(() => {
    if (props.open && props.user?.id) {
      setSelectedPlanId('')
      loadData()
    }
  }, [props.open, props.user?.id, loadData])

  const handleCreate = async () => {
    if (!props.user?.id || !selectedPlanId) {
      toast.error(t('Please select a subscription plan'))
      return
    }

    setCreating(true)
    try {
      const res = await createUserSubscription(props.user.id, {
        plan_id: Number(selectedPlanId),
      })
      if (res.success) {
        toast.success(res.data?.message || t('Added successfully'))
        setSelectedPlanId('')
        await loadData()
        props.onSuccess?.()
      }
    } catch {
      toast.error(t('Request failed'))
    } finally {
      setCreating(false)
    }
  }

  const handleConfirmAction = async () => {
    if (!confirmAction) return

    try {
      if (confirmAction.type === 'invalidate') {
        const res = await invalidateUserSubscription(confirmAction.subId)
        if (res.success) {
          toast.success(res.data?.message || t('Has been invalidated'))
          await loadData()
          props.onSuccess?.()
        }
      } else if (confirmAction.type === 'reset') {
        const res = await resetUserSubscriptionQuota(confirmAction.subId, {
          advance_reset_time: true,
        })
        if (res.success) {
          toast.success(res.data?.message || t('Subscription quota reset'))
          await loadData()
          props.onSuccess?.()
        }
      } else {
        const res = await deleteUserSubscription(confirmAction.subId)
        if (res.success) {
          toast.success(t('Deleted'))
          await loadData()
          props.onSuccess?.()
        }
      }
    } catch {
      toast.error(t('Operation failed'))
    } finally {
      setConfirmAction(null)
    }
  }

  const openEditDialog = (sub: UserSubscriptionRecord['subscription']) => {
    setEditingSub(sub)
    setEditValues({
      start_time: toDateTimeLocal(sub.start_time),
      end_time: toDateTimeLocal(sub.end_time),
      status: sub.status || 'active',
      amount_total: String(subscriptionQuotaUnitsToUSD(sub.amount_total)),
      amount_used: String(subscriptionQuotaUnitsToUSD(sub.amount_used)),
      period_amount: String(subscriptionQuotaUnitsToUSD(sub.period_amount)),
      period_used: String(subscriptionQuotaUnitsToUSD(sub.period_used)),
      model_limits: sub.model_limits || '',
    })
  }

  const handleSaveEdit = async () => {
    if (!editingSub) return

    try {
      const res = await updateUserSubscription(editingSub.id, {
        start_time: fromDateTimeLocal(editValues.start_time),
        end_time: fromDateTimeLocal(editValues.end_time),
        status: editValues.status,
        amount_total: parseSubscriptionQuotaUSDToUnits(editValues.amount_total),
        amount_used: parseSubscriptionQuotaUSDToUnits(editValues.amount_used),
        period_amount: parseSubscriptionQuotaUSDToUnits(
          editValues.period_amount
        ),
        period_used: parseSubscriptionQuotaUSDToUnits(editValues.period_used),
        model_limits: editValues.model_limits || '',
      })
      if (res.success) {
        toast.success(res.data?.message || t('Update succeeded'))
        setEditingSub(null)
        await loadData()
        props.onSuccess?.()
      }
    } catch {
      toast.error(t('Request failed'))
    }
  }

  return (
    <>
      <Sheet open={props.open} onOpenChange={props.onOpenChange}>
        <SheetContent className='overflow-y-auto sm:max-w-2xl'>
          <SheetHeader>
            <SheetTitle>{t('User Subscription Management')}</SheetTitle>
            <SheetDescription>
              {props.user?.username || '-'} (ID: {props.user?.id || '-'})
            </SheetDescription>
          </SheetHeader>

          <div className='mt-4 space-y-4'>
            <Alert>
              <AlertDescription>
                {t(
                  'Dashboard used quota is the user-wide cumulative consumption. The used quota inside this dialog is the usage inside each subscription package and does not need to match the dashboard value.'
                )}
              </AlertDescription>
            </Alert>

            <div className='flex gap-2'>
              <Select
                items={[
                  ...plans.map((p) => ({
                    value: String(p.plan.id),
                    label: `${p.plan.title} (${getCurrencySymbol(
                      p.plan.currency
                    )}${Number(p.plan.price_amount || 0).toFixed(2)})`,
                  })),
                ]}
                value={selectedPlanId}
                onValueChange={(value) =>
                  value !== null && setSelectedPlanId(value)
                }
              >
                <SelectTrigger className='flex-1'>
                  <SelectValue placeholder={t('Select subscription plan')} />
                </SelectTrigger>
                <SelectContent alignItemWithTrigger={false}>
                  <SelectGroup>
                    {plans.map((p) => (
                      <SelectItem key={p.plan.id} value={String(p.plan.id)}>
                        {p.plan.title} ({getCurrencySymbol(p.plan.currency)}
                        {Number(p.plan.price_amount || 0).toFixed(2)})
                      </SelectItem>
                    ))}
                  </SelectGroup>
                </SelectContent>
              </Select>
              <Button
                onClick={handleCreate}
                disabled={creating || !selectedPlanId}
              >
                <Plus className='mr-1 h-4 w-4' />
                {t('Add subscription')}
              </Button>
            </div>

            <div className='rounded-md border'>
              <Table>
                <TableHeader>
                  <TableRow>
                    <TableHead>ID</TableHead>
                    <TableHead>{t('Plan')}</TableHead>
                    <TableHead>{t('Status')}</TableHead>
                    <TableHead>{t('Validity')}</TableHead>
                    <TableHead>{t('Quota')}</TableHead>
                    <TableHead className='text-right'>{t('Actions')}</TableHead>
                  </TableRow>
                </TableHeader>
                <TableBody>
                  {loading ? (
                    <TableRow>
                      <TableCell colSpan={6} className='py-8 text-center'>
                        {t('Loading...')}
                      </TableCell>
                    </TableRow>
                  ) : subs.length === 0 ? (
                    <TableRow>
                      <TableCell
                        colSpan={6}
                        className='text-muted-foreground py-8 text-center'
                      >
                        {t('No subscription records')}
                      </TableCell>
                    </TableRow>
                  ) : (
                    subs.map((record) => {
                      const sub = record.subscription
                      const now = Date.now() / 1000
                      const isExpired =
                        (sub.end_time || 0) > 0 && sub.end_time < now
                      const isActive = sub.status === 'active' && !isExpired
                      const total = Number(sub.amount_total || 0)
                      const used = Number(sub.amount_used || 0)

                      return (
                        <TableRow key={sub.id}>
                          <TableCell>#{sub.id}</TableCell>
                          <TableCell>
                            <div>
                              <div className='font-medium'>
                                {planTitleMap.get(sub.plan_id) ||
                                  `#${sub.plan_id}`}
                              </div>
                              <div className='text-muted-foreground text-xs'>
                                {t('Source')}: {sub.source || '-'}
                              </div>
                            </div>
                          </TableCell>
                          <TableCell>
                            <SubscriptionStatusBadge sub={sub} t={t} />
                          </TableCell>
                          <TableCell>
                            <div className='text-xs'>
                              <div>
                                {t('Start')}: {formatTimestamp(sub.start_time)}
                              </div>
                              <div>
                                {t('End')}: {formatTimestamp(sub.end_time)}
                              </div>
                              <div>
                                {t('Next reset')}:{' '}
                                {sub.next_reset_time
                                  ? formatTimestamp(sub.next_reset_time)
                                  : t('Disabled')}
                              </div>
                            </div>
                          </TableCell>
                          <TableCell>
                            <div className='text-xs'>
                              <div>
                                {t('Total')}:{' '}
                                {total > 0
                                  ? `${formatSubscriptionQuotaAmount(used)}/${formatSubscriptionQuotaAmount(total)}`
                                  : t('Unlimited')}
                              </div>
                              <div>
                                {t('Period')}:{' '}
                                {Number(sub.period_amount || 0) > 0
                                  ? `${formatSubscriptionQuotaAmount(Number(sub.period_used || 0))}/${formatSubscriptionQuotaAmount(Number(sub.period_amount || 0))}`
                                  : t('Disabled')}
                              </div>
                            </div>
                          </TableCell>
                          <TableCell className='text-right'>
                            <div className='flex justify-end gap-1'>
                              <Button
                                size='sm'
                                variant='outline'
                                disabled={!isActive}
                                onClick={() =>
                                  setConfirmAction({
                                    type: 'reset',
                                    subId: sub.id,
                                  })
                                }
                              >
                                <RotateCcw className='mr-1 h-3.5 w-3.5' />
                                {t('Reset quota')}
                              </Button>
                              <Button
                                size='sm'
                                variant='outline'
                                onClick={() => openEditDialog(sub)}
                              >
                                <Pencil className='mr-1 h-3.5 w-3.5' />
                                {t('Edit')}
                              </Button>
                              <Button
                                size='sm'
                                variant='outline'
                                disabled={!isActive}
                                onClick={() =>
                                  setConfirmAction({
                                    type: 'invalidate',
                                    subId: sub.id,
                                  })
                                }
                              >
                                {t('Invalidate')}
                              </Button>
                              <Button
                                size='sm'
                                variant='destructive'
                                onClick={() =>
                                  setConfirmAction({
                                    type: 'delete',
                                    subId: sub.id,
                                  })
                                }
                              >
                                {t('Delete')}
                              </Button>
                            </div>
                          </TableCell>
                        </TableRow>
                      )
                    })
                  )}
                </TableBody>
              </Table>
            </div>
          </div>
        </SheetContent>
      </Sheet>

      {confirmAction && (
        <ConfirmDialog
          open
          onOpenChange={(open) => !open && setConfirmAction(null)}
          title={
            confirmAction.type === 'invalidate'
              ? t('Confirm invalidate')
              : confirmAction.type === 'reset'
                ? t('Confirm quota reset')
                : t('Confirm delete')
          }
          desc={
            confirmAction.type === 'invalidate'
              ? t(
                  'After invalidating, this subscription will be immediately deactivated. Historical records are not affected. Continue?'
                )
              : confirmAction.type === 'reset'
                ? t(
                    'This will clear the used quota, period used quota, and per-model usage for this subscription, then move the reset cycle forward. Continue?'
                  )
                : t(
                    'Deleting will permanently remove this subscription record (including benefit details). Continue?'
                  )
          }
          handleConfirm={handleConfirmAction}
          destructive={confirmAction.type === 'delete'}
        />
      )}

      <Dialog
        open={!!editingSub}
        onOpenChange={(open) => !open && setEditingSub(null)}
      >
        <DialogContent className='sm:max-w-xl'>
          <DialogHeader>
            <DialogTitle>{t('Edit Subscription')}</DialogTitle>
          </DialogHeader>
          <div className='grid gap-4 sm:grid-cols-2'>
            <div className='space-y-2'>
              <label className='text-sm font-medium'>{t('Start Time')}</label>
              <Input
                type='datetime-local'
                value={editValues.start_time}
                onChange={(event) =>
                  setEditValues((prev) => ({
                    ...prev,
                    start_time: event.target.value,
                  }))
                }
              />
            </div>
            <div className='space-y-2'>
              <label className='text-sm font-medium'>{t('End Time')}</label>
              <Input
                type='datetime-local'
                value={editValues.end_time}
                onChange={(event) =>
                  setEditValues((prev) => ({
                    ...prev,
                    end_time: event.target.value,
                  }))
                }
              />
            </div>
            <div className='space-y-2'>
              <label className='text-sm font-medium'>{t('Status')}</label>
              <Select
                items={[
                  { value: 'active', label: t('Active') },
                  { value: 'expired', label: t('Expired') },
                  { value: 'cancelled', label: t('Invalidated') },
                ]}
                value={editValues.status}
                onValueChange={(value) =>
                  value !== null &&
                  setEditValues((prev) => ({ ...prev, status: value }))
                }
              >
                <SelectTrigger>
                  <SelectValue />
                </SelectTrigger>
                <SelectContent alignItemWithTrigger={false}>
                  <SelectGroup>
                    <SelectItem value='active'>{t('Active')}</SelectItem>
                    <SelectItem value='expired'>{t('Expired')}</SelectItem>
                    <SelectItem value='cancelled'>
                      {t('Invalidated')}
                    </SelectItem>
                  </SelectGroup>
                </SelectContent>
              </Select>
            </div>
            <div className='space-y-2'>
              <label className='text-sm font-medium'>
                {t('Total Quota')} (USD)
              </label>
              <Input
                type='number'
                value={editValues.amount_total}
                onChange={(event) =>
                  setEditValues((prev) => ({
                    ...prev,
                    amount_total: event.target.value,
                  }))
                }
              />
            </div>
            <div className='space-y-2'>
              <label className='text-sm font-medium'>
                {t('Used Quota')} (USD)
              </label>
              <Input
                type='number'
                value={editValues.amount_used}
                onChange={(event) =>
                  setEditValues((prev) => ({
                    ...prev,
                    amount_used: event.target.value,
                  }))
                }
              />
            </div>
            <div className='space-y-2'>
              <label className='text-sm font-medium'>
                {t('Period Quota')} (USD)
              </label>
              <Input
                type='number'
                value={editValues.period_amount}
                onChange={(event) =>
                  setEditValues((prev) => ({
                    ...prev,
                    period_amount: event.target.value,
                  }))
                }
              />
            </div>
            <div className='space-y-2'>
              <label className='text-sm font-medium'>
                {t('Period Used')} (USD)
              </label>
              <Input
                type='number'
                value={editValues.period_used}
                onChange={(event) =>
                  setEditValues((prev) => ({
                    ...prev,
                    period_used: event.target.value,
                  }))
                }
              />
            </div>
            <div className='space-y-2 sm:col-span-2'>
              <label className='text-sm font-medium'>
                {t('Model Limits JSON')}
              </label>
              <Textarea
                rows={5}
                value={editValues.model_limits}
                onChange={(event) =>
                  setEditValues((prev) => ({
                    ...prev,
                    model_limits: event.target.value,
                  }))
                }
                placeholder='{"gpt-4.1":300,"codex-mini-latest":100}'
              />
            </div>
          </div>
          <DialogFooter>
            <Button variant='outline' onClick={() => setEditingSub(null)}>
              {t('Cancel')}
            </Button>
            <Button onClick={handleSaveEdit}>{t('Save changes')}</Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>
    </>
  )
}
