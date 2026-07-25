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
import assert from 'node:assert/strict'
import { describe, test } from 'node:test'
import {
  getUsageLogsSectionMeta,
  resolveUsageLogsSectionId,
  resolveUsageLogsRouteRedirect,
} from './section-meta.ts'

describe('usage logs section meta', () => {
  test('returns the correct copy for each section', () => {
    assert.equal(getUsageLogsSectionMeta('common').titleKey, 'Common Logs')
    assert.equal(getUsageLogsSectionMeta('task').titleKey, 'Task Logs')
  })
})

describe('usage logs route redirect resolution', () => {
  test('normalizes arbitrary section ids to the default section', () => {
    assert.equal(resolveUsageLogsSectionId('logs'), 'common')
    assert.equal(resolveUsageLogsSectionId('task'), 'task')
  })

  test('redirects unknown sections to the default section', () => {
    assert.deepEqual(resolveUsageLogsRouteRedirect('logs'), {
      section: 'common',
    })
  })

  test('keeps common searches intact', () => {
    assert.equal(resolveUsageLogsRouteRedirect('common', { type: ['1'] }), null)
  })

  test('drops type filters for non-common sections before navigation', () => {
    assert.deepEqual(
      resolveUsageLogsRouteRedirect('task', {
        type: ['1'],
        page: 2,
      }),
      {
        section: 'task',
        search: {
          type: undefined,
          page: 2,
        },
        replace: true,
      }
    )
  })
})
