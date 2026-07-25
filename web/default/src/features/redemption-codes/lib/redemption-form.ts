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
import type { TFunction } from 'i18next'
import { parseQuotaFromDollars, quotaUnitsToDollars } from '@/lib/format'
import { REDEMPTION_TYPES, REDEMPTION_VALIDATION } from '../constants'
import {
  type Redemption,
  type RedemptionFormData,
  type RedemptionType,
} from '../types'

export function getRedemptionFormSchema(t: TFunction) {
  return z
    .object({
      name: z.string().max(
        REDEMPTION_VALIDATION.NAME_MAX_LENGTH,
        t('Name must be between {{min}} and {{max}} characters', {
          min: REDEMPTION_VALIDATION.NAME_MIN_LENGTH,
          max: REDEMPTION_VALIDATION.NAME_MAX_LENGTH,
        })
      ),
      redeem_type: z.enum([
        REDEMPTION_TYPES.QUOTA,
        REDEMPTION_TYPES.SUBSCRIPTION,
      ]),
      quota_dollars: z.number().min(0),
      plan_id: z.number().int().min(0).optional(),
      expired_time: z.date().optional(),
      count: z
        .number()
        .min(
          REDEMPTION_VALIDATION.COUNT_MIN,
          t('Count must be between {{min}} and {{max}}', {
            min: REDEMPTION_VALIDATION.COUNT_MIN,
            max: REDEMPTION_VALIDATION.COUNT_MAX,
          })
        )
        .max(
          REDEMPTION_VALIDATION.COUNT_MAX,
          t('Count must be between {{min}} and {{max}}', {
            min: REDEMPTION_VALIDATION.COUNT_MIN,
            max: REDEMPTION_VALIDATION.COUNT_MAX,
          })
        )
        .optional(),
    })
    .superRefine((data, ctx) => {
      if (
        data.redeem_type === REDEMPTION_TYPES.QUOTA &&
        data.quota_dollars <= 0
      ) {
        ctx.addIssue({
          code: z.ZodIssueCode.custom,
          message: t('Quota must be a positive number'),
          path: ['quota_dollars'],
        })
      }
      if (
        data.redeem_type === REDEMPTION_TYPES.SUBSCRIPTION &&
        (!data.plan_id || data.plan_id <= 0)
      ) {
        ctx.addIssue({
          code: z.ZodIssueCode.custom,
          message: t('Please select a subscription plan'),
          path: ['plan_id'],
        })
      }
    })
}

export type RedemptionFormValues = {
  name: string
  redeem_type: RedemptionType
  quota_dollars: number
  plan_id?: number
  expired_time?: Date
  count?: number
}

export const REDEMPTION_FORM_DEFAULT_VALUES: RedemptionFormValues = {
  name: '',
  redeem_type: REDEMPTION_TYPES.QUOTA,
  quota_dollars: 10,
  plan_id: undefined,
  expired_time: undefined,
  count: 1,
}

export function transformFormDataToPayload(
  data: RedemptionFormValues
): RedemptionFormData {
  const isSubscription = data.redeem_type === REDEMPTION_TYPES.SUBSCRIPTION
  return {
    name: data.name,
    redeem_type: data.redeem_type,
    quota: isSubscription ? 0 : parseQuotaFromDollars(data.quota_dollars),
    plan_id: isSubscription ? Number(data.plan_id || 0) : 0,
    expired_time: data.expired_time
      ? Math.floor(data.expired_time.getTime() / 1000)
      : 0,
    count: data.count || 1,
  }
}

export function transformRedemptionToFormDefaults(
  redemption: Redemption
): RedemptionFormValues {
  return {
    name: redemption.name,
    redeem_type:
      redemption.redeem_type === REDEMPTION_TYPES.SUBSCRIPTION
        ? REDEMPTION_TYPES.SUBSCRIPTION
        : REDEMPTION_TYPES.QUOTA,
    quota_dollars:
      redemption.redeem_type === REDEMPTION_TYPES.SUBSCRIPTION
        ? 0
        : quotaUnitsToDollars(redemption.quota),
    plan_id: redemption.plan_id > 0 ? redemption.plan_id : undefined,
    expired_time:
      redemption.expired_time > 0
        ? new Date(redemption.expired_time * 1000)
        : undefined,
    count: 1,
  }
}
