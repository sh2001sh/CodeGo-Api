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
export interface ApiResponse<T = unknown> {
  success?: boolean
  message?: string
  data?: T
}

export interface RedemptionResult {
  redeem_type: 'quota' | 'subscription' | string
  quota?: number
  plan_id?: number
  plan_title?: string
  user_subscription_id?: number
}

export type TopupInfoResponse = ApiResponse<TopupInfo>
export type RedemptionResponse = ApiResponse<RedemptionResult>
export type AmountResponse = ApiResponse<string>
export type PaymentResponse = ApiResponse<Record<string, unknown>> & {
  url?: string
}
export type StripePaymentResponse = ApiResponse<{ pay_link: string }>
export type CreemPaymentResponse = ApiResponse<{ checkout_url: string }>

export interface CreemProduct {
  name: string
  productId: string
  price: number
  quota: number
  currency: 'USD' | 'EUR'
}

export interface CreemPaymentRequest {
  product_id: string
  payment_method: 'creem'
}

export interface PaymentMethod {
  name: string
  type: string
  color?: string
  min_topup?: number
  icon?: string
}

export interface TopupInfo {
  enable_stripe_topup: boolean
  enable_creem_topup: boolean
  min_topup: number
  stripe_min_topup: number
  amount_options: number[]
  discount: Record<number, number>
  topup_link?: string
  creem_products?: CreemProduct[]
  enable_redemption?: boolean
  payment_compliance_confirmed?: boolean
  payment_compliance_terms_version?: string
}

export interface PresetAmount {
  value: number
  discount?: number
}

export interface RedemptionRequest {
  key: string
}

export interface PaymentRequest {
  amount: number
  payment_method: string
}

export interface AmountRequest {
  amount: number
}

export interface UserWalletData {
  id: number
  username: string
  quota: number
  used_quota: number
  request_count: number
  group: string
}

export type TopupStatus = 'success' | 'pending' | 'expired' | 'failed'

export interface TopupRecord {
  id: number
  user_id: number
  amount: number
  money: number
  trade_no: string
  payment_method: string
  create_time: number
  complete_time?: number
  status: TopupStatus
}

export interface BillingHistoryResponse {
  items: TopupRecord[]
  total: number
}

export interface CompleteOrderRequest {
  trade_no: string
}
