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
import {
  USAGE_LOGS_DEFAULT_SECTION,
  isUsageLogsSectionId,
  type UsageLogsSectionId,
} from './section-registry-data.ts'

export type UsageLogsSectionMeta = {
  titleKey: string
  descriptionKey: string
}

const USAGE_LOGS_SECTION_META: Record<
  UsageLogsSectionId,
  UsageLogsSectionMeta
> = {
  common: {
    titleKey: 'Common Logs',
    descriptionKey: 'View and manage your API usage logs',
  },
  task: {
    titleKey: 'Task Logs',
    descriptionKey: 'View and manage your task logs',
  },
}

export function getUsageLogsSectionMeta(
  section: UsageLogsSectionId
): UsageLogsSectionMeta {
  return (
    USAGE_LOGS_SECTION_META[section] ??
    USAGE_LOGS_SECTION_META[USAGE_LOGS_DEFAULT_SECTION]
  )
}

export function resolveUsageLogsSectionId(section: string): UsageLogsSectionId {
  return isUsageLogsSectionId(section) ? section : USAGE_LOGS_DEFAULT_SECTION
}

export type UsageLogsRouteSearch = {
  type?: string[]
  [key: string]: unknown
}

export type UsageLogsRouteRedirect = {
  section: UsageLogsSectionId
  search?: Record<string, unknown>
  replace?: boolean
}

export function resolveUsageLogsRouteRedirect(
  section: string,
  search?: UsageLogsRouteSearch
): UsageLogsRouteRedirect | null {
  if (!isUsageLogsSectionId(section)) {
    return {
      section: USAGE_LOGS_DEFAULT_SECTION,
    }
  }

  if (
    section !== 'common' &&
    Array.isArray(search?.type) &&
    search.type.length > 0
  ) {
    return {
      section,
      search: {
        ...search,
        type: undefined,
      },
      replace: true,
    }
  }

  return null
}
