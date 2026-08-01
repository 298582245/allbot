import assert from 'node:assert/strict'
import test from 'node:test'

import { formatMarkdownInline, formatTextLinks } from './webChatText.js'

test('formatTextLinks converts every complete URL', () => {
  const first = 'https://example.com/path?a=1&b=2'
  const second = 'http://sub.example.org/#section'
  const actual = formatTextLinks(`入口 ${first}\n备用 ${second}`)

  assert.equal(actual, `入口 <a href="https://example.com/path?a=1&amp;b=2" target="_blank" rel="noopener noreferrer">https://example.com/path?a=1&amp;b=2</a>\n备用 <a href="http://sub.example.org/#section" target="_blank" rel="noopener noreferrer">http://sub.example.org/#section</a>`)
})

test('formatTextLinks stops only at configured delimiters', () => {
  const actual = formatTextLinks(`https://example.com/a,b。c <https://second.example.com/q> "https://third.example.com"`)

  assert.match(actual, /href="https:\/\/example\.com\/a,b。c"/)
  assert.match(actual, /&lt;<a href="https:\/\/second\.example\.com\/q"[^>]*>https:\/\/second\.example\.com\/q<\/a>&gt;/)
  assert.match(actual, /&quot;<a href="https:\/\/third\.example\.com"[^>]*>https:\/\/third\.example\.com<\/a>&quot;/)
})

test('formatTextLinks escapes non-link HTML', () => {
  const actual = formatTextLinks(`<script>alert('x')</script> https://example.com/?a=1&b=2`)

  assert.doesNotMatch(actual, /<script>/)
  assert.match(actual, /&lt;script&gt;alert\(&#39;x&#39;\)&lt;\/script&gt;/)
  assert.match(actual, /href="https:\/\/example\.com\/\?a=1&amp;b=2"/)
})

test('formatMarkdownInline supports explicit and automatic links together', () => {
  const actual = formatMarkdownInline('文档 [官网](https://example.com/docs) 镜像 https://mirror.example.com/a')

  assert.equal(actual, '文档 <a href="https://example.com/docs" target="_blank" rel="noopener noreferrer">官网</a> 镜像 <a href="https://mirror.example.com/a" target="_blank" rel="noopener noreferrer">https://mirror.example.com/a</a>')
})

test('formatMarkdownInline does not create nested anchors', () => {
  const actual = formatMarkdownInline('[https://example.com](https://example.org)')
  const codeLabel = formatMarkdownInline('[`https://label.example`](https://target.example)')

  assert.equal(actual, '<a href="https://example.org" target="_blank" rel="noopener noreferrer">https://example.com</a>')
  assert.equal((actual.match(/<a /g) || []).length, 1)
  assert.equal(codeLabel, '<a href="https://target.example" target="_blank" rel="noopener noreferrer"><code>https://label.example</code></a>')
  assert.equal((codeLabel.match(/<a /g) || []).length, 1)
})

test('formatMarkdownInline preserves markup around automatic links', () => {
  const actual = formatMarkdownInline('`https://code.example.com` **https://strong.example.com** *https://em.example.com*')

  assert.match(actual, /^<code><a href="https:\/\/code\.example\.com"/)
  assert.match(actual, /<strong><a href="https:\/\/strong\.example\.com"/)
  assert.match(actual, /<em><a href="https:\/\/em\.example\.com"/)
  assert.doesNotMatch(actual, /href="[^"]*[\*`]"/)
})

test('formatMarkdownInline preserves balanced parentheses in explicit links', () => {
  const actual = formatMarkdownInline('[示例](https://example.com/a_(b))')

  assert.equal(actual, '<a href="https://example.com/a_(b)" target="_blank" rel="noopener noreferrer">示例</a>')
})

test('formatMarkdownInline escapes HTML in explicit link text', () => {
  const actual = formatMarkdownInline('[<img src=x onerror=alert(1)>](https://example.com)')

  assert.doesNotMatch(actual, /<img/)
  assert.match(actual, /&lt;img src=x onerror=alert\(1\)&gt;/)
})
