// 数字输入：[-] 数字 [+] 三段式，只接受非负整数，可设上下限。

import { useCallback, useEffect, useState } from 'react'
import clsx from 'clsx'
import { useTranslation } from 'react-i18next'
import styles from './InputNumber.module.scss'

/* ============================ 纯函数（便于测试） ============================ */

/** 把数值收进 [min, max]；某一端为 null/undefined 时该方向视为无界。 */
export function clampToRange(
  value: number,
  min?: number | null,
  max?: number | null,
): number {
  let next = value
  if (typeof min === 'number' && next < min) next = min
  if (typeof max === 'number' && next > max) next = max
  return next
}

/**
 * 解析输入框草稿：空串或非数字回落 fallback，合法数字收进区间。
 * onChange 已把草稿过滤成纯数字，这里再校验一次，便于脱离 DOM 单测与防御非法入参。
 */
export function resolveDraft(
  draft: string,
  fallback: number,
  min?: number | null,
  max?: number | null,
): number {
  if (!/^\d+$/.test(draft)) return fallback
  return clampToRange(Number(draft), min, max)
}

/* ============================ 组件 ============================ */

interface InputNumberProps {
  value: number
  onChange: (value: number) => void
  /** 下界，省略或 null 表示无下界。 */
  min?: number | null
  /** 上界，省略或 null 表示无上界。 */
  max?: number | null
  /** 加减一次的步长，默认 1。 */
  step?: number
  disabled?: boolean
  /** 数字框的无障碍名称，说明这个数字代表什么（如「保留份数」）。 */
  label?: string
  className?: string
}

function InputNumber(props: InputNumberProps): JSX.Element {
  const {
    value,
    onChange,
    min = null,
    max = null,
    step = 1,
    disabled = false,
    label,
    className,
  } = props
  const { t } = useTranslation()

  // 草稿允许为空或越界（否则用户没法清空重敲），只在提交时收窄。
  const [draft, setDraft] = useState(String(value))

  // 外部改动 value（含本组件提交后的回落）时同步草稿。
  useEffect(() => {
    setDraft(String(value))
  }, [value])

  /** 提交草稿：非法值回落当前 value，越界值收窄到边界。 */
  const commit = useCallback(() => {
    const next = resolveDraft(draft, value, min, max)
    setDraft(String(next))
    if (next !== value) onChange(next)
  }, [draft, value, min, max, onChange])

  /**
   * 加减。基准取草稿而非 value：按钮会让输入框失焦并先触发一次 commit，
   * 若以 value 为基准，用户刚敲进去还没提交的数字会被丢掉。
   */
  const stepBy = useCallback(
    (delta: number) => {
      const base = resolveDraft(draft, value, min, max)
      const next = clampToRange(base + delta * step, min, max)
      setDraft(String(next))
      if (next !== value) onChange(next)
    },
    [draft, value, step, min, max, onChange],
  )

  const atMin = typeof min === 'number' && value <= min
  const atMax = typeof max === 'number' && value >= max

  return (
    <div className={clsx(styles.root, className)}>
      <button
        type="button"
        className={styles.step}
        aria-label={t('common.decrease')}
        disabled={disabled || atMin}
        // 不让按钮抢走输入框焦点，连续点按时不打断正在编辑的草稿。
        onMouseDown={(e) => e.preventDefault()}
        onClick={() => stepBy(-1)}
      >
        <span
          className={clsx(styles.icon, styles.iconMinus)}
          aria-hidden="true"
        />
      </button>
      <input
        type="text"
        className={styles.input}
        value={draft}
        disabled={disabled}
        aria-label={label}
        inputMode="numeric"
        autoComplete="off"
        spellCheck={false}
        // 直接吞掉非数字字符，而不是先显示再纠正。
        onChange={(e) => setDraft(e.target.value.replace(/\D/g, ''))}
        onBlur={commit}
        onKeyDown={(e) => {
          if (e.key === 'Enter') {
            commit()
          } else if (e.key === 'ArrowUp') {
            e.preventDefault()
            stepBy(1)
          } else if (e.key === 'ArrowDown') {
            e.preventDefault()
            stepBy(-1)
          }
        }}
      />
      <button
        type="button"
        className={styles.step}
        aria-label={t('common.increase')}
        disabled={disabled || atMax}
        onMouseDown={(e) => e.preventDefault()}
        onClick={() => stepBy(1)}
      >
        <span
          className={clsx(styles.icon, styles.iconPlus)}
          aria-hidden="true"
        />
      </button>
    </div>
  )
}

export default InputNumber
