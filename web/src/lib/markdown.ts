import DOMPurify from 'dompurify'
import MarkdownIt from 'markdown-it'

/**
 * Markdown of the agent (questions and summaries, D-088) to HTML the page can insert. Raw HTML in
 * the markdown is never parsed (`html: false`: it is shown as text), the result is sanitised by
 * DOMPurify, and every link opens in a new tab without a referrer and without reaching back to
 * this window. Only agent text goes through here; the marketer's text stays plain.
 */
const md = new MarkdownIt({ html: false, linkify: false, breaks: true, typographer: false })

const sanitizer = DOMPurify()

sanitizer.addHook('afterSanitizeAttributes', (node) => {
  if (node.tagName === 'A') {
    node.setAttribute('target', '_blank')
    node.setAttribute('rel', 'noopener noreferrer')
  }
})

/** Tags that make sense in a chat message: inline text, lists, quotes, code, headings. */
const ALLOWED_TAGS = [
  'p',
  'br',
  'strong',
  'em',
  'b',
  'i',
  's',
  'del',
  'code',
  'pre',
  'blockquote',
  'ul',
  'ol',
  'li',
  'a',
  'h1',
  'h2',
  'h3',
  'h4',
  'h5',
  'h6',
  'hr',
  'table',
  'thead',
  'tbody',
  'tr',
  'th',
  'td',
]

export function renderMarkdown(source: string): string {
  return sanitizer.sanitize(md.render(source), {
    ALLOWED_TAGS,
    ALLOWED_ATTR: ['href', 'title', 'target', 'rel', 'start'],
    // Only web and mail links; `javascript:` and `data:` fall out here.
    ALLOWED_URI_REGEXP: /^(?:https?:|mailto:)/i,
  })
}
