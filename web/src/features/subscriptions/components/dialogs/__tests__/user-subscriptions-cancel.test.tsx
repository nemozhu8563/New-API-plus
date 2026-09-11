import { render, screen, waitFor, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { afterEach, expect, test, vi } from 'vitest'

import { api } from '@/lib/api'

import { UserSubscriptionsDialog } from '../user-subscriptions-dialog'

const originalAdapter = api.defaults.adapter

afterEach(() => {
  api.defaults.adapter = originalAdapter
})

test('admin can cancel internal access without deleting the subscription or opening Stripe', async () => {
  const requests: string[] = []
  let cancelled = false
  const onSuccess = vi.fn()
  api.defaults.adapter = async (config) => {
    const url = config.url || ''
    requests.push(`${config.method} ${url}`)
    let data: unknown = []
    if (url === '/api/subscription/admin/user_subscriptions/7/invalidate') {
      cancelled = true
      data = null
    } else if (url === '/api/subscription/admin/users/8/subscriptions') {
      data = [
        {
          subscription: {
            id: 7,
            user_id: 8,
            plan_id: 1,
            status: cancelled ? 'cancelled' : 'active',
            source: 'order',
            start_time: 1_700_000_000,
            end_time: 4_000_000_000,
            amount_total: 1000,
            amount_used: 100,
          },
        },
      ]
    }
    return {
      data: { success: true, data },
      status: 200,
      statusText: 'OK',
      headers: {},
      config,
    }
  }

  const user = userEvent.setup()
  render(
    <UserSubscriptionsDialog
      open
      onOpenChange={() => undefined}
      user={{ id: 8, username: 'buyer' }}
      onSuccess={onSuccess}
    />
  )
  await screen.findByText('Active')
  await user.click(screen.getByRole('button', { name: 'Actions' }))
  await user.click(
    await screen.findByRole('menuitem', { name: 'Cancel subscription' })
  )
  const confirmation = await screen.findByRole('alertdialog', {
    name: 'Cancel subscription',
  })
  expect(
    within(confirmation).getByText(
      /immediately deactivated.*Historical records/
    )
  ).toBeInTheDocument()
  await user.click(
    within(confirmation).getByRole('button', { name: 'Continue' })
  )

  await screen.findByText('Invalidated')
  await waitFor(() => expect(onSuccess).toHaveBeenCalledOnce())
  expect(requests.filter((request) => request.startsWith('post '))).toEqual([
    'post /api/subscription/admin/user_subscriptions/7/invalidate',
  ])
  expect(
    requests.some(
      (request) => request.startsWith('delete ') || request.includes('/stripe/')
    )
  ).toBe(false)
})
