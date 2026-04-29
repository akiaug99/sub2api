import { describe, expect, it } from 'vitest'

import {
  buildAnnouncementImageMarkdown,
  insertTextAtSelection,
  renderAnnouncementMarkdown,
} from '@/utils/announcementMarkdown'

describe('announcementMarkdown', () => {
  it('builds markdown image syntax with sanitized alt text', () => {
    expect(
      buildAnnouncementImageMarkdown('data:image/png;base64,abc', 'Banner ]\nImage'),
    ).toBe('![Banner  Image](data:image/png;base64,abc)')
  })

  it('inserts uploaded image markdown at the current selection with spacing', () => {
    const result = insertTextAtSelection(
      'First paragraph\nSecond paragraph',
      '![Image](data:image/png;base64,abc)',
      15,
      15,
    )

    expect(result.nextValue).toBe(
      'First paragraph\n\n![Image](data:image/png;base64,abc)\n\nSecond paragraph',
    )
    expect(
      result.nextValue.slice(result.nextCursorStart, result.nextValue.indexOf('Second paragraph')),
    ).toBe('\n')
    expect(result.nextCursorEnd).toBe(result.nextCursorStart)
  })

  it('preserves uploaded data-url images while sanitizing markdown output', () => {
    const html = renderAnnouncementMarkdown('![Banner](data:image/png;base64,abc)')

    expect(html).toContain('<img')
    expect(html).toContain('src="data:image/png;base64,abc"')
  })

  it('strips unsafe javascript urls from markdown output', () => {
    const html = renderAnnouncementMarkdown('[bad](javascript:alert(1))')

    expect(html).not.toContain('javascript:alert')
  })
})
