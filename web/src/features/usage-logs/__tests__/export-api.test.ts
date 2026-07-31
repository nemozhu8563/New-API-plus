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
import { afterEach, describe, test } from 'node:test'

import { api } from '@/lib/api'

import { exportUsageLogsCSV } from '../api'

const originalGet = api.get

afterEach(() => {
  api.get = originalGet
})

describe('usage log export API', () => {
  test('requests the self endpoint as a blob and keeps the server filename', async () => {
    const blob = new Blob(['id\\n1\\n'], { type: 'text/csv' })
    let requestedPath = ''
    let requestedConfig: Record<string, unknown> | undefined
    api.get = (async (path: string, config?: Record<string, unknown>) => {
      requestedPath = path
      requestedConfig = config
      return {
        data: blob,
        headers: {
          'content-disposition':
            'attachment; filename="usage-logs-common-20260731.csv"',
        },
      }
    }) as typeof api.get

    const result = await exportUsageLogsCSV({
      logCategory: 'common',
      isAdmin: false,
      params: { type: 2 },
    })

    assert.equal(requestedPath, '/api/log/self/export')
    assert.deepEqual(requestedConfig?.params, { type: 2 })
    assert.equal(requestedConfig?.responseType, 'blob')
    assert.equal(requestedConfig?.skipErrorHandler, true)
    assert.equal(result.blob, blob)
    assert.equal(result.filename, 'usage-logs-common-20260731.csv')
  })

  test('surfaces the JSON message returned inside a blob error', async () => {
    api.get = (async () => {
      throw {
        response: {
          data: new Blob([
            JSON.stringify({
              success: false,
              message: 'Narrow the filters to 50000 rows or fewer',
            }),
          ]),
        },
      }
    }) as typeof api.get

    await assert.rejects(
      () =>
        exportUsageLogsCSV({
          logCategory: 'task',
          isAdmin: true,
          params: {},
        }),
      /Narrow the filters to 50000 rows or fewer/
    )
  })
})
