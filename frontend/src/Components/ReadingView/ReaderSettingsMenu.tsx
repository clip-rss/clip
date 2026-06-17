import * as DropdownMenu from '@radix-ui/react-dropdown-menu'
import { useReaderStore } from '../../Stores'
import type {
  ReaderBackground,
  ReaderFontFamily,
  ReaderFontSize,
  ReaderLineHeight,
  ReaderWidth,
} from '../../Types'
import { MoreIcon, CheckIcon } from './Icons'
import styles from './ReadingView.module.scss'

const FONT_OPTIONS: { value: ReaderFontFamily; label: string }[] = [
  { value: 'sans', label: '无衬线' },
  { value: 'serif', label: '衬线' },
  { value: 'mono', label: '等宽' },
]

const SIZE_OPTIONS: { value: ReaderFontSize; label: string }[] = [
  { value: 14, label: '小' },
  { value: 16, label: '中' },
  { value: 18, label: '大' },
]

const LINE_OPTIONS: { value: ReaderLineHeight; label: string }[] = [
  { value: 1.5, label: '紧凑' },
  { value: 1.8, label: '适中' },
  { value: 2.0, label: '宽松' },
]

const WIDTH_OPTIONS: { value: ReaderWidth; label: string }[] = [
  { value: '640', label: '窄' },
  { value: '800', label: '宽' },
  { value: 'full', label: '全宽' },
]

const BG_OPTIONS: { value: ReaderBackground; label: string }[] = [
  { value: 'default', label: '跟随主题' },
  { value: 'light', label: '亮色' },
  { value: 'sepia', label: '护眼' },
  { value: 'dark', label: '暗色' },
]

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
  const s = useReaderStore()

  return (
    <DropdownMenu.Root>
      <DropdownMenu.Trigger asChild>
        <button type="button" className={styles.toolbarBtn} title="阅读设置" aria-label="阅读设置">
          <MoreIcon size={18} />
        </button>
      </DropdownMenu.Trigger>
      <DropdownMenu.Portal>
        <DropdownMenu.Content className={styles.menuContent} align="end" sideOffset={6}>
          <DropdownMenu.Label className={styles.menuLabel}>字体</DropdownMenu.Label>
          <DropdownMenu.RadioGroup
            value={s.fontFamily}
            onValueChange={(v) => s.setFontFamily(v as ReaderFontFamily)}
          >
            {FONT_OPTIONS.map((o) => (
              <RadioRow key={o.value} value={o.value} label={o.label} />
            ))}
          </DropdownMenu.RadioGroup>

          <DropdownMenu.Separator className={styles.menuSeparator} />
          <DropdownMenu.Label className={styles.menuLabel}>字号</DropdownMenu.Label>
          <DropdownMenu.RadioGroup
            value={String(s.fontSize)}
            onValueChange={(v) => s.setFontSize(Number(v) as ReaderFontSize)}
          >
            {SIZE_OPTIONS.map((o) => (
              <RadioRow key={o.value} value={String(o.value)} label={o.label} />
            ))}
          </DropdownMenu.RadioGroup>

          <DropdownMenu.Separator className={styles.menuSeparator} />
          <DropdownMenu.Label className={styles.menuLabel}>行高</DropdownMenu.Label>
          <DropdownMenu.RadioGroup
            value={String(s.lineHeight)}
            onValueChange={(v) => s.setLineHeight(Number(v) as ReaderLineHeight)}
          >
            {LINE_OPTIONS.map((o) => (
              <RadioRow key={o.value} value={String(o.value)} label={o.label} />
            ))}
          </DropdownMenu.RadioGroup>

          <DropdownMenu.Separator className={styles.menuSeparator} />
          <DropdownMenu.Label className={styles.menuLabel}>宽度</DropdownMenu.Label>
          <DropdownMenu.RadioGroup
            value={s.width}
            onValueChange={(v) => s.setWidth(v as ReaderWidth)}
          >
            {WIDTH_OPTIONS.map((o) => (
              <RadioRow key={o.value} value={o.value} label={o.label} />
            ))}
          </DropdownMenu.RadioGroup>

          <DropdownMenu.Separator className={styles.menuSeparator} />
          <DropdownMenu.Label className={styles.menuLabel}>阅读背景</DropdownMenu.Label>
          <DropdownMenu.RadioGroup
            value={s.background}
            onValueChange={(v) => s.setBackground(v as ReaderBackground)}
          >
            {BG_OPTIONS.map((o) => (
              <RadioRow key={o.value} value={o.value} label={o.label} />
            ))}
          </DropdownMenu.RadioGroup>
        </DropdownMenu.Content>
      </DropdownMenu.Portal>
    </DropdownMenu.Root>
  )
}

export default ReaderSettingsMenu
