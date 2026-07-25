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
import { useAuthStore } from '@/stores/auth-store'
import { getSelf } from '@/lib/api'
import { useStatus } from '@/hooks/use-status'
import { useSystemConfig } from '@/hooks/use-system-config'
import {
  getPublicPlans,
  getSelfSubscriptionFull,
} from '@/features/subscriptions/api'
import type {
  PlanRecord,
  SelfSubscriptionData,
} from '@/features/subscriptions/types'
import { getMinTopupAmount } from '../lib'
import type {
  CreemProduct,
  PaymentMethod,
  PresetAmount,
  UserWalletData,
} from '../types'
import { useCreemPayment } from './use-creem-payment'
import { usePayment } from './use-payment'
import { useRedemption } from './use-redemption'
import { useTopupInfo } from './use-topup-info'

const STRIPE_METHOD: PaymentMethod = { name: 'Stripe', type: 'stripe' }

export function useWalletWorkspace() {
  const [user, setUser] = useState<UserWalletData | null>(null)
  const [userLoading, setUserLoading] = useState(true)
  const [subscriptionData, setSubscriptionData] =
    useState<SelfSubscriptionData | null>(null)
  const [subscriptionLoading, setSubscriptionLoading] = useState(true)
  const [publicPlans, setPublicPlans] = useState<PlanRecord[]>([])
  const [publicPlansLoading, setPublicPlansLoading] = useState(true)
  const [topupAmount, setTopupAmount] = useState(0)
  const [selectedPreset, setSelectedPreset] = useState<number | null>(null)
  const [paymentLoading, setPaymentLoading] = useState(false)
  const [confirmDialogOpen, setConfirmDialogOpen] = useState(false)
  const [billingDialogOpen, setBillingDialogOpen] = useState(false)
  const [redemptionCode, setRedemptionCode] = useState('')
  const [creemDialogOpen, setCreemDialogOpen] = useState(false)
  const [selectedCreemProduct, setSelectedCreemProduct] =
    useState<CreemProduct | null>(null)
  const setAuthUser = useAuthStore((state) => state.auth.setUser)
  const { status } = useStatus()
  const { currency } = useSystemConfig()
  const { topupInfo, presetAmounts, loading: topupLoading } = useTopupInfo()
  const {
    amount: paymentAmount,
    calculating,
    processing,
    calculatePaymentAmount,
    processPayment,
  } = usePayment()
  const { redeeming, redeemCode } = useRedemption()
  const { processing: creemProcessing, processCreemPayment } = useCreemPayment()

  const effectiveUsdExchangeRate = useMemo(
    () =>
      currency?.quotaDisplayType === 'USD' ? 1 : currency?.usdExchangeRate || 1,
    [currency?.quotaDisplayType, currency?.usdExchangeRate]
  )

  const fetchUser = useCallback(async () => {
    try {
      setUserLoading(true)
      const response = await getSelf()
      if (response.success && response.data) {
        setUser(response.data as UserWalletData)
        setAuthUser(response.data)
      }
    } finally {
      setUserLoading(false)
    }
  }, [setAuthUser])

  const fetchSubscriptionData = useCallback(async () => {
    try {
      setSubscriptionLoading(true)
      const response = await getSelfSubscriptionFull()
      setSubscriptionData(response.success ? response.data || null : null)
    } finally {
      setSubscriptionLoading(false)
    }
  }, [])

  const fetchPublicPlans = useCallback(async () => {
    try {
      setPublicPlansLoading(true)
      const response = await getPublicPlans()
      setPublicPlans(response.success ? response.data || [] : [])
    } finally {
      setPublicPlansLoading(false)
    }
  }, [])

  useEffect(() => {
    void fetchUser()
    void fetchSubscriptionData()
    void fetchPublicPlans()
  }, [fetchPublicPlans, fetchSubscriptionData, fetchUser])

  useEffect(() => {
    if (topupInfo && topupAmount === 0) {
      const minimum = getMinTopupAmount(topupInfo)
      setTopupAmount(minimum)
      void calculatePaymentAmount(minimum)
    }
  }, [calculatePaymentAmount, topupAmount, topupInfo])

  const handleSelectPreset = useCallback(
    (preset: PresetAmount) => {
      setTopupAmount(preset.value)
      setSelectedPreset(preset.value)
      void calculatePaymentAmount(preset.value)
    },
    [calculatePaymentAmount]
  )

  const handleTopupAmountChange = useCallback(
    (amount: number) => {
      setTopupAmount(amount)
      setSelectedPreset(null)
      void calculatePaymentAmount(amount)
    },
    [calculatePaymentAmount]
  )

  const handlePaymentMethodSelect = useCallback(async () => {
    setPaymentLoading(true)
    try {
      await calculatePaymentAmount(topupAmount)
      setConfirmDialogOpen(true)
    } finally {
      setPaymentLoading(false)
    }
  }, [calculatePaymentAmount, topupAmount])

  const handlePaymentConfirm = useCallback(async () => {
    const success = await processPayment(topupAmount)
    if (success) {
      setConfirmDialogOpen(false)
      await fetchUser()
    }
  }, [fetchUser, processPayment, topupAmount])

  const handleRedeem = useCallback(async () => {
    if (!redemptionCode) return
    const success = await redeemCode(redemptionCode)
    if (success) {
      setRedemptionCode('')
      await fetchUser()
    }
  }, [fetchUser, redeemCode, redemptionCode])

  const handleCreemProductSelect = useCallback((product: CreemProduct) => {
    setSelectedCreemProduct(product)
    setCreemDialogOpen(true)
  }, [])

  const handleCreemConfirm = useCallback(async () => {
    if (!selectedCreemProduct) return
    const success = await processCreemPayment(selectedCreemProduct.productId)
    if (success) {
      setCreemDialogOpen(false)
      setSelectedCreemProduct(null)
      await fetchUser()
    }
  }, [fetchUser, processCreemPayment, selectedCreemProduct])

  const getDiscountRate = useCallback(
    () => topupInfo?.discount?.[topupAmount] || 1,
    [topupAmount, topupInfo]
  )

  return {
    user,
    userLoading,
    subscriptionData,
    subscriptionLoading,
    publicPlans,
    publicPlansLoading,
    topupInfo,
    presetAmounts,
    topupLoading,
    topupAmount,
    selectedPreset,
    selectedPaymentMethod: STRIPE_METHOD,
    paymentAmount,
    calculating,
    paymentLoading,
    redemptionCode,
    redeeming,
    status,
    effectiveUsdExchangeRate,
    confirmDialogOpen,
    billingDialogOpen,
    creemDialogOpen,
    selectedCreemProduct,
    processing,
    creemProcessing,
    fetchUser,
    fetchSubscriptionData,
    fetchPublicPlans,
    handleSelectPreset,
    handleTopupAmountChange,
    handlePaymentMethodSelect,
    handlePaymentConfirm,
    handleRedeem,
    handleCreemProductSelect,
    handleCreemConfirm,
    getDiscountRate,
    setConfirmDialogOpen,
    setBillingDialogOpen,
    setCreemDialogOpen,
    setRedemptionCode,
  }
}
