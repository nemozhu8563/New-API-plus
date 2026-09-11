import { render, screen } from '@testing-library/react'
import { createInstance } from 'i18next'
import { I18nextProvider, initReactI18next } from 'react-i18next'
import { describe, expect, test, vi } from 'vitest'

vi.mock('../../../hooks/use-billing-history', () => ({
  useBillingHistory: () => ({
    records: [
      {
        id: 1,
        user_id: 1,
        amount: 10,
        money: 1,
        trade_no: 'success-order',
        payment_method: 'stripe',
        create_time: 1_700_000_000,
        status: 'success',
      },
      {
        id: 2,
        user_id: 1,
        amount: 10,
        money: 1,
        trade_no: 'pending-order',
        payment_method: 'stripe',
        create_time: 1_700_000_000,
        status: 'pending',
      },
      {
        id: 3,
        user_id: 1,
        amount: 10,
        money: 1,
        trade_no: 'expired-order',
        payment_method: 'stripe',
        create_time: 1_700_000_000,
        status: 'expired',
      },
    ],
    total: 3,
    page: 1,
    pageSize: 10,
    keyword: '',
    loading: false,
    completing: false,
    isAdmin: false,
    handlePageChange: vi.fn(),
    handlePageSizeChange: vi.fn(),
    handleSearch: vi.fn(),
    handleCompleteOrder: vi.fn(),
  }),
}))

const { BillingHistoryDialog } = await import('../billing-history-dialog')

describe('billing history dialog status localization', () => {
  test('renders every billing status in the active Chinese locale', async () => {
    const i18n = createInstance()
    await i18n.use(initReactI18next).init({
      lng: 'zh',
      resources: {
        zh: {
          translation: {
            Expired: '已过期',
            Pending: '待确认',
            Success: '成功',
          },
        },
      },
    })

    render(
      <I18nextProvider i18n={i18n}>
        <BillingHistoryDialog open onOpenChange={() => undefined} />
      </I18nextProvider>
    )

    expect(screen.getByText('成功')).toBeInTheDocument()
    expect(screen.getByText('待确认')).toBeInTheDocument()
    expect(screen.getByText('已过期')).toBeInTheDocument()
    expect(screen.queryByText('Success')).not.toBeInTheDocument()
    expect(screen.queryByText('Pending')).not.toBeInTheDocument()
    expect(screen.queryByText('Expired')).not.toBeInTheDocument()
  })
})
