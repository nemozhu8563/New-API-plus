'use client'

import { memo, useMemo } from 'react'
import { getMarkdown, parseMarkdownToStructure } from 'stream-markdown-parser'

import { cn } from '@/lib/utils'

import { getMarkdownContent, parseResponseContent } from './response-content'
import { renderChildren, renderFootnotes } from './response-renderer'
import type { ResponseProps } from './response-types'

const markdown = getMarkdown('new-api-response')
const MAX_PARSED_MARKDOWN_CHARS = 20_000

export const Response = memo((props: ResponseProps) => {
  const content = getMarkdownContent(props.children)
  const shouldParseMarkdown = content.length <= MAX_PARSED_MARKDOWN_CHARS
  const nodes = useMemo(() => {
    if (!shouldParseMarkdown) {
      return []
    }

    return parseMarkdownToStructure(content, markdown, {
      final: props.final ?? true,
      validateLink: markdown.options.validateLink,
    })
  }, [content, props.final, shouldParseMarkdown])
  const parsedContent = useMemo(() => parseResponseContent(nodes), [nodes])
  const renderedContent =
    parsedContent.bodyNodes.length > 0
      ? renderChildren(parsedContent.bodyNodes)
      : content
  const footnotes = renderFootnotes(parsedContent.footnotes)

  return (
    <div
      className={cn(
        'size-full min-w-0 text-pretty [&>*:first-child]:mt-0 [&>*:last-child]:mb-0',
        props.className
      )}
    >
      {renderedContent}
      {footnotes}
    </div>
  )
})

Response.displayName = 'Response'
