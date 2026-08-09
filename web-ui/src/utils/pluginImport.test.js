import assert from 'node:assert/strict'
import test from 'node:test'

import { derivePluginId, inspectPluginArchive, inspectPluginDirectory, normalizeImportPath } from './pluginImport.js'

function file(name, content = '', path = name) {
  return {
    name,
    size: Buffer.byteLength(content),
    webkitRelativePath: path,
    text: async () => content
  }
}

function manifest(runtime = 'nodejs', entry = runtime === 'python' ? 'main.py' : 'main.js') {
  return JSON.stringify({ name: '示例插件', version: '1.0.0', runtime, entry, trigger: '^测试$' })
}

function errors(result) {
  return result.diagnostics.filter(item => item.level === 'error').map(item => item.message)
}

test('normalizeImportPath normalizes separators and rejects unsafe paths', () => {
  assert.equal(normalizeImportPath('folder\\nested/./main.js'), 'folder/nested/main.js')
  assert.throws(() => normalizeImportPath('../main.js'), /上级目录/)
  assert.throws(() => normalizeImportPath('/main.js'), /绝对路径/)
  assert.throws(() => normalizeImportPath('C:\\main.js'), /绝对路径/)
  assert.throws(() => normalizeImportPath('main\0.js'), /NUL/)
})

test('inspectPluginDirectory accepts Node.js root with wrapper directory', async () => {
  const result = await inspectPluginDirectory([
    file('plugin.json', manifest(), 'demo-plugin/plugin.json'),
    file('main.js', 'module.exports = {}', 'demo-plugin/main.js'),
    file('README.md', '# demo', 'demo-plugin/README.md')
  ])
  assert.equal(result.valid, true)
  assert.equal(result.pluginId, 'demo-plugin')
  assert.deepEqual(result.files.map(item => item.path), ['plugin.json', 'main.js', 'README.md'])
})

test('inspectPluginDirectory accepts Python plugin without wrapper', async () => {
  const result = await inspectPluginDirectory([
    file('plugin.json', manifest('python'), 'plugin.json'),
    file('main.py', 'def main(): pass', 'main.py')
  ])
  assert.equal(result.valid, true)
})

test('inspectPluginDirectory reports missing manifest and entry', async () => {
  const missingManifest = await inspectPluginDirectory([file('README.md', 'help', 'demo/README.md')])
  assert.equal(missingManifest.valid, false)
  assert.ok(errors(missingManifest).some(message => message.includes('plugin.json')))

  const missingEntry = await inspectPluginDirectory([file('plugin.json', manifest()), file('README.md', 'help')])
  assert.equal(missingEntry.valid, false)
  assert.ok(errors(missingEntry).some(message => message.includes('入口')))
})

test('inspectPluginDirectory rejects duplicate, case-conflicting and file-directory paths', async () => {
  const result = await inspectPluginDirectory([
    file('plugin.json', manifest(), 'plugin.json'),
    file('main.js', '', 'main.js'),
    file('MAIN.JS', '', 'MAIN.JS'),
    file('config', '', 'config'),
    file('nested.js', '', 'config/nested.js'),
    file('duplicate.js', '', 'duplicate.js'),
    file('duplicate.js', '', 'duplicate.js')
  ])
  assert.equal(result.valid, false)
  assert.ok(errors(result).some(message => message.includes('大小写冲突')))
  assert.ok(errors(result).some(message => message.includes('重复文件路径')))
  assert.ok(errors(result).some(message => message.includes('文件与目录路径冲突')))
})

test('inspectPluginDirectory validates manifest JSON runtime and entry', async () => {
  const invalidJSON = await inspectPluginDirectory([file('plugin.json', '{oops'), file('main.js')])
  assert.ok(errors(invalidJSON).some(message => message.includes('解析失败')))

  const inconsistent = await inspectPluginDirectory([
    file('plugin.json', manifest('python', 'main.js')),
    file('main.js')
  ])
  assert.ok(errors(inconsistent).some(message => message.includes('entry 必须')))
  assert.ok(errors(inconsistent).some(message => message.includes('main.py')))

  const unsupported = await inspectPluginDirectory([
    file('plugin.json', manifest('ruby', 'main.rb')),
    file('main.rb')
  ])
  assert.ok(errors(unsupported).some(message => message.includes('仅支持')))
})

test('inspectPluginArchive only checks ZIP extension and size', () => {
  assert.equal(inspectPluginArchive({ name: 'demo.zip', size: 100 }).valid, true)
  assert.equal(inspectPluginArchive({ name: 'demo.tar', size: 100 }).valid, false)
  assert.equal(inspectPluginArchive({ name: 'demo.zip', size: 65 * 1024 * 1024 }).valid, false)
  assert.equal(derivePluginId('My Plugin.zip'), 'my_plugin')
})
