import type { ReactNode } from 'react'
import {
  shouldOpenLinkInNewTab,
  type ImageNode,
  type LinkNode,
  type TextNode,
} from 'stream-markdown-parser'

import { ResponseImage } from './response-renderer-image'
import type { RenderChildren } from './response-types'

export function renderTextNode(node: TextNode): ReactNode {
  return node.content
}

export function renderLink(
  node: LinkNode,
  key: string,
  renderChildren: RenderChildren
): ReactNode {
  const opensInNewTab = shouldOpenLinkInNewTab(node.href)
  const rel = opensInNewTab ? 'noreferrer noopener' : undefined
  const target = opensInNewTab ? '_blank' : undefined

  return (
    <a
      className='text-primary underline-offset-4 hover:underline'
      href={node.href}
      key={key}
      rel={rel}
      target={target}
      title={node.title ?? undefined}
    >
      {renderChildren(node.children)}
    </a>
  )
}

export function renderImage(node: ImageNode, key: string): ReactNode {
  return <ResponseImage key={key} node={node} />
}
