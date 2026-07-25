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
import type { TFunction } from 'i18next'
import dayjs from '@/lib/dayjs'
import type { SubscriptionPlan } from '../types'

export function formatDuration(
  plan: Partial<SubscriptionPlan>,
  t: TFunction
): string {
  const unit = plan?.duration_unit || 'month'
  const value = plan?.duration_value || 1
  const unitLabels: Record<string, string> = {
    year: value === 1 ? t('year') : t('years'),
    month: value === 1 ? t('month') : t('months'),
    day: value === 1 ? t('day') : t('days'),
    hour: value === 1 ? t('hour') : t('hours'),
    custom: t('Custom (seconds)'),
  }
  if (unit === 'custom') {
    const seconds = plan?.custom_seconds || 0
    if (seconds >= 86400) {
      const days = Math.floor(seconds / 86400)
      return `${days} ${days === 1 ? t('day') : t('days')}`
    }
    if (seconds >= 3600) {
      const hours = Math.floor(seconds / 3600)
      return `${hours} ${hours === 1 ? t('hour') : t('hours')}`
    }
    return `${seconds} ${seconds === 1 ? t('second') : t('seconds')}`
  }
  return `${value} ${unitLabels[unit] || unit}`
}

export function formatResetPeriod(
  plan: Partial<SubscriptionPlan>,
  t: TFunction
): string {
  const period = plan?.quota_reset_period || 'never'
  if (period === 'daily') return t('Daily')
  if (period === 'weekly') return t('Weekly')
  if (period === 'monthly') return t('Monthly')
  if (period === 'custom') {
    const seconds = Number(plan?.quota_reset_custom_seconds || 0)
    if (seconds >= 86400) {
      const days = Math.floor(seconds / 86400)
      return `${days} ${days === 1 ? t('day') : t('days')}`
    }
    if (seconds >= 3600) {
      const hours = Math.floor(seconds / 3600)
      return `${hours} ${hours === 1 ? t('hour') : t('hours')}`
    }
    if (seconds >= 60) {
      const minutes = Math.floor(seconds / 60)
      return `${minutes} ${minutes === 1 ? t('minute') : t('minutes')}`
    }
    return `${seconds} ${seconds === 1 ? t('second') : t('seconds')}`
  }
  return t('No Reset')
}

export function formatTimestamp(ts: number): string {
  if (!ts) return '-'
  return dayjs(ts * 1000).format('YYYY-MM-DD HH:mm:ss')
}
