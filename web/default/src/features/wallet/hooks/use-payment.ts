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
import { useCallback, useState } from 'react'
import i18next from 'i18next'
import { toast } from 'sonner'
import {
  calculateStripeAmount,
  isApiSuccess,
  requestStripePayment,
} from '../api'

export function usePayment() {
  const [amount, setAmount] = useState(0)
  const [calculating, setCalculating] = useState(false)
  const [processing, setProcessing] = useState(false)

  const calculatePaymentAmount = useCallback(async (topupAmount: number) => {
    try {
      setCalculating(true)
      const response = await calculateStripeAmount({ amount: topupAmount })
      if (isApiSuccess(response) && response.data) {
        const calculatedAmount = Number.parseFloat(response.data)
        setAmount(calculatedAmount)
        return calculatedAmount
      }
    } catch {
      // The form keeps the amount editable when the quote endpoint is unavailable.
    } finally {
      setCalculating(false)
    }
    setAmount(0)
    return 0
  }, [])

  const processPayment = useCallback(async (topupAmount: number) => {
    try {
      setProcessing(true)
      const response = await requestStripePayment({
        amount: Math.floor(topupAmount),
        payment_method: 'stripe',
      })
      if (!isApiSuccess(response) || !response.data?.pay_link) {
        toast.error(response.message || i18next.t('Payment request failed'))
        return false
      }
      window.open(response.data.pay_link, '_blank')
      toast.success(i18next.t('Redirecting to payment page...'))
      return true
    } catch {
      toast.error(i18next.t('Payment request failed'))
      return false
    } finally {
      setProcessing(false)
    }
  }, [])

  return {
    amount,
    calculating,
    processing,
    calculatePaymentAmount,
    processPayment,
    setAmount,
  }
}
