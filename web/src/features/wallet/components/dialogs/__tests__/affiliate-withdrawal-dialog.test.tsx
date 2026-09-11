import assert from 'node:assert/strict'

import { Window } from 'happy-dom'
import { afterAll as after, describe, test } from 'vitest'

const domWindow = new Window()
const domGlobals = [
  'window',
  'document',
  'navigator',
  'HTMLElement',
  'HTMLButtonElement',
  'HTMLInputElement',
  'HTMLTextAreaElement',
  'SVGElement',
  'Node',
  'Element',
  'Event',
  'CustomEvent',
  'MouseEvent',
  'KeyboardEvent',
  'FocusEvent',
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

const { act } = await import('react')
const { createRoot } = await import('react-dom/client')
const { createInstance } = await import('i18next')
const { I18nextProvider, initReactI18next } = await import('react-i18next')
const { AffiliateWithdrawalDialog } =
  await import('../affiliate-withdrawal-dialog')

const i18n = createInstance()
await i18n.use(initReactI18next).init({
  lng: 'en',
  resources: {
    en: {
      translation: {
        'Add offline payment details or a note for review':
          'Add offline payment details or a note for review',
        'Available cashback': 'Available cashback',
        Cancel: 'Cancel',
        'Request cashback withdrawal': 'Request cashback withdrawal',
        'Submit request': 'Submit request',
        'The amount is frozen after submission. An administrator confirms the offline payment or rejects and returns it.':
          'The amount is frozen after submission. An administrator confirms the offline payment or rejects and returns it.',
        'Withdrawal amount': 'Withdrawal amount',
        'Withdrawal note': 'Withdrawal note',
      },
    },
  },
})

const reactTestGlobals = globalThis as typeof globalThis & {
  IS_REACT_ACT_ENVIRONMENT?: boolean
}
reactTestGlobals.IS_REACT_ACT_ENVIRONMENT = true

function changeInputValue(input: HTMLInputElement, value: string) {
  const valueSetter = Object.getOwnPropertyDescriptor(
    domWindow.HTMLInputElement.prototype,
    'value'
  )?.set
  assert.ok(valueSetter)
  valueSetter.call(input, value)
  input.dispatchEvent(
    new domWindow.Event('input', { bubbles: true }) as unknown as Event
  )
}

function changeTextareaValue(textarea: HTMLTextAreaElement, value: string) {
  const valueSetter = Object.getOwnPropertyDescriptor(
    domWindow.HTMLTextAreaElement.prototype,
    'value'
  )?.set
  assert.ok(valueSetter)
  valueSetter.call(textarea, value)
  textarea.dispatchEvent(
    new domWindow.Event('input', { bubbles: true }) as unknown as Event
  )
}

function findButton(label: string) {
  return [...document.querySelectorAll('button')].find(
    (button) => button.textContent === label
  )
}

async function renderWithdrawalDialog(
  overrides: Partial<Parameters<typeof AffiliateWithdrawalDialog>[0]> = {}
) {
  const container = document.createElement('div')
  document.body.append(container)
  const root = createRoot(container)
  const props: Parameters<typeof AffiliateWithdrawalDialog>[0] = {
    open: true,
    onOpenChange: () => undefined,
    onConfirm: async () => true,
    availableQuota: 1_000_000,
    submitting: false,
    ...overrides,
  }
  await act(async () => {
    root.render(
      <I18nextProvider i18n={i18n}>
        <AffiliateWithdrawalDialog {...props} />
      </I18nextProvider>
    )
  })
  return { container, root }
}

async function cleanupDialog(
  root: Awaited<ReturnType<typeof renderWithdrawalDialog>>['root'],
  container: HTMLElement
) {
  await act(async () => root.unmount())
  container.remove()
}

describe('affiliate withdrawal dialog', () => {
  after(() => {
    domWindow.close()
  })

  test('enables submission only for a positive amount within available cashback', async () => {
    const { container, root } = await renderWithdrawalDialog()
    const amountInput = document.querySelector<HTMLInputElement>(
      '#affiliate-withdrawal-amount'
    )
    const submitButton = findButton('Submit request')
    assert.ok(amountInput)
    assert.ok(submitButton)
    assert.equal(submitButton.disabled, true)

    await act(async () => changeInputValue(amountInput, '3'))
    assert.equal(submitButton.disabled, true)

    await act(async () => changeInputValue(amountInput, '2'))
    assert.equal(submitButton.disabled, false)

    await cleanupDialog(root, container)
  })

  test('submits exact quota units and closes only after a successful request', async () => {
    let submittedAmount = 0
    let submittedNote = ''
    const openChanges: boolean[] = []
    const { container, root } = await renderWithdrawalDialog({
      onOpenChange: (open) => {
        openChanges.push(open)
      },
      onConfirm: async (amountQuota, note) => {
        submittedAmount = amountQuota
        submittedNote = note
        return true
      },
    })
    const amountInput = document.querySelector<HTMLInputElement>(
      '#affiliate-withdrawal-amount'
    )
    const noteInput = document.querySelector<HTMLTextAreaElement>(
      '#affiliate-withdrawal-note'
    )
    assert.ok(amountInput)
    assert.ok(noteInput)

    await act(async () => {
      changeInputValue(amountInput, '1.5')
      changeTextareaValue(noteInput, 'PayPal account')
    })
    const submitButton = findButton('Submit request')
    assert.ok(submitButton)
    assert.equal(submitButton.disabled, false)
    await act(async () => submitButton.click())

    assert.equal(submittedAmount, 750_000)
    assert.equal(submittedNote, 'PayPal account')
    assert.deepEqual(openChanges, [false])

    await cleanupDialog(root, container)
  })

  test('keeps the dialog open when the withdrawal request fails', async () => {
    const openChanges: boolean[] = []
    const { container, root } = await renderWithdrawalDialog({
      onOpenChange: (open) => {
        openChanges.push(open)
      },
      onConfirm: async () => false,
    })
    const amountInput = document.querySelector<HTMLInputElement>(
      '#affiliate-withdrawal-amount'
    )
    assert.ok(amountInput)
    await act(async () => changeInputValue(amountInput, '1'))

    const submitButton = findButton('Submit request')
    assert.ok(submitButton)
    await act(async () => submitButton.click())

    assert.deepEqual(openChanges, [])
    assert.ok(
      document.querySelector<HTMLInputElement>('#affiliate-withdrawal-amount')
    )

    await cleanupDialog(root, container)
  })

  test('disables cancel and submit while a request is in progress', async () => {
    const { container, root } = await renderWithdrawalDialog({
      submitting: true,
    })
    const cancelButton = findButton('Cancel')
    const submitButton = findButton('Submit request')
    assert.ok(cancelButton)
    assert.ok(submitButton)
    assert.equal(cancelButton.disabled, true)
    assert.equal(submitButton.disabled, true)

    await cleanupDialog(root, container)
  })
})
