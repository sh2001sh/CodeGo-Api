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
import { useEffect, useState } from 'react'
import { ApiKeysDeleteDialog } from './api-keys-delete-dialog'
import { ApiKeysMutateDrawer } from './api-keys-mutate-drawer'
import { useApiKeys } from './api-keys-provider'

export function ApiKeysDialogs() {
  const { open, setOpen, currentRow } = useApiKeys()
  const [lastMutateSide, setLastMutateSide] = useState<'left' | 'right'>(
    'right'
  )
  const mutateSide =
    open === 'create' ? 'left' : open === 'update' ? 'right' : lastMutateSide

  useEffect(() => {
    if (open === 'create') {
      setLastMutateSide('left')
    } else if (open === 'update') {
      setLastMutateSide('right')
    }
  }, [open])

  return (
    <>
      <ApiKeysMutateDrawer
        open={open === 'create' || open === 'update'}
        onOpenChange={(isOpen) => !isOpen && setOpen(null)}
        currentRow={open === 'update' ? currentRow || undefined : undefined}
        side={mutateSide}
      />
      <ApiKeysDeleteDialog />
    </>
  )
}
