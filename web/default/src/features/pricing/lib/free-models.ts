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
import type { PricingModel } from '../types'

function getEnabledGroupRatios(
  model: PricingModel,
  groupRatios?: Record<string, number>
): number[] {
  const sourceRatios = groupRatios ?? model.group_ratio ?? {}
  const groups = Array.isArray(model.enable_groups) ? model.enable_groups : []

  return groups
    .map((group) => sourceRatios[group])
    .filter((ratio): ratio is number => Number.isFinite(ratio))
}

export function isFreeModel(
  model: PricingModel,
  groupRatios?: Record<string, number>
): boolean {
  const ratios = getEnabledGroupRatios(model, groupRatios)
  return ratios.some((ratio) => ratio === 0)
}

export function getFreeEligibleGroups(
  model: PricingModel,
  groupRatios?: Record<string, number>
): string[] {
  const sourceRatios = groupRatios ?? model.group_ratio ?? {}
  const groups = Array.isArray(model.enable_groups) ? model.enable_groups : []

  return groups.filter((group) => sourceRatios[group] === 0)
}

export function countFreeModels(
  models: PricingModel[],
  groupRatios?: Record<string, number>
): number {
  return models.reduce(
    (count, model) => count + (isFreeModel(model, groupRatios) ? 1 : 0),
    0
  )
}
