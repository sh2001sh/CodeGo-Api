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
import { toast } from 'sonner'

const SUCCESS_STREAK_KEY = 'workshop:success-streak'
const FAILURE_STREAK_KEY = 'workshop:failure-streak'
const NIGHT_TRAINING_KEY = 'workshop:night-training'

function getTodayToken() {
  const now = new Date()
  const year = now.getFullYear()
  const month = `${now.getMonth() + 1}`.padStart(2, '0')
  const day = `${now.getDate()}`.padStart(2, '0')
  return `${year}-${month}-${day}`
}

function readCounter(key: string): number {
  const raw = window.localStorage.getItem(key)
  const value = Number(raw)
  return Number.isFinite(value) && value > 0 ? value : 0
}

function writeCounter(key: string, value: number) {
  window.localStorage.setItem(key, `${value}`)
}

export function maybeShowNightWorkshopToast() {
  const now = new Date()
  const hour = now.getHours()
  if (hour < 2 || hour >= 5) return
  const todayKey = `${NIGHT_TRAINING_KEY}:${getTodayToken()}`
  if (window.localStorage.getItem(todayKey) === '1') return
  window.localStorage.setItem(todayKey, '1')
  toast.info('深夜特训开始了，今晚的第一声召唤格外清醒')
}

export function recordWorkshopCallSuccess() {
  const nextSuccessCount = readCounter(SUCCESS_STREAK_KEY) + 1
  writeCounter(SUCCESS_STREAK_KEY, nextSuccessCount)
  writeCounter(FAILURE_STREAK_KEY, 0)
  if (nextSuccessCount === 5) {
    toast.success('精灵连击已触发，连续 5 次召唤成功')
  }
}

export function recordWorkshopCallFailure() {
  const nextFailureCount = readCounter(FAILURE_STREAK_KEY) + 1
  writeCounter(FAILURE_STREAK_KEY, nextFailureCount)
  writeCounter(SUCCESS_STREAK_KEY, 0)
  if (nextFailureCount === 3) {
    toast.error('精灵有点累了，稍后再试一次')
  }
}
