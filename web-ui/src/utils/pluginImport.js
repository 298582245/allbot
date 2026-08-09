const MANIFEST_MAX_BYTES = 1024 * 1024
const ARCHIVE_MAX_BYTES = 64 * 1024 * 1024
const SUPPORTED_RUNTIMES = {
  nodejs: 'main.js',
  python: 'main.py'
}

export function normalizeImportPath(value) {
  const raw = String(value ?? '')
  if (!raw || raw.includes('\0')) throw new Error('文件路径不能为空或包含 NUL 字符')

  const path = raw.replaceAll('\\', '/')
  if (path.startsWith('/') || /^[a-zA-Z]:($|\/)/.test(path)) {
    throw new Error(`不允许绝对路径：${raw}`)
  }

  const segments = []
  for (const segment of path.split('/')) {
    if (!segment || segment === '.') continue
    if (segment === '..') throw new Error(`不允许上级目录路径：${raw}`)
    segments.push(segment)
  }
  if (!segments.length) throw new Error('文件路径不能为空')
  return segments.join('/')
}

export function derivePluginId(value) {
  return String(value || '')
    .trim()
    .replace(/\.zip$/i, '')
    .toLowerCase()
    .replace(/[^a-z0-9_-]+/g, '_')
    .replace(/^[_-]+|[_-]+$/g, '')
}

export async function inspectPluginDirectory(fileList) {
  const sourceFiles = Array.from(fileList || [])
  const diagnostics = []
  const normalized = []

  for (const file of sourceFiles) {
    const rawPath = file.webkitRelativePath || file.name
    try {
      normalized.push({ file, rawPath, path: normalizeImportPath(rawPath), size: Number(file.size || 0) })
    } catch (error) {
      diagnostics.push(createDiagnostic('error', error.message, rawPath || file.name || '未知文件'))
    }
  }

  let wrapperName = ''
  if (normalized.length && normalized.every(item => item.path.includes('/'))) {
    const firstSegments = new Set(normalized.map(item => item.path.split('/')[0]))
    if (firstSegments.size === 1) {
      wrapperName = normalized[0].path.split('/')[0]
      normalized.forEach(item => {
        item.path = item.path.slice(wrapperName.length + 1)
      })
    }
  }

  appendPathConflictDiagnostics(normalized, diagnostics)

  const manifestItem = normalized.find(item => item.path === 'plugin.json')
  let manifest = null
  if (!manifestItem) {
    diagnostics.push(createDiagnostic('error', '插件根目录缺少 plugin.json', 'plugin.json'))
  } else if (manifestItem.size > MANIFEST_MAX_BYTES) {
    diagnostics.push(createDiagnostic('error', 'plugin.json 不能超过 1 MiB', 'plugin.json'))
  } else {
    try {
      const content = await manifestItem.file.text()
      manifest = JSON.parse(content)
      if (!manifest || typeof manifest !== 'object' || Array.isArray(manifest)) {
        manifest = null
        diagnostics.push(createDiagnostic('error', 'plugin.json 必须是 JSON 对象', 'plugin.json'))
      }
    } catch (error) {
      diagnostics.push(createDiagnostic('error', `plugin.json 解析失败：${error.message}`, 'plugin.json'))
    }
  }

  if (manifest) appendManifestDiagnostics(manifest, normalized, diagnostics)
  if (!sourceFiles.length) diagnostics.push(createDiagnostic('error', '请选择包含插件文件的文件夹'))
  if (!diagnostics.some(item => item.level === 'error')) {
    diagnostics.push(createDiagnostic('success', '前端检查通过，导入时仍会由服务端完整校验'))
  }

  const sourceName = wrapperName || '所选文件夹'
  return {
    sourceName,
    pluginId: derivePluginId(wrapperName),
    files: normalized,
    totalSize: normalized.reduce((total, item) => total + item.size, 0),
    manifest,
    diagnostics,
    valid: !diagnostics.some(item => item.level === 'error')
  }
}

export function inspectPluginArchive(file) {
  const diagnostics = []
  if (!file) diagnostics.push(createDiagnostic('error', '请选择 ZIP 压缩包'))
  if (file && !/\.zip$/i.test(file.name || '')) {
    diagnostics.push(createDiagnostic('error', '仅支持 .zip 格式的插件压缩包', file.name))
  }
  if (file && Number(file.size || 0) > ARCHIVE_MAX_BYTES) {
    diagnostics.push(createDiagnostic('error', 'ZIP 压缩包不能超过 64 MiB', file.name))
  }
  if (file && !diagnostics.length) {
    diagnostics.push(createDiagnostic('info', 'ZIP 包内目录和 plugin.json 将由服务端校验', file.name))
  }
  return {
    sourceName: file?.name || '',
    pluginId: derivePluginId(file?.name),
    files: file ? [{ file, rawPath: file.name, path: file.name, size: Number(file.size || 0) }] : [],
    totalSize: Number(file?.size || 0),
    manifest: null,
    diagnostics,
    valid: Boolean(file) && !diagnostics.some(item => item.level === 'error')
  }
}

function appendPathConflictDiagnostics(files, diagnostics) {
  const exactPaths = new Set()
  const foldedPaths = new Map()
  const filePaths = new Map()

  for (const item of files) {
    if (exactPaths.has(item.path)) {
      diagnostics.push(createDiagnostic('error', `存在重复文件路径：${item.path}`, item.path))
      continue
    }
    exactPaths.add(item.path)

    const folded = item.path.toLocaleLowerCase('en-US')
    const previous = foldedPaths.get(folded)
    if (previous && previous !== item.path) {
      diagnostics.push(createDiagnostic('error', `文件路径存在大小写冲突：${previous} / ${item.path}`, item.path))
    } else {
      foldedPaths.set(folded, item.path)
    }
    filePaths.set(folded, item.path)
  }

  for (const item of files) {
    const segments = item.path.split('/')
    for (let index = 1; index < segments.length; index += 1) {
      const parent = segments.slice(0, index).join('/')
      const matchedFile = filePaths.get(parent.toLocaleLowerCase('en-US'))
      if (matchedFile) {
        diagnostics.push(createDiagnostic('error', `文件与目录路径冲突：${matchedFile} / ${item.path}`, item.path))
        break
      }
    }
  }
}

function appendManifestDiagnostics(manifest, files, diagnostics) {
  const runtime = String(manifest.runtime || '').trim().toLowerCase()
  const expectedEntry = SUPPORTED_RUNTIMES[runtime]
  if (!expectedEntry) {
    diagnostics.push(createDiagnostic('error', 'plugin.json 的 runtime 仅支持 nodejs 或 python', 'plugin.json'))
    return
  }

  const entry = String(manifest.entry || '').trim().replaceAll('\\', '/')
  if (entry !== expectedEntry) {
    diagnostics.push(createDiagnostic('error', `${runtime} 插件的 entry 必须为 ${expectedEntry}`, 'plugin.json'))
  }

  const paths = new Set(files.map(item => item.path))
  if (!paths.has(expectedEntry)) {
    diagnostics.push(createDiagnostic('error', `插件根目录缺少入口文件 ${expectedEntry}`, expectedEntry))
  }
  if (paths.has('main.js') && paths.has('main.py')) {
    diagnostics.push(createDiagnostic('warning', `同时检测到 main.js 和 main.py，将使用 runtime 对应的 ${expectedEntry}`))
  }
}

function createDiagnostic(level, message, path = '') {
  return { level, message, path }
}
