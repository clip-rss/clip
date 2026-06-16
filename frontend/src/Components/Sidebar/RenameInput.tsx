import { useEffect, useRef, useState } from 'react'
import styles from './Sidebar.module.scss'

interface RenameInputProps {
  initialValue: string
  onSubmit: (value: string) => void
  onCancel: () => void
}

/** 行内重命名输入框：挂载即聚焦全选，Enter 提交 / Esc 取消 / 失焦提交。 */
function RenameInput(props: RenameInputProps): JSX.Element {
  const { initialValue, onSubmit, onCancel } = props
  const [value, setValue] = useState(initialValue)
  const inputRef = useRef<HTMLInputElement>(null)

  useEffect(() => {
    const input = inputRef.current
    if (input) {
      input.focus()
      input.select()
    }
  }, [])

  function commit(): void {
    const trimmed = value.trim()
    if (trimmed) onSubmit(trimmed)
    else onCancel()
  }

  function handleKeyDown(e: React.KeyboardEvent<HTMLInputElement>): void {
    if (e.key === 'Enter') {
      e.preventDefault()
      commit()
    } else if (e.key === 'Escape') {
      e.preventDefault()
      onCancel()
    }
  }

  return (
    <input
      ref={inputRef}
      className={styles.renameInput}
      value={value}
      onChange={(e) => setValue(e.target.value)}
      onKeyDown={handleKeyDown}
      onBlur={commit}
      // 阻止点击冒泡到行的 onClick（避免触发选中）
      onClick={(e) => e.stopPropagation()}
      onDragStart={(e) => e.preventDefault()}
    />
  )
}

export default RenameInput
