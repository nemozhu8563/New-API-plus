/*
Copyright (C) 2023-2026 QuantumNous

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

For commercial licensing, please contact support@quantumnous.com
*/
import assert from 'node:assert/strict'
import { describe, test } from 'node:test'

import { buildUsageLogsExportParams } from '../export'

describe('usage log export filters', () => {
  test('common export uses applied URL filters and strips pagination', () => {
    const params = buildUsageLogsExportParams({
      logCategory: 'common',
      isAdmin: true,
      searchParams: {
        page: 4,
        startTime: 1_720_000_000_000,
        endTime: 1_720_003_600_000,
        model: 'gpt-5',
        username: 'alice',
        channel: '8',
        type: ['2'],
      },
      columnFilters: [{ id: 'model_name', value: 'claude-sonnet' }],
    })

    assert.deepEqual(params, {
      type: 2,
      model_name: 'claude-sonnet',
      channel: 8,
      username: 'alice',
      start_timestamp: 1_720_000_000,
      end_timestamp: 1_720_003_600,
    })
    assert.equal('p' in params, false)
    assert.equal('page_size' in params, false)
  })

  test('self task export ignores admin channel and maps the applied task ID', () => {
    const params = buildUsageLogsExportParams({
      logCategory: 'task',
      isAdmin: false,
      searchParams: {
        startTime: 1_720_000_000_000,
        endTime: 1_720_003_600_000,
        channel: '9',
        filter: 'task-applied',
      },
      columnFilters: [],
    })

    assert.deepEqual(params, {
      start_timestamp: 1_720_000_000,
      end_timestamp: 1_720_003_600,
      task_id: 'task-applied',
    })
  })

  test('drawing export preserves millisecond timestamps', () => {
    const params = buildUsageLogsExportParams({
      logCategory: 'drawing',
      isAdmin: true,
      searchParams: {
        startTime: 1_720_000_000_123,
        endTime: 1_720_003_600_456,
        filter: 'mj-applied',
      },
      columnFilters: [],
    })

    assert.deepEqual(params, {
      start_timestamp: 1_720_000_000_123,
      end_timestamp: 1_720_003_600_456,
      mj_id: 'mj-applied',
    })
  })
})
