import { render, screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { beforeEach, describe, expect, test, vi } from 'vitest'

import { SiteTelemetryConsent } from '../site-telemetry-consent'

const telemetry = vi.hoisted(() => ({
  getAnalyticsConsent: vi.fn(),
  isSiteTelemetryEnabled: vi.fn(),
  setAnalyticsConsent: vi.fn(),
}))

vi.mock('@/lib/site-telemetry', () => telemetry)

describe('SiteTelemetryConsent', () => {
  beforeEach(() => {
    telemetry.getAnalyticsConsent.mockReturnValue(null)
    telemetry.isSiteTelemetryEnabled.mockReturnValue(true)
  })

  test('records an explicit acceptance and dismisses the banner', async () => {
    const user = userEvent.setup()
    render(<SiteTelemetryConsent />)

    await user.click(screen.getByRole('button', { name: 'Allow analytics' }))

    expect(telemetry.setAnalyticsConsent).toHaveBeenCalledWith('granted')
    expect(screen.queryByRole('dialog')).not.toBeInTheDocument()
  })

  test('stays hidden when a choice already exists', () => {
    telemetry.getAnalyticsConsent.mockReturnValue('denied')

    render(<SiteTelemetryConsent />)

    expect(screen.queryByRole('dialog')).not.toBeInTheDocument()
  })
})
