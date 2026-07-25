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
import { api } from '@/lib/api'
import type {
  ApiResponse,
  FundingSource,
  PlanRecord,
  PlanPayload,
  UserSubscriptionRecord,
  CreateUserSubscriptionRequest,
  UpdateUserSubscriptionRequest,
  ResetUserSubscriptionQuotaRequest,
  SubscriptionPayResponse,
  SubscriptionPayRequest,
  SelfSubscriptionData,
  SubscriptionOrderStatus,
  SubscriptionFuelQuote,
} from './types'

// ============================================================================
// Admin Plan Management
// ============================================================================

export async function getAdminPlans(): Promise<ApiResponse<PlanRecord[]>> {
  const res = await api.get('/api/subscription/admin/plans')
  return res.data
}

export async function quoteSubscriptionFuel(payload: {
  subscriptionId: number
  quota: number
}): Promise<ApiResponse<SubscriptionFuelQuote>> {
  const res = await api.post('/api/subscription/fuel/quote', {
    subscription_id: payload.subscriptionId,
    quota: payload.quota,
  })
  return res.data
}

export async function purchaseSubscriptionFuel(payload: {
  subscriptionId: number
  quota: number
  paymentMethod: string
}): Promise<SubscriptionPayResponse & { url?: string }> {
  const res = await api.post('/api/subscription/fuel/purchase', {
    subscription_id: payload.subscriptionId,
    quota: payload.quota,
    payment_method: payload.paymentMethod,
  })
  return { ...res.data, url: res.data.url }
}

export async function createPlan(
  data: PlanPayload
): Promise<ApiResponse<PlanRecord>> {
  const res = await api.post('/api/subscription/admin/plans', data)
  return res.data
}

export async function updatePlan(
  id: number,
  data: PlanPayload
): Promise<ApiResponse<PlanRecord>> {
  const res = await api.put(`/api/subscription/admin/plans/${id}`, data)
  return res.data
}

export async function patchPlanStatus(
  id: number,
  enabled: boolean
): Promise<ApiResponse> {
  const res = await api.patch(`/api/subscription/admin/plans/${id}`, {
    enabled,
  })
  return res.data
}

export async function deletePlan(id: number): Promise<ApiResponse> {
  const res = await api.delete(`/api/subscription/admin/plans/${id}`)
  return res.data
}

// ============================================================================
// Admin User Subscription Management
// ============================================================================

export async function getUserSubscriptions(
  userId: number
): Promise<ApiResponse<UserSubscriptionRecord[]>> {
  const res = await api.get(
    `/api/subscription/admin/users/${userId}/subscriptions`
  )
  return res.data
}

export async function createUserSubscription(
  userId: number,
  data: CreateUserSubscriptionRequest
): Promise<ApiResponse<{ message?: string }>> {
  const res = await api.post(
    `/api/subscription/admin/users/${userId}/subscriptions`,
    data
  )
  return res.data
}

export async function invalidateUserSubscription(
  subId: number
): Promise<ApiResponse<{ message?: string }>> {
  const res = await api.post(
    `/api/subscription/admin/user_subscriptions/${subId}/invalidate`
  )
  return res.data
}

export async function updateUserSubscription(
  subId: number,
  data: UpdateUserSubscriptionRequest
): Promise<ApiResponse<{ message?: string }>> {
  const res = await api.put(
    `/api/subscription/admin/user_subscriptions/${subId}`,
    data
  )
  return res.data
}

export async function deleteUserSubscription(
  subId: number
): Promise<ApiResponse> {
  const res = await api.delete(
    `/api/subscription/admin/user_subscriptions/${subId}`
  )
  return res.data
}

export async function resetUserSubscriptionQuota(
  subId: number,
  data: ResetUserSubscriptionQuotaRequest = {}
): Promise<ApiResponse<{ message?: string; next_reset_time?: number }>> {
  const res = await api.post(
    `/api/subscription/admin/user_subscriptions/${subId}/reset`,
    data
  )
  return res.data
}

// ============================================================================
// User-facing Subscription Payment
// ============================================================================

export async function paySubscriptionStripe(
  data: SubscriptionPayRequest
): Promise<SubscriptionPayResponse> {
  const res = await api.post('/api/subscription/stripe/pay', data)
  return res.data
}

export async function paySubscriptionCreem(
  data: SubscriptionPayRequest
): Promise<SubscriptionPayResponse> {
  const res = await api.post('/api/subscription/creem/pay', data)
  return res.data
}

export async function getSubscriptionOrderStatus(
  tradeNo: string
): Promise<ApiResponse<SubscriptionOrderStatus>> {
  const res = await api.get(`/api/subscription/orders/${tradeNo}`)
  return res.data
}

export async function cancelSubscriptionOrder(
  tradeNo: string
): Promise<ApiResponse> {
  const res = await api.post(`/api/subscription/orders/${tradeNo}/cancel`)
  return res.data
}

// ============================================================================
// User Self Subscriptions
// ============================================================================

export async function getSelfSubscriptions(): Promise<
  ApiResponse<UserSubscriptionRecord[]>
> {
  const res = await api.get('/api/subscription/self')
  return res.data
}

export async function getSelfSubscriptionFull(): Promise<
  ApiResponse<SelfSubscriptionData>
> {
  const res = await api.get('/api/subscription/self')
  return res.data
}

export async function getPublicPlans(): Promise<ApiResponse<PlanRecord[]>> {
  const res = await api.get('/api/packages/public')
  return res.data
}

export async function updateBillingPreference(payload: {
  billingPreference?: string
  fundingSourceOrder?: FundingSource[]
  subscriptionOrderIds?: number[]
}): Promise<ApiResponse> {
  const res = await api.put('/api/subscription/self/preference', {
    billing_preference: payload.billingPreference,
    funding_source_order: payload.fundingSourceOrder,
    subscription_order_ids: payload.subscriptionOrderIds,
  })
  return res.data
}

export async function getGroups(): Promise<ApiResponse<string[]>> {
  const res = await api.get('/api/group')
  return res.data
}
