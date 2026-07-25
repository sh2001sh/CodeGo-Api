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
import type { FundingSource } from './types'

type Translate = (key: string, options?: Record<string, unknown>) => string

export type BillingPreference =
  | 'subscription_first'
  | 'wallet_first'
  | 'subscription_only'
  | 'wallet_only'

export function normalizeBillingPreference(value?: string): BillingPreference {
  switch (value) {
    case 'wallet_first':
    case 'subscription_only':
    case 'wallet_only':
    case 'subscription_first':
      return value
    default:
      return 'subscription_first'
  }
}

export function getDefaultFundingSourceOrder(
  preference?: string
): FundingSource[] {
  switch (normalizeBillingPreference(preference)) {
    case 'wallet_first':
      return ['wallet', 'subscription']
    case 'subscription_only':
      return ['subscription']
    case 'wallet_only':
      return ['wallet']
    case 'subscription_first':
    default:
      return ['subscription', 'wallet']
  }
}

export function normalizeFundingSourceOrder(
  order?: string[] | null,
  preference?: string
): FundingSource[] {
  const fallback = getDefaultFundingSourceOrder(preference)
  if (!order?.length) {
    return [...fallback]
  }

  const validSources = new Set<FundingSource>(['subscription', 'wallet'])
  const result: FundingSource[] = []
  for (const item of order) {
    if (!validSources.has(item as FundingSource)) {
      continue
    }
    const source = item as FundingSource
    if (!result.includes(source)) {
      result.push(source)
    }
  }
  if (!result.length) {
    return [...fallback]
  }
  if (!result.some((item) => item === 'subscription' || item === 'wallet')) {
    return [...fallback]
  }
  return result
}

export function getBillingPreferenceFromFundingSourceOrder(
  order: FundingSource[]
): BillingPreference {
  const subscriptionIndex = order.indexOf('subscription')
  const walletIndex = order.indexOf('wallet')

  if (subscriptionIndex >= 0 && walletIndex >= 0) {
    return subscriptionIndex < walletIndex
      ? 'subscription_first'
      : 'wallet_first'
  }
  if (subscriptionIndex >= 0) {
    return 'subscription_only'
  }
  if (walletIndex >= 0) {
    return 'wallet_only'
  }
  return 'subscription_first'
}

export function getFundingSourceLabel(
  source: FundingSource,
  t: Translate
): string {
  switch (source) {
    case 'subscription':
      return t('Subscription quota')
    case 'wallet':
      return t('Wallet balance')
    default:
      return source
  }
}

export function getFundingSourceDescription(
  source: FundingSource,
  t: Translate
): string {
  switch (source) {
    case 'subscription':
      return t('Consume plan quota in subscription order')
    case 'wallet':
      return t('Currently available wallet balance')
    default:
      return ''
  }
}
