import { describe, expect, it } from 'vitest'

import { renderMarkdown } from '../markdown'

/** The rendered HTML as DOM, to look at elements and attributes rather than text. */
const dom = (source: string) =>
  new DOMParser().parseFromString(`<body>${renderMarkdown(source)}</body>`, 'text/html').body

describe('renderMarkdown', () => {
  it('renders emphasis, lists and line breaks', () => {
    const html = renderMarkdown('**Kitle**: 48 kişi\n\n- İstanbul\n- Ankara\n\nsatır\nsonu')
    expect(html).toContain('<strong>Kitle</strong>')
    expect(html).toMatch(/<ul>\s*<li>İstanbul<\/li>\s*<li>Ankara<\/li>\s*<\/ul>/)
    expect(html).toContain('satır<br>')
  })

  it('shows raw HTML as text instead of parsing it', () => {
    const html = renderMarkdown('<img src=x onerror=alert(1)> and <b>bold</b>')
    expect(html).not.toContain('<img')
    expect(html).not.toContain('<b>')
    expect(html).toContain('&lt;img src=x onerror=alert(1)&gt;')
    expect(html).toContain('&lt;b&gt;bold&lt;/b&gt;')
  })

  it('drops scripts, event handlers and javascript: links', () => {
    expect(dom('<script>alert(1)</script>').querySelector('script')).toBeNull()
    expect(dom('<a href="javascript:alert(1)" onclick="x()">a</a>').querySelector('a')).toBeNull()
    // markdown-it refuses the link itself and leaves the source as text; nothing is clickable.
    const link = dom('[tıkla](javascript:alert(1))')
    expect(link.querySelector('a')).toBeNull()
    expect(link.textContent).toContain('tıkla')
    const data = dom('[x](data:text/html;base64,PHNjcmlwdD4=)')
    expect(data.querySelector('a[href]')).toBeNull()
    for (const a of dom('[a](https://x.example) [b](JAVASCRIPT:alert(1))').querySelectorAll('a')) {
      expect(a.getAttribute('href') ?? '').not.toMatch(/^javascript:/i)
    }
  })

  it('opens links in a new tab without a referrer', () => {
    const html = renderMarkdown('[shop](https://shop.example.com/?a=1)')
    expect(html).toContain('href="https://shop.example.com/?a=1"')
    expect(html).toContain('target="_blank"')
    expect(html).toContain('rel="noopener noreferrer"')
  })

  it('keeps nothing but the allowed tags and attributes', () => {
    const body = dom('# Başlık\n\n<p style="color:red" onclick="x()">a</p>\n\n`kod`')
    expect(body.querySelector('h1')!.textContent).toBe('Başlık')
    expect(body.querySelector('[style], [onclick]')).toBeNull()
    expect(body.querySelector('code')!.textContent).toBe('kod')
    // The raw paragraph tag was shown as text, not parsed.
    expect(body.textContent).toContain('<p style="color:red" onclick="x()">a</p>')
  })
})
