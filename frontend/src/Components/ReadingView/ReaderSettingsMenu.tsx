import { useTranslation } from 'react-i18next'
import * as DropdownMenu from '@radix-ui/react-dropdown-menu'
import { useReaderStore } from '../../Stores'
import type {
  ReaderBackground,
  ReaderFontFamily,
  ReaderFontSize,
  ReaderLineHeight,
  ReaderWidth,
} from '../../Types'
import { LetterCaseIcon, CheckIcon } from './Icons'
import styles from './ReadingView.module.scss'

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

function ReaderSettingsMenu(): JSX.Element {
  const { t } = useTranslation()
  const s = useReaderStore()

  const fontOptions = [
    { value: 'sans' as ReaderFontFamily, label: t('reader.font.sans') },
    { value: 'serif' as ReaderFontFamily, label: t('reader.font.serif') },
    { value: 'mono' as ReaderFontFamily, label: t('reader.font.mono') },
  ]
  const sizeOptions = [
    { value: 14 as ReaderFontSize, label: t('reader.size.small') },
    { value: 16 as ReaderFontSize, label: t('reader.size.medium') },
    { value: 18 as ReaderFontSize, label: t('reader.size.large') },
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
    <DropdownMenu.Root>
      <DropdownMenu.Trigger asChild>
        <button
          type="button"
          className={styles.toolbarBtn}
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
          <DropdownMenu.RadioGroup
            value={String(s.fontSize)}
            onValueChange={(v) => s.setFontSize(Number(v) as ReaderFontSize)}
          >
            {sizeOptions.map((o) => (
              <RadioRow key={o.value} value={String(o.value)} label={o.label} />
            ))}
          </DropdownMenu.RadioGroup>

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

          <DropdownMenu.Separator className={styles.menuSeparator} />
          <DropdownMenu.Label className={styles.menuLabel}>
            {t('reader.settings.width')}
          </DropdownMenu.Label>
          <DropdownMenu.RadioGroup
            value={s.width}
            onValueChange={(v) => s.setWidth(v as ReaderWidth)}
          >
            {widthOptions.map((o) => (
              <RadioRow key={o.value} value={o.value} label={o.label} />
            ))}
          </DropdownMenu.RadioGroup>

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
