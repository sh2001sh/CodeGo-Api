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
export type SidebarGroupAvailabilityStatus =
  | 'healthy'
  | 'slow'
  | 'degraded'
  | 'unknown'

export type SidebarGroupModelStatusItem = {
  model: string
  status: SidebarGroupAvailabilityStatus
  success_rate: number | null
  sample_window: number
  series_window?: number
  bucket_seconds?: number
  request_count?: number
  series?: SidebarGroupStatusBucket[]
}

export type SidebarGroupStatusBucket = {
  ts: number
  success_rate: number | null
  request_count: number
}

export type SidebarGroupStatusItem = {
  group: string
  status: SidebarGroupAvailabilityStatus
  request_count?: number
  models: SidebarGroupModelStatusItem[]
}

export type SidebarGroupStatusResponse = {
  success: boolean
  message?: string
  data?: SidebarGroupStatusItem[]
}
