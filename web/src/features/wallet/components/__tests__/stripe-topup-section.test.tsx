import assert from 'node:assert/strict'
import { after, describe, test } from 'node:test'

import { Window } from 'happy-dom'

const domWindow = new Window()
const domGlobals = [
  'window',
  'document',
  'navigator',
  'HTMLElement',
  'HTMLButtonElement',
  'HTMLInputElement',
  'SVGElement',
  'Node',
  'Element',
  'Event',
  'CustomEvent',
  'MutationObserver',
  'ResizeObserver',
  'requestAnimationFrame',
  'cancelAnimationFrame',
  'getComputedStyle',
] as const

for (const key of domGlobals) {
  Object.defineProperty(globalThis, key, {
    configurable: true,
    value: domWindow[key],
  })
}

const { act, useState } = await import('react')
const { createRoot } = await import('react-dom/client')
const { createInstance } = await import('i18next')
const { I18nextProvider, initReactI18next } = await import('react-i18next')
const { StripeTopupSection } = await import('../stripe-topup-section')

const i18n = createInstance()
await i18n.use(initReactI18next).init({
  lng: 'en',
  resources: {
    en: {
      translation: {
        Amount: 'Amount',
        'Credits added': 'Credits added',
        'Decrease quantity': 'Decrease quantity',
        'Each package includes {{credits}} ({{price}})':
          'Each package includes {{credits}} ({{price}})',
        'Increase quantity': 'Increase quantity',
        'Order summary': 'Order summary',
        Pay: 'Pay',
        'Purchase quantity': 'Purchase quantity',
        Quantity: 'Quantity',
        Total: 'Total',
      },
    },
  },
})

const reactTestGlobals = globalThis as typeof globalThis & {
  IS_REACT_ACT_ENVIRONMENT?: boolean
}
reactTestGlobals.IS_REACT_ACT_ENVIRONMENT = true

async function renderStripeTopup(options?: {
  maxAmount?: number
  processing?: boolean
}) {
  const checkoutAmounts: number[] = []
  const container = document.createElement('div')
  document.body.append(container)
  const root = createRoot(container)

  function Harness() {
    const [amount, setAmount] = useState(20)
    return (
      <StripeTopupSection
        topupAmount={amount}
        maxAmount={options?.maxAmount}
        processing={options?.processing ?? false}
        onAmountChange={setAmount}
        onCheckout={() => {
          checkoutAmounts.push(amount)
        }}
      />
    )
  }

  await act(async () => {
    root.render(
      <I18nextProvider i18n={i18n}>
        <Harness />
      </I18nextProvider>
    )
  })

  return { checkoutAmounts, container, root }
}

function findButton(container: HTMLElement, label: string) {
  return [...container.querySelectorAll('button')].find((button) =>
    button.textContent?.includes(label)
  )
}

describe('Stripe topup package selection', () => {
  after(() => {
    domWindow.close()
  })

  test('offers fixed packages without an arbitrary amount input', async () => {
    const { container, root } = await renderStripeTopup()

    assert.equal(container.querySelector('input[type="number"]'), null)
    assert.equal(container.querySelectorAll('button[aria-pressed]').length, 6)

    const hundredButton = findButton(container, '¥100')
    assert.ok(hundredButton)
    await act(async () => hundredButton.click())

    assert.equal(hundredButton.getAttribute('aria-pressed'), 'true')
    assert.equal(container.textContent?.includes('$100 USD'), true)
    assert.equal(
      container.querySelector('output[aria-label="Quantity"]')?.textContent,
      '5'
    )

    await act(async () => root.unmount())
    container.remove()
  })

  test('changes whole packages and submits the selected credits directly', async () => {
    const { checkoutAmounts, container, root } = await renderStripeTopup({
      maxAmount: 40,
    })
    const decreaseButton = container.querySelector<HTMLButtonElement>(
      'button[aria-label="Decrease quantity"]'
    )
    const increaseButton = container.querySelector<HTMLButtonElement>(
      'button[aria-label="Increase quantity"]'
    )
    assert.ok(decreaseButton)
    assert.ok(increaseButton)
    assert.equal(decreaseButton.disabled, true)

    await act(async () => increaseButton.click())
    assert.equal(
      container.querySelector('output[aria-label="Quantity"]')?.textContent,
      '2'
    )
    assert.equal(increaseButton.disabled, true)

    const checkoutButton = findButton(container, 'Pay')
    assert.ok(checkoutButton)
    await act(async () => checkoutButton.click())
    assert.deepEqual(checkoutAmounts, [40])

    await act(async () => root.unmount())
    container.remove()
  })
})
