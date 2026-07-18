'use strict'

function response(status, retcode, errmsg, data) {
  return { status, retcode, errmsg, data }
}

function readSingleQuery(query, name) {
  const value = query && query[name]
  if (Array.isArray(value)) {
    throw new Error(`${name} 不能重复提供`)
  }
  return value
}

function readRequiredQuery(query, name) {
  const value = readSingleQuery(query, name)
  if (typeof value !== 'string' || value.trim() === '') {
    throw new Error(`${name} 必须是非空字符串`)
  }
  return value.trim()
}

function readOptionalQuery(query, name) {
  const value = readSingleQuery(query, name)
  if (value === undefined || value === null || value === '') {
    return ''
  }
  if (typeof value !== 'string' || value.trim() === '') {
    throw new Error(`${name} 必须是非空字符串`)
  }
  return value.trim()
}

const HTTP_URL_PATTERN = /https?:\/\/[^\s<>"']+/giu
const HAS_HTTP_URL_PATTERN = /https?:\/\/[^\s<>"']+/iu
const CHINESE_FIELD_PATTERN = /^([\p{Script=Han}][\p{Script=Han}A-Za-z0-9（）()·/_-]{0,19})(\s*[：:])(\s*)(.*)$/u
const ADDRESS_FIELDS = new Set(['地址', '链接', '网址', 'URL', '访问地址', '下载地址', '详情地址'])

function escapeMarkdownText(value) {
  return value.replace(/[\\`*_[\]{}()<>#+\-.!|~]/g, '\\$&')
}

function escapeMarkdownLinkDestination(url) {
  return url
    .replace(/\\/g, '%5C')
    .replace(/\(/g, '%28')
    .replace(/\)/g, '%29')
    .replace(/</g, '%3C')
    .replace(/>/g, '%3E')
}

function trimUrlTrailingPunctuation(value) {
  let url = value.replace(/[，。；！？、.,;!]+$/u, '')
  while (url.endsWith(')')) {
    const openCount = (url.match(/\(/g) || []).length
    const closeCount = (url.match(/\)/g) || []).length
    if (closeCount <= openCount) break
    url = url.slice(0, -1)
  }
  return url
}

function formatAddressValue(value) {
  let result = ''
  let offset = 0

  for (const match of value.matchAll(HTTP_URL_PATTERN)) {
    const matchedUrl = match[0]
    const url = trimUrlTrailingPunctuation(matchedUrl)
    const trailing = matchedUrl.slice(url.length)
    result += escapeMarkdownText(value.slice(offset, match.index))
    result += `[${escapeMarkdownText(url)}](${escapeMarkdownLinkDestination(url)})`
    result += escapeMarkdownText(trailing)
    offset = match.index + matchedUrl.length
  }

  return result + escapeMarkdownText(value.slice(offset))
}

function messageToMarkdown(message) {
  let titleAdded = false

  return message.split(/\r?\n/).map((line) => {
    const content = line.trim()
    if (content === '') {
      return ''
    }

    const field = content.match(CHINESE_FIELD_PATTERN)
    if (field) {
      const [, label, separator, spacing, value] = field
      const formattedValue = ADDRESS_FIELDS.has(label)
        ? formatAddressValue(value)
        : escapeMarkdownText(value)
      return `**${escapeMarkdownText(label)}${separator.trim()}**${spacing}${formattedValue}`
    }

    const escaped = escapeMarkdownText(content)
    if (!titleAdded) {
      titleAdded = true
      return `### ${escaped}`
    }
    return escaped
  }).join('\n')
}

module.exports.action = async function action(ctx, req, res) {
  try {
    const message = req.body && req.body.message
    if (typeof message !== 'string' || message.trim() === '') {
      throw new Error('message 必须是非空字符串')
    }

    const platform = readOptionalQuery(req.query, 'platform')
    const adapterId = readRequiredQuery(req.query, 'adapter_id')
    const groupId = readOptionalQuery(req.query, 'group_id')
    const userId = readOptionalQuery(req.query, 'user_id')

    if (Boolean(groupId) === Boolean(userId)) {
      throw new Error('group_id 和 user_id 必须且只能提供一个')
    }

    const target = groupId ? { groupId } : { userId }
    const sendOptions = {
      adapterId,
      ...target
    }
    if (platform) {
      sendOptions.platform = platform
    }

    if (HAS_HTTP_URL_PATTERN.test(message)) {
      await ctx.sendRichMessage({
        ...sendOptions,
        parts: [{ type: 'markdown', markdown: messageToMarkdown(message) }],
        prefer: 'markdown',
        fallbackText: message
      })
    } else {
      await ctx.push({ ...sendOptions, content: message })
    }

    res.status(200).json(response('ok', 0, '', {
      ...(platform ? { platform } : {}),
      adapterId,
      ...target
    }))
  } catch (error) {
    const errmsg = error instanceof Error ? error.message : String(error)
    res.status(200).json(response('failed', 100, errmsg, null))
  }
}
