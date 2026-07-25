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
import {
  formatSubscriptionPlanTitle,
  getSubscriptionPlanSubtitle,
} from '@/features/subscriptions/lib'
import type { SubscriptionPlan } from '@/features/subscriptions/types'

export function translatePlanTitle(
  value: string | null | undefined,
  t: TFunction
) {
  const normalized = formatSubscriptionPlanTitle(value)
  if (!normalized) return t('Plan')
  return normalized
}

export function translatePlanSubtitle(
  plan: Partial<SubscriptionPlan> | null | undefined,
  t: TFunction
) {
  const subtitle = getSubscriptionPlanSubtitle(plan)
  switch (subtitle) {
    case 'Starter':
      return t('Starter plans')
    case 'Monthly':
      return t('Monthly plan')
    case 'Weekly':
      return t('Weekly plan')
    case 'Daily':
      return t('Day pass')
    default:
      return subtitle
  }
}

export function translatePlanAction(action: string | undefined, t: TFunction) {
  switch (action) {
    case 'renew':
      return t('Renew now')
    case 'upgrade':
      return t('Upgrade now')
    case 'disabled':
      return t('Unavailable for subscription')
    case 'subscribe':
      return t('Subscribe now')
    default:
      return t('Subscribe')
  }
}

export function translateDisabledReason(
  value: string | null | undefined,
  t: TFunction
) {
  const normalized = String(value || '').trim()
  if (!normalized) return ''
  if (
    normalized === '当前还有更高档且未用完的生效套餐，暂不支持直接降级。' ||
    normalized.includes('cannot subscribe to a lower-tier plan')
  ) {
    return t('A higher active plan with remaining quota prevents downgrading.')
  }
  if (
    normalized.includes(
      'renewal requires at least 30% of the current package quota to be used'
    )
  ) {
    return t(
      'Renewal is available after at least 30% of the current package quota is used.'
    )
  }
  return normalized
}
