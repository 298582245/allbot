const webURLPattern = /https?:\/\/[\w\-]+(\.[\w\-]+)+[/#?]?.*?(?=[\s<>"']|$)/g

export function formatTextLinks(value) {
  return linkifyText(String(value || ''), escapeHTML)
}

export function formatMarkdownInline(value) {
  return renderMarkdownSegment(String(value || ''), true)
}

export function safeLinkURL(value) {
  const url = String(value || '').trim().toLowerCase()
  return url.startsWith('http://') || url.startsWith('https://') || url.startsWith('/api/open/images/')
}

export function escapeHTML(value) {
  return String(value || '')
    .replace(/&/g, '&amp;')
    .replace(/</g, '&lt;')
    .replace(/>/g, '&gt;')
    .replace(/"/g, '&quot;')
    .replace(/'/g, '&#39;')
}

export function escapeAttribute(value) {
  return escapeHTML(value).replace(/`/g, '&#96;')
}

function renderMarkdownSegment(source, allowLinks) {
  const html = []
  let plainStart = 0
  let index = 0

  const flushPlain = (end) => {
    if (end > plainStart) html.push(escapeHTML(source.slice(plainStart, end)))
  }

  while (index < source.length) {
    const markdownLink = source[index] === '[' ? parseMarkdownLink(source, index) : null
    if (markdownLink) {
      flushPlain(index)
      if (safeLinkURL(markdownLink.href)) {
        html.push(`<a href="${escapeAttribute(markdownLink.href)}" target="_blank" rel="noopener noreferrer">${renderMarkdownSegment(markdownLink.text, false)}</a>`)
      } else {
        html.push(escapeHTML(markdownLink.raw))
      }
      index = markdownLink.end
      plainStart = index
      continue
    }

    const markdownToken = parseMarkdownToken(source, index)
    if (markdownToken) {
      flushPlain(index)
      const content = markdownToken.type === 'code'
        ? (allowLinks ? linkifyText(markdownToken.text, escapeHTML) : escapeHTML(markdownToken.text))
        : renderMarkdownSegment(markdownToken.text, allowLinks)
      html.push(`<${markdownToken.type}>${content}</${markdownToken.type}>`)
      index = markdownToken.end
      plainStart = index
      continue
    }

    const url = allowLinks ? matchURLAt(source, index) : ''
    if (url) {
      flushPlain(index)
      html.push(createLink(url, url))
      index += url.length
      plainStart = index
      continue
    }
    index += 1
  }

  flushPlain(source.length)
  return html.join('')
}

function parseMarkdownLink(source, start) {
  const labelEnd = source.indexOf('](', start + 1)
  if (labelEnd < 0) return null

  const hrefStart = labelEnd + 2
  let depth = 0
  for (let index = hrefStart; index < source.length; index += 1) {
    const char = source[index]
    if (/\s/.test(char)) return null
    if (char === '(') {
      depth += 1
    } else if (char === ')') {
      if (depth > 0) {
        depth -= 1
      } else {
        const end = index + 1
        return {
          text: source.slice(start + 1, labelEnd),
          href: source.slice(hrefStart, index),
          raw: source.slice(start, end),
          end
        }
      }
    }
  }
  return null
}

function parseMarkdownToken(source, start) {
  const tokens = [
    { marker: '`', type: 'code' },
    { marker: '**', type: 'strong' },
    { marker: '*', type: 'em' }
  ]
  for (const token of tokens) {
    if (!source.startsWith(token.marker, start)) continue
    const contentStart = start + token.marker.length
    const contentEnd = source.indexOf(token.marker, contentStart)
    if (contentEnd <= contentStart) return null
    return {
      type: token.type,
      text: source.slice(contentStart, contentEnd),
      end: contentEnd + token.marker.length
    }
  }
  return null
}

function linkifyText(value, formatPlainText) {
  const source = String(value || '')
  const html = []
  let lastIndex = 0

  webURLPattern.lastIndex = 0
  for (const match of source.matchAll(webURLPattern)) {
    const url = match[0]
    html.push(formatPlainText(source.slice(lastIndex, match.index)))
    html.push(createLink(url, url))
    lastIndex = match.index + url.length
  }
  html.push(formatPlainText(source.slice(lastIndex)))
  return html.join('')
}

function matchURLAt(source, index) {
  webURLPattern.lastIndex = index
  const match = webURLPattern.exec(source)
  return match?.index === index ? match[0] : ''
}

function createLink(href, text) {
  return `<a href="${escapeAttribute(href)}" target="_blank" rel="noopener noreferrer">${escapeHTML(text)}</a>`
}
