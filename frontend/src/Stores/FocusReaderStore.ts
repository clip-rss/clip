// 专注模式的排版偏好：与阅读视图那套各自独立存储，调这边不影响那边。
// 没有宽度项。
//
// 结构与 ReaderStore 对称（默认值 / 入站收窄 / 写后端 / 单向同步），
// 但共用 ReaderStore 的 narrowReaderPrefs 收窄逻辑，白名单只有一份。

import { create } from 'zustand'
import type {
  FocusReaderPrefs,
  ReaderBackground,
  ReaderFontFamily,
  ReaderLineHeight,
  Settings,
} from '../Types'
import {
  narrowReaderPrefs,
  toReaderPrefs,
  type FocusReaderSettingsFields,
} from './ReaderStore'
import { useSettingsStore } from './SettingsStore'

interface FocusReaderState extends FocusReaderPrefs {
  setFontFamily: (fontFamily: ReaderFontFamily) => void
  setFontSize: (fontSize: number) => void
  setLineHeight: (lineHeight: ReaderLineHeight) => void
  setBackground: (background: ReaderBackground) => void
}

/** 出厂默认排版，与后端 store.DefaultSettings() 的 Focus* 字段保持一致。 */
export const DEFAULT_FOCUS_READER_PREFS: FocusReaderPrefs = {
  fontFamily: 'sans',
  fontSize: 16,
  lineHeight: 1.8,
  background: 'default',
}

/** 把后端设置收窄为合法排版偏好。 */
export function toFocusReaderPrefs(
  settings: FocusReaderSettingsFields,
): FocusReaderPrefs {
  return narrowReaderPrefs(
    {
      fontFamily: settings.focusFontFamily,
      fontSize: settings.focusFontSize,
      lineHeight: settings.focusLineHeight,
      background: settings.focusBackground,
    },
    DEFAULT_FOCUS_READER_PREFS,
  )
}

/* ---------- store ---------- */

/** 写后端。失败时 SettingsStore 自行回滚，随后订阅会把旧值同步回本 store。 */
function persist(partial: Partial<Settings>): void {
  void useSettingsStore.getState().update(partial)
}

export const useFocusReaderStore = create<FocusReaderState>()((set) => ({
  ...DEFAULT_FOCUS_READER_PREFS,

  setFontFamily(fontFamily) {
    set({ fontFamily })
    persist({ focusFontFamily: fontFamily })
  },
  setFontSize(fontSize) {
    set({ fontSize })
    persist({ focusFontSize: fontSize })
  },
  setLineHeight(lineHeight) {
    set({ lineHeight })
    persist({ focusLineHeight: lineHeight })
  },
  setBackground(background) {
    set({ background })
    persist({ focusBackground: background })
  },
}))

/* ---------- 后端 → store 单向同步 ---------- */

// 仅在取值真正变化时 setState，避免与写后端形成回环。
function syncFromSettings(settings: Settings | null): void {
  if (!settings) return
  const next = toFocusReaderPrefs(settings)
  const cur = useFocusReaderStore.getState()
  if (
    next.fontFamily === cur.fontFamily &&
    next.fontSize === cur.fontSize &&
    next.lineHeight === cur.lineHeight &&
    next.background === cur.background
  ) {
    return
  }
  useFocusReaderStore.setState(next)
}

useSettingsStore.subscribe((state) => {
  syncFromSettings(state.settings)
})

/* ---------- 首次播种 ---------- */

/**
 * 后端是否还没有专注模式的排版偏好。
 *
 * 设置在后端是一整个 JSON blob，老版本写下的行里没有 focus* 这些 key，
 * Go 反序列化后全是零值（""/0），据此判断「还没播种过」。
 * 四项都为空才算未播种：只要用户动过其中任意一项，就不该再拿阅读视图的值覆盖。
 */
export function focusPrefsUnset(settings: Settings): boolean {
  return (
    settings.focusFontFamily === '' &&
    settings.focusFontSize === 0 &&
    settings.focusLineHeight === 0 &&
    settings.focusBackground === ''
  )
}

/**
 * 首次播种：把当前阅读视图的偏好复制一份作为专注模式的初始值。
 *
 * 设计稿要求专注模式排版「与阅读视图相同」。两套偏好拆开之后，
 * 老用户升级上来若直接用出厂默认，等于把已经调好的排版重置了 —— 与 LegacyPrefs
 * 处理旧 localStorage 时同一个道理。播种后两者再无关系，各调各的。
 *
 * 幂等：写入成功后 focus* 不再是零值，下次启动自然跳过；写入失败则不播种，下次再试。
 */
export async function seedFocusPrefsFromReader(): Promise<void> {
  const store = useSettingsStore.getState()
  const settings = store.settings
  if (!settings || !focusPrefsUnset(settings)) return

  // 取收窄后的值：后端里可能还是 0/'' 这类零值，不能直接照搬过去。
  const reader = toReaderPrefs(settings)
  await store.update({
    focusFontFamily: reader.fontFamily,
    focusFontSize: reader.fontSize,
    focusLineHeight: reader.lineHeight,
    focusBackground: reader.background,
  })
}
