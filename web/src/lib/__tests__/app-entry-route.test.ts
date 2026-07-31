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

import { DEFAULT_CONSOLE_ROUTE, resolveAppEntryRoute } from '../app-entry-route'

describe('application entry routing', () => {
  test('sends anonymous visitors to sign in', () => {
    assert.equal(resolveAppEntryRoute(false), '/sign-in')
  })

  test('sends authenticated visitors directly to the overview dashboard', () => {
    assert.equal(resolveAppEntryRoute(true), DEFAULT_CONSOLE_ROUTE)
    assert.equal(DEFAULT_CONSOLE_ROUTE, '/dashboard/overview')
  })
})
