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
/**
 * Column definitions factory
 */
import type { ColumnDef } from '@tanstack/react-table'
import { useCommonLogsColumns } from '../components/columns/common-logs-columns'
import { useTaskLogsColumns } from '../components/columns/task-logs-columns'
import type { LogCategory } from '../types'

/**
 * Get column definitions based on log category
 * Returns any[] due to different log types (UsageLog, TaskLog)
 */
export function useColumnsByCategory(
  logCategory: LogCategory,
  isAdmin: boolean
  // eslint-disable-next-line @typescript-eslint/no-explicit-any
): ColumnDef<any>[] {
  const commonColumns = useCommonLogsColumns(isAdmin)
  const taskColumns = useTaskLogsColumns(isAdmin)

  switch (logCategory) {
    case 'common':
      return commonColumns
    case 'task':
      return taskColumns
    default:
      return commonColumns
  }
}
