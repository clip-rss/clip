import { describe, expect, it } from 'vitest'
import en from './locales/en.json'
import zh from './locales/zh.json'
import zhTW from './locales/zh-TW.json'

type JsonValue = JsonObject | JsonValue[] | string | number | boolean | null
type JsonObject = { [key: string]: JsonValue }

function keySet(value: JsonValue, prefix = ''): Set<string> {
  const keys = new Set<string>()
  if (!value || typeof value !== 'object' || Array.isArray(value)) return keys
  for (const [key, child] of Object.entries(value)) {
    const path = prefix ? `${prefix}.${key}` : key
    keys.add(path)
    for (const nested of keySet(child, path)) keys.add(nested)
  }
  return keys
}

function leafKeys(value: JsonValue): Set<string> {
  const keys = new Set<string>()
  if (!value || typeof value !== 'object' || Array.isArray(value)) return keys
  for (const [key, child] of Object.entries(value)) {
    if (child && typeof child === 'object' && !Array.isArray(child)) {
      for (const nested of leafKeys(child)) keys.add(`${key}.${nested}`)
    } else {
      keys.add(key)
    }
  }
  return keys
}

describe('locale catalogs', () => {
  it('does not contain orphaned zh-TW keys and keeps updater complete', () => {
    const supported = new Set([...keySet(en), ...keySet(zh)])
    for (const key of keySet(zhTW)) {
      expect(supported.has(key), `orphan locale key: ${key}`).toBe(true)
    }

    const enUpdater = leafKeys(en.updater)
    const zhUpdater = leafKeys(zh.updater)
    const zhTWUpdater = leafKeys(zhTW.updater)
    expect(zhTWUpdater).toEqual(enUpdater)
    expect(zhTWUpdater).toEqual(zhUpdater)
  })
})
