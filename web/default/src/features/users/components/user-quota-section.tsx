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
import type { UseFormReturn } from 'react-hook-form'
import type { TFunction } from 'i18next'
import { Pencil } from 'lucide-react'
import { formatQuota, parseQuotaFromDollars } from '@/lib/format'
import { Button } from '@/components/ui/button'
import {
  FormControl,
  FormDescription,
  FormField,
  FormItem,
  FormLabel,
  FormMessage,
} from '@/components/ui/form'
import { Input } from '@/components/ui/input'
import type { UserFormValues } from '../lib'

interface UserQuotaSectionProps {
  form: UseFormReturn<UserFormValues>
  t: TFunction
  currencyLabel: string
  tokensOnly: boolean
  onAdjustQuota: () => void
}

function formatQuotaInput(value: number | undefined, tokensOnly: boolean) {
  if (tokensOnly) return String(value || 0)
  return (value || 0).toFixed(6)
}

export function UserQuotaSection(props: UserQuotaSectionProps) {
  return (
    <FormField
      control={props.form.control}
      name='quota_dollars'
      render={({ field }) => (
        <FormItem>
          <FormLabel>
            {props.t('Remaining Quota ({{currency}})', {
              currency: props.currencyLabel,
            })}
          </FormLabel>
          <div className='flex gap-2'>
            <FormControl>
              <Input
                value={formatQuotaInput(field.value, props.tokensOnly)}
                readOnly
                className='flex-1'
              />
            </FormControl>
            <Button
              type='button'
              variant='outline'
              onClick={props.onAdjustQuota}
            >
              <Pencil className='mr-1 h-4 w-4' />
              {props.t('Adjust Quota')}
            </Button>
          </div>
          <FormDescription>
            {formatQuota(parseQuotaFromDollars(field.value || 0))}
          </FormDescription>
          <FormMessage />
        </FormItem>
      )}
    />
  )
}
