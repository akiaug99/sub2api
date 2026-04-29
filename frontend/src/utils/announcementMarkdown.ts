import { marked } from 'marked'
import DOMPurify from 'dompurify'

marked.setOptions({
  breaks: true,
  gfm: true,
})

export interface TextInsertResult {
  nextValue: string
  nextCursorStart: number
  nextCursorEnd: number
}

function normalizeAltText(value: string): string {
  return value.replace(/[\]\r\n]+/g, ' ').trim()
}

export function buildAnnouncementImageMarkdown(imageSrc: string, altText: string = ''): string {
  const safeAltText = normalizeAltText(altText)
  return `![${safeAltText}](${imageSrc})`
}

export function insertTextAtSelection(
  currentValue: string,
  insertText: string,
  selectionStart?: number | null,
  selectionEnd?: number | null,
): TextInsertResult {
  const start = Math.max(0, Math.min(selectionStart ?? currentValue.length, currentValue.length))
  const end = Math.max(start, Math.min(selectionEnd ?? start, currentValue.length))

  const prefix = currentValue.slice(0, start)
  const suffix = currentValue.slice(end)

  let leadingBreak = ''
  if (prefix.length > 0) {
    leadingBreak = prefix.endsWith('\n\n') ? '' : prefix.endsWith('\n') ? '\n' : '\n\n'
  }

  let trailingBreak = ''
  if (suffix.length > 0) {
    trailingBreak = suffix.startsWith('\n\n') ? '' : suffix.startsWith('\n') ? '\n' : '\n\n'
  }

  const insertion = `${leadingBreak}${insertText}${trailingBreak}`
  const nextValue = `${prefix}${insertion}${suffix}`
  const nextCursor = prefix.length + insertion.length

  return {
    nextValue,
    nextCursorStart: nextCursor,
    nextCursorEnd: nextCursor,
  }
}

export function renderAnnouncementMarkdown(content: string): string {
  if (!content) return ''
  const html = marked.parse(content) as string
  return DOMPurify.sanitize(html, {
    ADD_DATA_URI_TAGS: ['img'],
  })
}
