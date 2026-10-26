import { test } from 'node:test'
import assert from 'node:assert/strict'
import { zh, en, interpolate, normalizeChoice, resolveLanguage, translate } from '../src/lib/i18n/core.ts'

test('zh and en dictionaries have 100% symmetric keys', () => {
  const zhKeys = Object.keys(zh).sort()
  const enKeys = Object.keys(en).sort()
  assert.equal(zhKeys.length, enKeys.length, 'Key counts must match')
  assert.deepEqual(zhKeys, enKeys, 'Key lists must be identical')
  for (const [k, v] of Object.entries(zh)) {
    assert.ok(typeof v === 'string' && v.trim().length > 0, `zh key ${k} must not be empty`)
  }
  for (const [k, v] of Object.entries(en)) {
    assert.ok(typeof v === 'string' && v.trim().length > 0, `en key ${k} must not be empty`)
  }
})

test('interpolation substitutes params and leaves unsupplied ones intact', () => {
  assert.equal(interpolate('hello {name}', { name: 'world' }), 'hello world')
  assert.equal(interpolate('has {count} items', { count: 3 }), 'has 3 items')
  assert.equal(interpolate('has {count} and {missing}', { count: 0 }), 'has 0 and {missing}')
  assert.equal(interpolate('plain string'), 'plain string')
})

test('resolveLanguage respects explicit choices and detects browser locale', () => {
  assert.equal(resolveLanguage('zh-CN', 'en-US'), 'zh-CN')
  assert.equal(resolveLanguage('en-US', 'zh-CN'), 'en-US')
  assert.equal(resolveLanguage('auto', 'en-US'), 'en-US')
  assert.equal(resolveLanguage('auto', 'en-GB'), 'en-US')
  assert.equal(resolveLanguage('auto', 'zh-CN'), 'zh-CN')
  assert.equal(resolveLanguage('auto', 'zh-TW'), 'zh-CN')
  assert.equal(resolveLanguage('auto', 'ja-JP'), 'zh-CN') // fallback to zh-CN
})

test('normalizeChoice safely handles null, empty, or garbage values', () => {
  assert.equal(normalizeChoice(null), 'auto')
  assert.equal(normalizeChoice(''), 'auto')
  assert.equal(normalizeChoice('unknown'), 'auto')
  assert.equal(normalizeChoice('zh-CN'), 'zh-CN')
  assert.equal(normalizeChoice('en-US'), 'en-US')
})

test('translate returns localized string with params or falls back', () => {
  assert.equal(translate('zh-CN', 'app.tagline'), '·接驳')
  assert.equal(translate('en-US', 'app.tagline'), '· Shuttle')
  assert.equal(translate('zh-CN', 'app.dialog.exit.desc', { count: 2 }), '还有 2 个终端在运行，退出将全部终止其中进程。')
  assert.equal(translate('en-US', 'app.dialog.exit.desc', { count: 2 }), '2 terminal(s) still running. Exiting will terminate all processes. Exit anyway?')
  assert.equal(translate('en-US', 'nonexistent.key'), 'nonexistent.key')
})
