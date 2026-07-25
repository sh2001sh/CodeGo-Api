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
import z from 'zod'
import { createFileRoute, redirect } from '@tanstack/react-router'
import { UsageLogs } from '@/features/usage-logs'
import {
  resolveUsageLogsSectionId,
  resolveUsageLogsRouteRedirect,
} from '@/features/usage-logs/section-meta'

const logTypeValues = ['0', '1', '2', '3', '4', '5', '6'] as const

const usageLogsSearchSchema = z.object({
  page: z.number().optional().catch(1),
  pageSize: z.number().optional().catch(20),
  type: z.array(z.enum(logTypeValues)).optional().catch([]),
  filter: z.string().optional().catch(''),
  model: z.string().optional().catch(''),
  token: z.string().optional().catch(''),
  channel: z.string().optional().catch(''),
  group: z.string().optional().catch(''),
  username: z.string().optional().catch(''),
  requestId: z.string().optional().catch(''),
  upstreamRequestId: z.string().optional().catch(''),
  startTime: z.number().optional(),
  endTime: z.number().optional(),
})

export const Route = createFileRoute('/_authenticated/usage-logs/$section')({
  beforeLoad: ({ params, search }) => {
    const redirectState = resolveUsageLogsRouteRedirect(params.section, search)
    if (redirectState) {
      throw redirect({
        to: '/usage-logs/$section',
        params: { section: resolveUsageLogsSectionId(redirectState.section) },
        ...(redirectState.search ? { search: redirectState.search } : {}),
        ...(redirectState.replace ? { replace: true } : {}),
      })
    }
  },
  validateSearch: usageLogsSearchSchema,
  component: UsageLogs,
})
