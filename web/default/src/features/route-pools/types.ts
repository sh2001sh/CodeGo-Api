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
export type RoutePoolMember = {
  channel_id: number
  cost_multiplier: number
  model_cost_overrides: string
  enabled: boolean
}

export type RoutePool = {
  id: number
  name: string
  group: string
  enabled: boolean
  auto_discover: boolean
}

export type RoutePoolGroup = {
  group: string
  pool_id: number
  enabled: boolean
  algorithm_active: boolean
  auto_discover: boolean
  channels: Array<{
    channel_id: number
    channel_name: string
    channel_status: number
    models: string
    enabled: boolean
    cost_multiplier: number
    model_cost_overrides: string
  }>
}

export type RoutePoolDetail = {
  pool: RoutePool
  members: RoutePoolMember[]
}

export type RoutePoolMetrics = {
  members: Array<{
    channel_id: number
    channel_name: string
    eligible: boolean
    score: number
    health: {
      state: string
      success_rate_5m: number
      ttft_p50_ms: number
      ttft_p95_ms: number
      cooling_until?: string
    }
  }>
}

export type RoutePoolDraft = RoutePool & { members: RoutePoolMember[] }

export const createBlankRoutePoolDraft = (): RoutePoolDraft => ({
  id: 0,
  name: '',
  group: '',
  enabled: true,
  auto_discover: false,
  members: [],
})
