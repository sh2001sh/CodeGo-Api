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
import { parseQuotaFromDollars } from '@/lib/format'
import type { User } from '../types'
import { UserQuotaDialog } from './user-quota-dialog'

interface UserQuotaDialogsProps {
  currentRow?: User
  quotaDialogOpen: boolean
  currentQuotaRaw: number
  onQuotaOpenChange: (open: boolean) => void
  onSuccess: () => void
}

export function UserQuotaDialogs(props: UserQuotaDialogsProps) {
  if (!props.currentRow) return null

  return (
    <UserQuotaDialog
      open={props.quotaDialogOpen}
      onOpenChange={props.onQuotaOpenChange}
      userId={props.currentRow.id}
      currentQuota={parseQuotaFromDollars(props.currentQuotaRaw || 0)}
      onSuccess={props.onSuccess}
    />
  )
}
