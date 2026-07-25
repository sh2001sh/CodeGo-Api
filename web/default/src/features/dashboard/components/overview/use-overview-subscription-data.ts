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
import { useMemo } from 'react'
import { useQuery } from '@tanstack/react-query'
import {
  getPublicPlans,
  getSelfSubscriptionFull,
} from '@/features/subscriptions/api'
import { EMPTY_SUBSCRIPTIONS } from '@/features/subscriptions/lib'
import type {
  PlanRecord,
  SelfSubscriptionData,
} from '@/features/subscriptions/types'

export function useOverviewSubscriptionData() {
  const subscriptionsQuery = useQuery({
    queryKey: ['dashboard', 'overview', 'subscriptions'],
    queryFn: async (): Promise<SelfSubscriptionData> => {
      const result = await getSelfSubscriptionFull()
      return result.success
        ? (result.data ?? EMPTY_SUBSCRIPTIONS)
        : EMPTY_SUBSCRIPTIONS
    },
    staleTime: 60 * 1000,
  })

  const plansQuery = useQuery({
    queryKey: ['dashboard', 'overview', 'subscription-plans'],
    queryFn: async (): Promise<PlanRecord[]> => {
      const result = await getPublicPlans()
      return result.success ? (result.data ?? []) : []
    },
    staleTime: 5 * 60 * 1000,
  })

  const isLoading = subscriptionsQuery.isLoading || plansQuery.isLoading
  const isFetching = subscriptionsQuery.isFetching || plansQuery.isFetching

  return useMemo(
    () => ({
      subscriptionData: subscriptionsQuery.data ?? EMPTY_SUBSCRIPTIONS,
      plans: plansQuery.data ?? [],
      isLoading,
      isFetching,
      refetchSubscriptions: subscriptionsQuery.refetch,
      refetchPlans: plansQuery.refetch,
    }),
    [
      isFetching,
      isLoading,
      plansQuery.data,
      plansQuery.refetch,
      subscriptionsQuery.data,
      subscriptionsQuery.refetch,
    ]
  )
}
