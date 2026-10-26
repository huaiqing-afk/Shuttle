import { test } from 'node:test'
import assert from 'node:assert/strict'
import { normalizePalette, resolvePalette, terminalPalettes } from '../src/features/terminal/palettes.ts'

test('eight distinct presets each supply all 16 ANSI colors and terminal surfaces', () => {
  assert.equal(terminalPalettes.length, 8)
  assert.equal(new Set(terminalPalettes.map(p => p.id)).size, 8)
  for (const palette of terminalPalettes) {
    assert.equal(Object.keys(palette.theme).length, 20)
    for (const color of Object.values(palette.theme)) assert.match(color as string, /^#[0-9a-f]{6}$/i)
  }
})

test('auto follows app theme while explicit choices stay fixed', () => {
  assert.equal(resolvePalette('auto', 'dark').id, 'tokyo-night')
  assert.equal(resolvePalette('auto', 'light').id, 'tokyo-day')
  for (const palette of terminalPalettes) {
    assert.equal(normalizePalette(palette.id), palette.id)
    assert.equal(resolvePalette(palette.id, 'dark'), palette)
    assert.equal(resolvePalette(palette.id, 'light'), palette)
  }
})

test('missing or obsolete storage choices fall back to auto', () => {
  for (const value of [null, '', 'obsolete', '__proto__']) assert.equal(normalizePalette(value), 'auto')
  assert.equal(resolvePalette('obsolete', 'light').id, 'tokyo-day')
})
