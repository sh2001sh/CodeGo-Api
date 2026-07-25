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
function isLocalHost(hostname: string): boolean {
  return ['localhost', '127.0.0.1', '0.0.0.0'].includes(hostname)
}

function getWindowOrigin(): string {
  if (typeof window === 'undefined') return ''
  return window.location.origin
}

function hasProtocol(value: string): boolean {
  return /^[a-z][a-z\d+\-.]*:\/\//i.test(value)
}

export function normalizePublicServerAddress(value?: string): string {
  const raw = (value || getWindowOrigin()).trim()
  let base = raw.replace(/\/+$/, '').replace(/\/v1$/i, '')
  if (base && !hasProtocol(base)) {
    base = `https://${base}`
  }

  try {
    const url = new URL(base)
    if (url.protocol === 'http:' && !isLocalHost(url.hostname)) {
      url.protocol = 'https:'
    }
    return url.toString().replace(/\/+$/, '')
  } catch {
    if (
      /^http:\/\//i.test(base) &&
      !/^http:\/\/(localhost|127\.0\.0\.1|0\.0\.0\.0)([:/]|$)/i.test(base)
    ) {
      return base.replace(/^http:\/\//i, 'https://')
    }
    return base
  }
}

export function getConfiguredServerAddress(fallback?: string): string {
  try {
    const raw = localStorage.getItem('status')
    if (raw) {
      const status = JSON.parse(raw) as { server_address?: string }
      if (status.server_address) {
        return normalizePublicServerAddress(status.server_address)
      }
    }
  } catch {
    /* empty */
  }

  return normalizePublicServerAddress(fallback || getWindowOrigin())
}
