import { render, screen } from '@testing-library/react'
import { createInstance } from 'i18next'
import { I18nextProvider, initReactI18next } from 'react-i18next'
import { beforeAll, describe, expect, test } from 'vitest'

import {
  ChainOfThought,
  ChainOfThoughtHeader,
} from '@/components/ai-elements/chain-of-thought'
import {
  PromptInputAttachment,
  PromptInputProvider,
} from '@/components/ai-elements/prompt-input'
import { Tool, ToolHeader, ToolOutput } from '@/components/ai-elements/tool'

const i18n = createInstance()

beforeAll(async () => {
  await i18n.use(initReactI18next).init({
    lng: 'zh',
    resources: {
      zh: {
        translation: {
          Attachment: '附件',
          'Awaiting Approval': '等待批准',
          'Chain of Thought': '思维链',
          Completed: '已完成',
          Denied: '已拒绝',
          Error: '错误',
          Image: '图片',
          Pending: '待处理',
          Responded: '已响应',
          Result: '结果',
          Running: '运行中',
        },
      },
    },
  })
})

function renderLocalized(ui: React.ReactNode) {
  return render(<I18nextProvider i18n={i18n}>{ui}</I18nextProvider>)
}

describe('AI element localization', () => {
  test('localizes every tool status and output heading', () => {
    const statuses = [
      ['input-streaming', '待处理'],
      ['input-available', '运行中'],
      ['approval-requested', '等待批准'],
      ['approval-responded', '已响应'],
      ['output-available', '已完成'],
      ['output-error', '错误'],
      ['output-denied', '已拒绝'],
    ] as const

    renderLocalized(
      <>
        {statuses.map(([status]) => (
          <Tool key={status}>
            <ToolHeader type='tool-test' state={status} />
          </Tool>
        ))}
        <ToolOutput output='done' errorText={undefined} />
      </>
    )

    for (const [, label] of statuses) {
      expect(screen.getByText(label)).toBeInTheDocument()
    }
    expect(screen.getByText('结果')).toBeInTheDocument()
  })

  test('localizes default reasoning and attachment labels', () => {
    renderLocalized(
      <>
        <ChainOfThought>
          <ChainOfThoughtHeader />
        </ChainOfThought>
        <PromptInputProvider>
          <PromptInputAttachment
            data={{
              id: 'image',
              type: 'file',
              mediaType: 'image/png',
              url: 'https://example.com/image.png',
            }}
          />
          <PromptInputAttachment
            data={{
              id: 'file',
              type: 'file',
              mediaType: 'text/plain',
              url: 'data:text/plain;base64,',
            }}
          />
        </PromptInputProvider>
      </>
    )

    expect(screen.getByText('思维链')).toBeInTheDocument()
    expect(screen.getByText('图片')).toBeInTheDocument()
    expect(screen.getByAltText('图片')).toBeInTheDocument()
    expect(screen.getByText('附件')).toBeInTheDocument()
  })
})
