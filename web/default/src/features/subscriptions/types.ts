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
import { z } from 'zod'

// ============================================================================
// Subscription Plan Schema & Types
// ============================================================================

export const subscriptionPlanSchema = z.object({
  id: z.number(),
  title: z.string(),
  subtitle: z.string().optional(),
  price_amount: z.number(),
  currency: z.string().default('USD'),
  duration_unit: z.enum(['year', 'month', 'day', 'hour', 'custom']),
  duration_value: z.number(),
  custom_seconds: z.number().optional(),
  quota_reset_period: z.enum(['never', 'daily', 'weekly', 'monthly', 'custom']),
  quota_reset_custom_seconds: z.number().optional(),
  enabled: z.boolean(),
  internal_only: z.boolean().default(false),
  sort_order: z.number(),
  max_purchase_per_user: z.number(),
  plan_type: z
    .enum(['monthly', 'weekly', 'daily', 'starter'])
    .default('monthly')
    .optional(),
  fuel_enabled: z.boolean().default(false).optional(),
  fuel_unit_price: z.number().default(0).optional(),
  fuel_min_quota: z.number().default(0).optional(),
  fuel_quota_step: z.number().default(0).optional(),
  total_amount: z.number(),
  period_amount: z.number().optional(),
  model_limits: z.string().optional(),
  upgrade_group: z.string().optional(),
  stripe_price_id: z.string().optional(),
  creem_product_id: z.string().optional(),
})

export type SubscriptionPlan = z.infer<typeof subscriptionPlanSchema>

export interface PlanRecord {
  plan: SubscriptionPlan
  action?: 'subscribe' | 'renew' | 'upgrade' | 'disabled'
  base_amount_due?: number
  amount_due?: number
  disabled_reason?: string
}

// ============================================================================
// User Subscription Schema & Types
// ============================================================================

export const userSubscriptionSchema = z.object({
  id: z.number(),
  user_id: z.number(),
  plan_id: z.number(),
  status: z.string(),
  source: z.string().optional(),
  start_time: z.number(),
  end_time: z.number(),
  amount_total: z.number(),
  amount_used: z.number(),
  period_amount: z.number().optional(),
  period_used: z.number().optional(),
  model_limits: z.string().optional(),
  model_usage: z.string().optional(),
  next_reset_time: z.number().optional(),
})

export type UserSubscription = z.infer<typeof userSubscriptionSchema>

export interface UserSubscriptionRecord {
  subscription: UserSubscription
}

// ============================================================================
// API Request/Response Types
// ============================================================================

export interface ApiResponse<T = unknown> {
  success: boolean
  message?: string
  data?: T
}

export interface PlanPayload {
  plan: Partial<SubscriptionPlan>
}

export interface SubscriptionPayRequest {
  plan_id: number
  purchase_type?: 'normal'
}

export interface SubscriptionPayResponse {
  success: boolean
  message?: string
  data?: {
    pay_link?: string
    checkout_url?: string
    pay_url?: string
    qrcode_url?: string
    order_id?: string
    amount_due?: number
    action?: 'subscribe' | 'renew' | 'upgrade' | 'disabled'
    form?: Record<string, unknown>
  }
  url?: string
}

export interface SubscriptionFuelQuote {
  subscription_id: number
  plan_id: number
  plan_title: string
  quota: number
  unit_price: number
  amount_due: number
  expires_at: number
  min_quota: number
  quota_step: number
  quota_per_unit: number
}

export interface SubscriptionOrderStatus {
  trade_no: string
  status: 'pending' | 'success' | 'expired' | string
  plan_id: number
  plan_title?: string
  money: number
  payment_method?: string
  payment_provider?: string
  create_time?: number
  complete_time?: number
}

export interface CreateUserSubscriptionRequest {
  plan_id: number
}

export interface UpdateUserSubscriptionRequest {
  start_time: number
  end_time: number
  status: string
  amount_total: number
  amount_used: number
  period_amount: number
  period_used: number
  model_limits: string
}

export interface ResetUserSubscriptionQuotaRequest {
  advance_reset_time?: boolean
}

// ============================================================================
// Self Subscription Data (user-facing)
// ============================================================================

export interface SelfSubscriptionData {
  billing_preference: string
  funding_source_order: FundingSource[]
  subscription_order_ids: number[]
  subscriptions: UserSubscriptionRecord[]
  all_subscriptions: UserSubscriptionRecord[]
}

export type FundingSource = 'subscription' | 'wallet'

// ============================================================================
// Dialog Types
// ============================================================================

export type SubscriptionsDialogType =
  | 'create'
  | 'update'
  | 'toggle-status'
  | 'delete'
