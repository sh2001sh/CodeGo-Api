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
import { useState, useMemo } from 'react'
import { type Table } from '@tanstack/react-table'
import { Download, Trash2 } from 'lucide-react'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'
import { Button } from '@/components/ui/button'
import {
  Tooltip,
  TooltipContent,
  TooltipTrigger,
} from '@/components/ui/tooltip'
import { ConfirmDialog } from '@/components/confirm-dialog'
import { CopyButton } from '@/components/copy-button'
import { DataTableBulkActions as BulkActionsToolbar } from '@/components/data-table'
import {
  deleteInvalidRedemptions,
  getRedemptions,
  searchRedemptions,
} from '../api'
import { downloadRedemptionTextFile } from '../lib'
import { type Redemption } from '../types'
import { useRedemptions } from './redemptions-provider'

type DataTableBulkActionsProps<TData> = {
  table: Table<TData>
}

export function DataTableBulkActions<TData>({
  table,
}: DataTableBulkActionsProps<TData>) {
  const { t } = useTranslation()
  const { triggerRefresh } = useRedemptions()
  const [showDeleteInvalidConfirm, setShowDeleteInvalidConfirm] =
    useState(false)
  const [isDeleting, setIsDeleting] = useState(false)
  const [isExporting, setIsExporting] = useState(false)
  const selectedRows = table.getFilteredSelectedRowModel().rows
  const globalFilter = String(table.getState().globalFilter || '').trim()
  const filteredRows = table.getFilteredRowModel().rows

  const contentToCopy = useMemo(() => {
    const selectedCodes = selectedRows.map((row) => {
      const redemption = row.original as Redemption
      return `${redemption.name}\t${redemption.key}`
    })
    return selectedCodes.join('\n')
  }, [selectedRows])

  const buildExportContent = (rows: Redemption[]) =>
    rows.map((redemption) => `${redemption.name}\t${redemption.key}`).join('\n')

  const exportRows = (rows: Redemption[], fileBaseName: string) => {
    if (rows.length === 0) {
      toast.error(t('No redemption codes to export'))
      return
    }
    downloadRedemptionTextFile(`${buildExportContent(rows)}\n`, fileBaseName)
    toast.success(
      t('Exported {{count}} redemption codes', { count: rows.length })
    )
  }

  const fetchAllRowsForCurrentFilter = async () => {
    const pageSize = 200
    let page = 1
    let total: number
    const items: Redemption[] = []

    do {
      const result = globalFilter
        ? await searchRedemptions({
            keyword: globalFilter,
            p: page,
            page_size: pageSize,
          })
        : await getRedemptions({
            p: page,
            page_size: pageSize,
          })
      const batch = result.data?.items || []
      total = result.data?.total || batch.length
      items.push(...batch)
      page += 1
    } while (items.length < total)

    return items
  }

  const handleExport = async () => {
    setIsExporting(true)
    try {
      if (selectedRows.length > 0) {
        exportRows(
          selectedRows.map((row) => row.original as Redemption),
          globalFilter
            ? `${globalFilter}-selected-redemptions`
            : 'selected-redemptions'
        )
        return
      }

      if (globalFilter) {
        const rows = await fetchAllRowsForCurrentFilter()
        exportRows(rows, `${globalFilter}-redemptions`)
        return
      }

      exportRows(
        filteredRows.map((row) => row.original as Redemption),
        'redemptions-current-page'
      )
    } finally {
      setIsExporting(false)
    }
  }

  const handleDeleteInvalid = async () => {
    setIsDeleting(true)
    try {
      const result = await deleteInvalidRedemptions()

      if (result.success) {
        const count = result.data || 0
        toast.success(
          t('Successfully deleted {{count}} invalid redemption codes', {
            count,
          })
        )
        table.resetRowSelection()
        triggerRefresh()
        setShowDeleteInvalidConfirm(false)
      }
    } finally {
      setIsDeleting(false)
    }
  }

  return (
    <>
      <BulkActionsToolbar table={table} entityName={t('redemption code')}>
        <CopyButton
          value={contentToCopy}
          variant='outline'
          size='icon'
          className='size-8'
          tooltip={t('Copy selected codes')}
          successTooltip={t('Codes copied!')}
          aria-label={t('Copy selected codes')}
        />

        <Tooltip>
          <TooltipTrigger
            render={
              <Button
                variant='outline'
                size='icon'
                onClick={handleExport}
                className='size-8'
                disabled={isExporting}
                aria-label={t('Export redemption codes')}
                title={t('Export redemption codes')}
              />
            }
          >
            <Download />
            <span className='sr-only'>{t('Export redemption codes')}</span>
          </TooltipTrigger>
          <TooltipContent>
            <p>
              {selectedRows.length > 0
                ? t('Export selected redemption codes')
                : globalFilter
                  ? t('Export all redemption codes matching the current filter')
                  : t('Export current page redemption codes')}
            </p>
          </TooltipContent>
        </Tooltip>

        <Tooltip>
          <TooltipTrigger
            render={
              <Button
                variant='destructive'
                size='icon'
                onClick={() => setShowDeleteInvalidConfirm(true)}
                className='size-8'
                aria-label={t('Delete invalid redemption codes')}
                title={t('Delete invalid redemption codes')}
              />
            }
          >
            <Trash2 />
            <span className='sr-only'>{t('Delete invalid codes')}</span>
          </TooltipTrigger>
          <TooltipContent>
            <p>{t('Delete invalid codes (used/disabled/expired)')}</p>
          </TooltipContent>
        </Tooltip>
      </BulkActionsToolbar>

      <ConfirmDialog
        destructive
        open={showDeleteInvalidConfirm}
        onOpenChange={setShowDeleteInvalidConfirm}
        handleConfirm={handleDeleteInvalid}
        isLoading={isDeleting}
        className='max-w-md'
        title={t('Delete Invalid Redemption Codes?')}
        desc={
          <>
            {t('This will delete all')} <strong>{t('used')}</strong>,{' '}
            <strong>{t('disabled')}</strong>
            {t(', and')} <strong>{t('expired')}</strong>{' '}
            {t('redemption codes.')}
            <br />
            {t('This action cannot be undone.')}
          </>
        }
        confirmText={t('Delete Invalid')}
      />
    </>
  )
}
