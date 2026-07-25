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
import { getCurrencyDisplay } from '@/lib/currency'
import type { SubscriptionPlan } from '../types'
import { formatDuration, formatResetPeriod } from './format'

const DAY_PASS_KEYWORDS = ['day pass', '日卡']
const DEFAULT_QUOTA_PER_UNIT = 500000

function trimText(value?: string | null): string {
  return String(value || '')
    .replaceAll(String.fromCharCode(0), '')
    .trim()
}

function formatNumber(value: number): string {
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

function getSubscriptionQuotaPerUnit(): number {
  const quotaPerUnit = Number(getCurrencyDisplay().config.quotaPerUnit || 0)
  return quotaPerUnit > 0 ? quotaPerUnit : DEFAULT_QUOTA_PER_UNIT
}

export function subscriptionQuotaUnitsToUSD(amount?: number | null): number {
  return Number(amount || 0) / getSubscriptionQuotaPerUnit()
}

export function parseSubscriptionQuotaUSDToUnits(
  amount?: number | string | null
): number {
  const numericAmount = Number(amount || 0)
  if (!Number.isFinite(numericAmount)) return 0
  return Math.round(numericAmount * getSubscriptionQuotaPerUnit())
}

export function normalizeSubscriptionText(value?: string | null): string {
  return trimText(value)
}

export function formatSubscriptionPlanTitle(value?: string | null): string {
  return normalizeSubscriptionText(value).replace(
    /([A-Za-z])(?=[\u4e00-\u9fff])/g,
    '$1 '
  )
}

export function getSubscriptionDisabledReasonText(
  value?: string | null
): string {
  const normalized = normalizeSubscriptionText(value)
  switch (normalized) {
    case 'cannot subscribe to a lower-tier plan while your current package is active':
    case 'cannot subscribe to a lower-tier plan while your current package still has remaining quota':
      return '当前还有更高档且未用完的生效套餐，暂不支持直接降级。'
    case 'renewal requires at least 30% of the current package quota to be used':
      return '当前套餐至少使用 30%（剩余额度不高于 70%）后才可续费。'
    default:
      return normalized
  }
}

export function isDayPassPlan(
  plan?: Partial<SubscriptionPlan> | null
): boolean {
  if (!plan) return false
  if (plan.plan_type === 'daily') return true
  if (plan.duration_unit === 'day' && Number(plan.duration_value || 0) <= 2) {
    return true
  }
  const title = normalizeSubscriptionText(plan.title).toLowerCase()
  return DAY_PASS_KEYWORDS.some((keyword) => title.includes(keyword))
}

export function isMonthlyCardPlan(
  plan?: Partial<SubscriptionPlan> | null
): boolean {
  if (!plan || isDayPassPlan(plan)) return false
  if (plan.plan_type === 'monthly') return true
  return plan.duration_unit === 'month'
}

export function getSubscriptionCurrencyLabel(currency?: string | null): string {
  const normalized = trimText(currency).toUpperCase()
  switch (normalized) {
    case 'CNY':
    case 'RMB':
      return '¥'
    case 'EUR':
      return 'EUR '
    case 'USD':
    default:
      return '$'
  }
}

export function formatSubscriptionPlanPrice(
  amount?: number | null,
  currency?: string | null
): string {
  const value = Number(amount || 0)
  return `${getSubscriptionCurrencyLabel(currency)}${formatNumber(value)}`
}

export function formatSubscriptionQuotaAmount(amount?: number | null): string {
  const usdAmount = subscriptionQuotaUnitsToUSD(amount)
  return `$${formatNumber(usdAmount)}`
}

export function getSubscriptionPlanSubtitle(
  plan?: Partial<SubscriptionPlan> | null
): string {
  const subtitle = normalizeSubscriptionText(plan?.subtitle)
  if (subtitle) return subtitle
  if (plan?.plan_type === 'starter') return 'Starter'
  if (plan?.plan_type === 'weekly') return 'Weekly'
  return isDayPassPlan(plan) ? 'Daily' : 'Monthly'
}

export function getSubscriptionPlanActionLabel(
  action: string | undefined,
  t: TFunction
): string {
  switch (action) {
    case 'renew':
      return '立即续费'
    case 'upgrade':
      return '立即升级'
    case 'disabled':
      return '当前不可订阅'
    case 'subscribe':
      return '立即订阅'
    default:
      return t('Subscribe')
  }
}

export function getSubscriptionPlanDescription(
  plan: Partial<SubscriptionPlan>,
  totalAmount: number,
  periodAmount: number,
  t: TFunction
): string {
  if (isDayPassPlan(plan)) {
    const totalText =
      totalAmount > 0 ? formatSubscriptionQuotaAmount(totalAmount) : '不限'
    return `有效期 ${formatDuration(plan, t)}，总额度 ${totalText}，日卡额度独立结算，扣费时默认优先于月卡。`
  }

  if (isMonthlyCardPlan(plan)) {
    const totalText =
      totalAmount > 0 ? formatSubscriptionQuotaAmount(totalAmount) : '不限'
    return `有效期 ${formatDuration(plan, t)}，本月可用额度 ${totalText}，一个月内可自由使用，用完或到期后结束。`
  }

  if (periodAmount > 0) {
    const totalText =
      totalAmount > 0 ? formatSubscriptionQuotaAmount(totalAmount) : '不限'
    return `额度按周期刷新，周期额度 ${formatSubscriptionQuotaAmount(periodAmount)}，总额度 ${totalText}。`
  }

  if (totalAmount > 0) {
    return `有效期 ${formatDuration(plan, t)}，总额度 ${formatSubscriptionQuotaAmount(totalAmount)}。`
  }

  return `有效期 ${formatDuration(plan, t)}，总额度不限。`
}

export function getSubscriptionPlanDetailText(
  plan: Partial<SubscriptionPlan>,
  totalAmount: number,
  periodAmount: number,
  t: TFunction
): string {
  const detailParts = [`有效期 ${formatDuration(plan, t)}`]
  const resetLabel = formatResetPeriod(plan, t)

  if (isDayPassPlan(plan)) {
    detailParts.push(
      totalAmount > 0
        ? `总额度 ${formatSubscriptionQuotaAmount(totalAmount)}`
        : '总额度不限'
    )
    detailParts.push('日卡额度独立结算，扣费时默认优先于月卡')
    return detailParts.join('；')
  }

  if (isMonthlyCardPlan(plan)) {
    if (totalAmount > 0) {
      detailParts.push(
        `本月可用额度 ${formatSubscriptionQuotaAmount(totalAmount)}`
      )
    } else {
      detailParts.push('本月可用额度不限')
    }
    detailParts.push('月卡不设置周刷新或周期额度')
    detailParts.push('适合持续使用 codego-api 与相关模型')
    return detailParts.join('；')
  }

  if (resetLabel !== t('No Reset')) {
    detailParts.push(`额度重置 ${resetLabel}`)
  }
  if (periodAmount > 0) {
    detailParts.push(
      `${isMonthlyCardPlan(plan) ? '月度额度' : '周期额度'} ${formatSubscriptionQuotaAmount(periodAmount)}`
    )
  }
  if (totalAmount > 0) {
    detailParts.push(`总额度 ${formatSubscriptionQuotaAmount(totalAmount)}`)
  } else {
    detailParts.push('总额度不限')
  }
  detailParts.push('适合持续使用 codego-api 与相关模型')
  return detailParts.join('；')
}
