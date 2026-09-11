import { render, screen } from '@testing-library/react'
import { createInstance } from 'i18next'
import { I18nextProvider, initReactI18next } from 'react-i18next'
import { beforeAll, describe, expect, test } from 'vitest'

import { DateTimePicker } from '@/components/datetime-picker'
import { JsonEditor } from '@/components/json-editor'
import { PasswordInput } from '@/components/password-input'
import { TagInput } from '@/components/tag-input'
import { BreadcrumbEllipsis } from '@/components/ui/breadcrumb'
import { ComboboxInput } from '@/components/ui/combobox-input'
import { CommandDialog } from '@/components/ui/command'
import { Dialog, DialogContent, DialogTitle } from '@/components/ui/dialog'
import {
  Pagination,
  PaginationEllipsis,
  PaginationNext,
  PaginationPrevious,
} from '@/components/ui/pagination'
import { Spinner } from '@/components/ui/spinner'

const i18n = createInstance()

beforeAll(async () => {
  await i18n.use(initReactI18next).init({
    lng: 'zh',
    resources: {
      zh: {
        translation: {
          Clear: '清除',
          Close: '关闭',
          'Command Palette': '命令面板',
          'Delete row': '删除行',
          'Go to next page': '转到下一页',
          'Go to previous page': '转到上一页',
          Loading: '加载中',
          More: '更多',
          'More pages': '更多页面',
          Next: '下一页',
          Pagination: '分页',
          Previous: '上一页',
          'Remove tag': '移除标签',
          'Search for a command to run...': '搜索要运行的命令……',
          'Select or type...': '选择或输入……',
          'Toggle password visibility': '切换密码可见性',
        },
      },
    },
  })
})

function renderLocalized(ui: React.ReactNode) {
  return render(<I18nextProvider i18n={i18n}>{ui}</I18nextProvider>)
}

describe('shared component localization', () => {
  test('uses the active locale for common accessible names', () => {
    renderLocalized(
      <>
        <PasswordInput />
        <TagInput value={['alpha']} onChange={() => undefined} />
        <Spinner />
        <DateTimePicker value={new Date(2026, 7, 26)} />
        <JsonEditor value='{"key":"value"}' onChange={() => undefined} />
      </>
    )

    expect(
      screen.getByRole('button', { name: '切换密码可见性' })
    ).toBeInTheDocument()
    expect(screen.getByRole('button', { name: '移除标签' })).toBeInTheDocument()
    expect(screen.getByRole('status', { name: '加载中' })).toBeInTheDocument()
    expect(screen.getByRole('button', { name: '清除' })).toBeInTheDocument()
    expect(screen.getByRole('button', { name: '删除行' })).toBeInTheDocument()
  })

  test('localizes pagination navigation and fallback button text', () => {
    renderLocalized(
      <Pagination>
        <PaginationPrevious href='#previous' />
        <PaginationEllipsis />
        <PaginationNext href='#next' />
      </Pagination>
    )

    expect(screen.getByRole('navigation', { name: '分页' })).toBeInTheDocument()
    expect(
      screen.getByRole('button', { name: '转到上一页' })
    ).toHaveTextContent('上一页')
    expect(
      screen.getByRole('button', { name: '转到下一页' })
    ).toHaveTextContent('下一页')
    expect(screen.getByText('更多页面')).toBeInTheDocument()
  })

  test('localizes the default dialog close control', () => {
    renderLocalized(
      <Dialog open>
        <DialogContent>
          <DialogTitle>标题</DialogTitle>
        </DialogContent>
      </Dialog>
    )

    expect(screen.getByRole('button', { name: '关闭' })).toBeInTheDocument()
  })

  test('localizes shared component defaults', () => {
    renderLocalized(
      <>
        <CommandDialog open>
          <div>命令内容</div>
        </CommandDialog>
        <BreadcrumbEllipsis />
        <ComboboxInput options={[]} onValueChange={() => undefined} />
      </>
    )

    expect(
      screen.getByRole('dialog', { name: '命令面板' })
    ).toHaveAccessibleDescription('搜索要运行的命令……')
    expect(screen.getByText('更多')).toBeInTheDocument()
    expect(screen.getByPlaceholderText('选择或输入……')).toBeInTheDocument()
  })
})
