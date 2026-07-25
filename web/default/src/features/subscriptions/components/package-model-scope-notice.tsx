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
import { BadgeInfo } from 'lucide-react'
import { useTranslation } from 'react-i18next'
import { cn } from '@/lib/utils'

export function PackageModelScopeNotice(props: { className?: string }) {
  const { t } = useTranslation()
  return (
    <div
      role='note'
      className={cn(
        'border-border bg-muted/35 flex items-start gap-3 rounded-lg border px-4 py-3',
        props.className
      )}
    >
      <BadgeInfo
        className='text-primary mt-0.5 size-4 shrink-0'
        aria-hidden='true'
      />
      <div>
        <p className='text-foreground text-sm font-semibold'>
          {t('Package purchase rules')}
        </p>
        <ul className='text-muted-foreground mt-1 list-disc space-y-1 pl-4 text-xs leading-5'>
          <li>
            {t(
              'Plan quota and direct top-ups use the same wallet for every model.'
            )}
          </li>
          <li>
            {t(
              'Renewal is available after at least 30% of the current plan quota is used. The price follows the used percentage with a 30% minimum; renewal restarts the term and unused quota does not roll over.'
            )}
          </li>
        </ul>
      </div>
    </div>
  )
}
