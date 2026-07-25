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
import { createSectionRegistry } from '@/features/system-settings/utils/section-registry'
import {
  USAGE_LOGS_DEFAULT_SECTION,
  USAGE_LOGS_SECTION_IDS,
  isUsageLogsSectionId as isUsageLogsSectionIdData,
  type UsageLogsSectionId,
} from './section-registry-data.ts'

const USAGE_LOGS_SECTIONS = [
  {
    id: 'common',
    titleKey: 'Common Logs',
    descriptionKey: 'View and manage your API usage logs',
    build: () => null,
  },
  {
    id: 'task',
    titleKey: 'Task Logs',
    descriptionKey: 'View and manage your task logs',
    build: () => null,
  },
] as const

const usageLogsRegistry = createSectionRegistry<
  UsageLogsSectionId,
  Record<string, never>,
  []
>({
  sections: USAGE_LOGS_SECTIONS,
  defaultSection: USAGE_LOGS_DEFAULT_SECTION,
  basePath: '/usage-logs',
  urlStyle: 'path',
})

export {
  USAGE_LOGS_SECTION_IDS,
  USAGE_LOGS_DEFAULT_SECTION,
  type UsageLogsSectionId,
}

export function isUsageLogsSectionId(s: string): s is UsageLogsSectionId {
  return isUsageLogsSectionIdData(s)
}

export const getUsageLogsSectionNavItems = usageLogsRegistry.getSectionNavItems
