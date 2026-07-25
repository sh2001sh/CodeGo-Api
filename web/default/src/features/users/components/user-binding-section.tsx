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
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { BINDING_FIELDS } from '../constants'
import type { User } from '../types'

interface UserBindingSectionProps {
  currentRow?: User
  t: TFunction
}

export function UserBindingSection(props: UserBindingSectionProps) {
  return (
    <div className='space-y-4'>
      <h3 className='text-sm font-medium'>{props.t('Binding Information')}</h3>
      <p className='text-muted-foreground text-xs'>
        {props.t(
          'Third-party account bindings (read-only, managed by user in profile settings)'
        )}
      </p>

      <div className='space-y-3'>
        {BINDING_FIELDS.map(({ key, label }) => (
          <div key={key}>
            <Label className='text-muted-foreground text-xs'>
              {props.t(label)}
            </Label>
            <Input
              value={(props.currentRow?.[key as keyof User] as string) || '-'}
              disabled
              className='mt-1'
            />
          </div>
        ))}
      </div>
    </div>
  )
}
