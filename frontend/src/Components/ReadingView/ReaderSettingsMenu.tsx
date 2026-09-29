import { useTranslation } from 'react-i18next'
import * as DropdownMenu from '@radix-ui/react-dropdown-menu'
import { useFocusReaderStore, useReaderStore } from '../../Stores'
import { READER_FONT_SIZE_MAX, READER_FONT_SIZE_MIN } from '../../Utils'
import type {
  ReaderBackground,
  ReaderFontFamily,
  ReaderLineHeight,
  ReaderWidth,
} from '../../Types'
import { InputNumber } from '../InputNumber'
import { LetterCaseIcon, CheckIcon } from './Icons'
import styles from './ReadingView.module.scss'

interface ReaderSettingsMenuProps {
  /**
   * 读写哪一套偏好：'reader'（默认）是阅读视图那套，'focus' 是专注模式独立的那套。
   * 专注模式没有「宽度」项。
   */
  variant?: 'reader' | 'focus'
  /** 触发按钮的类名。默认用阅读视图工具栏的 .toolbarBtn；专注模式的控制条另有一套按钮样式，由调用方注入。 */
  triggerClassName?: string
  /** 菜单展开状态变化。专注模式靠它钉住会自动隐藏的控制条。 */
  onOpenChange?: (open: boolean) => void
}

function RadioRow(props: { value: string; label: string }): JSX.Element {
  return (
    <DropdownMenu.RadioItem className={styles.menuItem} value={props.value}>
      <span className={styles.menuCheck}>
        <DropdownMenu.ItemIndicator>
          <CheckIcon size={14} />
        </DropdownMenu.ItemIndicator>
      </span>
      {props.label}
    </DropdownMenu.RadioItem>
  )
}

function ReaderSettingsMenu(props: ReaderSettingsMenuProps): JSX.Element {
  const { variant = 'reader', triggerClassName, onOpenChange } = props
  const { t } = useTranslation()
  const reader = useReaderStore()
  const focus = useFocusReaderStore()

  // 两套偏好同形（fontFamily / fontSize / lineHeight / background + 同名 setter），
  // 所以整份菜单可以共用，只把读写目标换掉；宽度只有阅读视图那套才有。
  const isFocus = variant === 'focus'
  const s = isFocus ? focus : reader

  const fontOptions = [
    { value: 'sans' as ReaderFontFamily, label: t('reader.font.sans') },
    { value: 'serif' as ReaderFontFamily, label: t('reader.font.serif') },
    { value: 'mono' as ReaderFontFamily, label: t('reader.font.mono') },
  ]
  const lineOptions = [
    { value: 1.5 as ReaderLineHeight, label: t('reader.lineHeight.compact') },
    { value: 1.8 as ReaderLineHeight, label: t('reader.lineHeight.moderate') },
    { value: 2.0 as ReaderLineHeight, label: t('reader.lineHeight.loose') },
  ]
  const widthOptions = [
    { value: '640' as ReaderWidth, label: t('reader.width.narrow') },
    { value: '800' as ReaderWidth, label: t('reader.width.wide') },
    { value: 'full' as ReaderWidth, label: t('reader.width.full') },
  ]
  const bgOptions = [
    {
      value: 'default' as ReaderBackground,
      label: t('reader.background.default'),
    },
    { value: 'light' as ReaderBackground, label: t('reader.background.light') },
    { value: 'sepia' as ReaderBackground, label: t('reader.background.sepia') },
    { value: 'dark' as ReaderBackground, label: t('reader.background.dark') },
  ]

  return (
    <DropdownMenu.Root onOpenChange={onOpenChange}>
      <DropdownMenu.Trigger asChild>
        <button
          type="button"
          className={triggerClassName ?? styles.toolbarBtn}
          title={t('reader.settings.title')}
          aria-label={t('reader.settings.title')}
        >
          <LetterCaseIcon size={18} />
        </button>
      </DropdownMenu.Trigger>
      <DropdownMenu.Portal>
        <DropdownMenu.Content
          className={styles.menuContent}
          align="end"
          sideOffset={6}
        >
          <DropdownMenu.Label className={styles.menuLabel}>
            {t('reader.settings.font')}
          </DropdownMenu.Label>
          <DropdownMenu.RadioGroup
            value={s.fontFamily}
            onValueChange={(v) => s.setFontFamily(v as ReaderFontFamily)}
          >
            {fontOptions.map((o) => (
              <RadioRow key={o.value} value={o.value} label={o.label} />
            ))}
          </DropdownMenu.RadioGroup>

          <DropdownMenu.Separator className={styles.menuSeparator} />
          <DropdownMenu.Label className={styles.menuLabel}>
            {t('reader.settings.fontSize')}
          </DropdownMenu.Label>
          {/*
            Radix 菜单会劫持键盘：字符键拿去做 typeahead 定位菜单项、方向键拿去做导航、
            Tab 被 preventDefault。输入框要能正常打字，就得把键盘事件截在这一层，
            不让它冒泡到 DropdownMenu.Content 上的处理器（见 react-menu 的 MenuContentImpl）。
          */}
          <div
            className={styles.menuStepper}
            onKeyDown={(e) => e.stopPropagation()}
          >
            <InputNumber
              value={s.fontSize}
              onChange={s.setFontSize}
              min={READER_FONT_SIZE_MIN}
              max={READER_FONT_SIZE_MAX}
              label={t('reader.settings.fontSize')}
            />
          </div>

          <DropdownMenu.Separator className={styles.menuSeparator} />
          <DropdownMenu.Label className={styles.menuLabel}>
            {t('reader.settings.lineHeight')}
          </DropdownMenu.Label>
          <DropdownMenu.RadioGroup
            value={String(s.lineHeight)}
            onValueChange={(v) =>
              s.setLineHeight(Number(v) as ReaderLineHeight)
            }
          >
            {lineOptions.map((o) => (
              <RadioRow key={o.value} value={String(o.value)} label={o.label} />
            ))}
          </DropdownMenu.RadioGroup>

          {isFocus ? null : (
            <>
              <DropdownMenu.Separator className={styles.menuSeparator} />
              <DropdownMenu.Label className={styles.menuLabel}>
                {t('reader.settings.width')}
              </DropdownMenu.Label>
              <DropdownMenu.RadioGroup
                value={reader.width}
                onValueChange={(v) => reader.setWidth(v as ReaderWidth)}
              >
                {widthOptions.map((o) => (
                  <RadioRow key={o.value} value={o.value} label={o.label} />
                ))}
              </DropdownMenu.RadioGroup>
            </>
          )}

          <DropdownMenu.Separator className={styles.menuSeparator} />
          <DropdownMenu.Label className={styles.menuLabel}>
            {t('reader.settings.background')}
          </DropdownMenu.Label>
          <DropdownMenu.RadioGroup
            value={s.background}
            onValueChange={(v) => s.setBackground(v as ReaderBackground)}
          >
            {bgOptions.map((o) => (
              <RadioRow key={o.value} value={o.value} label={o.label} />
            ))}
          </DropdownMenu.RadioGroup>
        </DropdownMenu.Content>
      </DropdownMenu.Portal>
    </DropdownMenu.Root>
  )
}

export default ReaderSettingsMenu
