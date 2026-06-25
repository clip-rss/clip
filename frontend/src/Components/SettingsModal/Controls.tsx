import type { CSSProperties } from 'react'
import clsx from 'clsx'
import styles from './SettingsModal.module.scss'

/** 设置行：左侧标题 + 可选说明，右侧控件。设置面板各区统一布局。 */
export function SettingRow(props: {
  label: string
  description?: string
  children?: React.ReactNode
}): JSX.Element {
  const { label, description, children } = props
  return (
    <div className={styles.row}>
      <div className={styles.rowText}>
        <span className={styles.rowLabel}>{label}</span>
        {description ? (
          <span className={styles.rowDesc}>{description}</span>
        ) : null}
      </div>
      {children ? <div className={styles.rowControl}>{children}</div> : null}
    </div>
  )
}

/** 分段单选控件：泛型 value，按钮组形态。 */
export function SegmentedControl<T extends string | number>(props: {
  value: T
  options: { value: T; label: string }[]
  onChange: (value: T) => void
}): JSX.Element {
  const { value, options, onChange } = props
  const activeIndex = Math.max(
    0,
    options.findIndex((o) => o.value === value),
  )
  const style = {
    '--seg-count': Math.max(1, options.length),
    '--seg-indicator-offset': `${activeIndex * 100}%`,
  } as CSSProperties

  return (
    <div className={styles.segmented} role="radiogroup" style={style}>
      {options.length ? (
        <span className={styles.segIndicator} aria-hidden="true" />
      ) : null}
      {options.map((o) => (
        <button
          key={String(o.value)}
          type="button"
          role="radio"
          aria-checked={o.value === value}
          data-label={o.label}
          className={clsx(
            styles.segItem,
            o.value === value && styles.segItemActive,
          )}
          onClick={() => onChange(o.value)}
        >
          <span className={styles.segLabel}>{o.label}</span>
        </button>
      ))}
    </div>
  )
}

/** 开关控件。 */
export function Toggle(props: {
  checked: boolean
  onChange: (checked: boolean) => void
  label?: string
}): JSX.Element {
  const { checked, onChange, label } = props
  return (
    <button
      type="button"
      role="switch"
      aria-checked={checked}
      aria-label={label}
      className={clsx(styles.toggle, checked && styles.toggleOn)}
      onClick={() => onChange(!checked)}
    >
      <span
        className={clsx(styles.toggleKnob, checked && styles.toggleKnobOn)}
      />
    </button>
  )
}
