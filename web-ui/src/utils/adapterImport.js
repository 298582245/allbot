export function normalizeAdapterImportPath(value) {
  const raw = String(value ?? '')
  if (!raw || raw.includes('\0')) throw new Error('文件路径不能为空或包含 NUL 字符')
  const path = raw.replaceAll('\\', '/')
  if (path.startsWith('/') || /^[a-zA-Z]:($|\/)/.test(path)) throw new Error(`不允许绝对路径：${raw}`)
  const segments = []
  for (const segment of path.split('/')) {
    if (!segment || segment === '.') continue
    if (segment === '..') throw new Error(`不允许上级目录路径：${raw}`)
    segments.push(segment)
  }
  if (!segments.length) throw new Error('文件路径不能为空')
  return segments.join('/')
}

export async function inspectAdapterDirectory(fileList) {
  const files = Array.from(fileList || [])
  const diagnostics = []
  const normalized = []
  let sourceName = ''
  for (const file of files) {
    const rawPath = file.webkitRelativePath || file.name
    try {
      normalized.push({ file, path: normalizeAdapterImportPath(rawPath), size: Number(file.size || 0) })
    } catch (error) {
      diagnostics.push({ level: 'error', message: error.message, path: rawPath || file.name || '' })
    }
  }
  if (normalized.length && normalized.every(item => item.path.includes('/'))) {
    const wrappers = new Set(normalized.map(item => item.path.split('/')[0]))
    if (wrappers.size === 1) {
      sourceName = normalized[0].path.split('/')[0]
      normalized.forEach(item => { item.path = item.path.slice(item.path.indexOf('/') + 1) })
    }
  }
  const manifestFile = normalized.find(item => item.path === 'adapter.json')
  let manifest = null
  if (!manifestFile) diagnostics.push({ level: 'error', message: '适配器根目录缺少 adapter.json', path: 'adapter.json' })
  else {
    try {
      manifest = JSON.parse(await manifestFile.file.text())
      if (!manifest || typeof manifest !== 'object' || Array.isArray(manifest)) throw new Error('adapter.json 必须是 JSON 对象')
    } catch (error) {
      manifest = null
      diagnostics.push({ level: 'error', message: `adapter.json 解析失败：${error.message}`, path: 'adapter.json' })
    }
  }
  if (manifest) {
    const entry = String(manifest.entry || '').replaceAll('\\', '/')
    if (!entry) diagnostics.push({ level: 'error', message: 'adapter.json 缺少入口文件 entry', path: 'adapter.json' })
    else if (!normalized.some(item => item.path === entry)) diagnostics.push({ level: 'error', message: `适配器目录缺少入口文件 ${entry}`, path: entry })
  }
  if (!files.length) diagnostics.push({ level: 'error', message: '请选择包含适配器文件的文件夹' })
  if (!diagnostics.some(item => item.level === 'error')) diagnostics.push({ level: 'success', message: '已找到 adapter.json 和入口文件' })
  return {
    sourceName: sourceName || '所选文件夹',
    files: normalized,
    totalSize: normalized.reduce((total, item) => total + item.size, 0),
    manifest,
    diagnostics,
    valid: !diagnostics.some(item => item.level === 'error')
  }
}

export function inspectAdapterArchive(file) {
  const diagnostics = []
  if (!file) diagnostics.push({ level: 'error', message: '请选择 ZIP 压缩包' })
  else if (!/\.zip$/i.test(file.name || '')) diagnostics.push({ level: 'error', message: '仅支持 .zip 格式的适配器压缩包', path: file.name })
  else if (Number(file.size || 0) > 64 * 1024 * 1024) diagnostics.push({ level: 'error', message: 'ZIP 压缩包不能超过 64 MiB', path: file.name })
  else diagnostics.push({ level: 'info', message: 'ZIP 包内 adapter.json 和入口文件将由服务端校验', path: file.name })
  return { sourceName: file?.name || '', files: [], totalSize: Number(file?.size || 0), manifest: null, diagnostics, valid: Boolean(file) && !diagnostics.some(item => item.level === 'error') }
}
