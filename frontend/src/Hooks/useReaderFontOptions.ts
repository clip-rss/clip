// 阅读器字体选择的共享选项构建器。
//
// 工具栏 ReaderSettingsMenu 与设置面板「阅读」tab 共用这一份，消除两处各维护一份
// fontOptions 的重复；分组固定为「预设三档 → 已下载」。

import { useTranslation } from 'react-i18next'
import { useFontStore } from '../Stores'
import { READER_FONT_PRESETS } from '../Types'
import type { ReaderFontFamily } from '../Types'

/** 字体选项归属的分组：内置预设 / 已下载。 */
export type ReaderFontGroup = 'preset' | 'downloaded'

export interface ReaderFontOption {
  value: ReaderFontFamily
  label: string
  group: ReaderFontGroup
}

/**
 * 字体选项：预设三档 + 已下载字体。
 *
 * 已下载字体的显示名取自目录清单；清单未加载时退化为字体 id（仍可选中，
 * 只是不好看，比整项消失好）。
 */
export function useReaderFontOptions(): ReaderFontOption[] {
  const { t } = useTranslation()
  const installed = useFontStore((s) => s.installed)
  const catalog = useFontStore((s) => s.catalog)

  const nameById = new Map((catalog?.fonts ?? []).map((f) => [f.id, f.name]))
  return [
    ...READER_FONT_PRESETS.map((v) => ({
      value: v as ReaderFontFamily,
      label: t(`reader.font.${v}`),
      group: 'preset' as const,
    })),
    ...installed.map((f) => ({
      value: f.id,
      label: nameById.get(f.id) ?? f.id,
      group: 'downloaded' as const,
    })),
  ]
}
