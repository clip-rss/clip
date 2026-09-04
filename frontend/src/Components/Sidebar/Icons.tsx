// 左侧栏内联 SVG 图标。统一 stroke=currentColor，由父级控制颜色与尺寸。

interface IconProps {
  size?: number
  className?: string
}

function base(props: IconProps): {
  width: number
  height: number
  className?: string
} {
  const size = props.size ?? 16
  return { width: size, height: size, className: props.className }
}

/** 展开箭头（指向右，展开时由 CSS 旋转 90°）。 */
export function ChevronIcon(props: IconProps): JSX.Element {
  return (
    <svg
      {...base({ size: 12, ...props })}
      viewBox="0 0 24 24"
      fill="none"
      stroke="currentColor"
      strokeWidth="2.5"
      strokeLinecap="round"
      strokeLinejoin="round"
    >
      <path d="m9 18 6-6-6-6" />
    </svg>
  )
}

export function FolderIcon(props: IconProps): JSX.Element {
  return (
    <svg
      {...base(props)}
      viewBox="0 0 24 24"
      fill="none"
      stroke="currentColor"
      strokeWidth="2"
      strokeLinecap="round"
      strokeLinejoin="round"
    >
      <path d="M4 20a2 2 0 0 1-2-2V6a2 2 0 0 1 2-2h5l2 3h7a2 2 0 0 1 2 2v9a2 2 0 0 1-2 2z" />
    </svg>
  )
}

/** 订阅源默认图标（favicon 缺失时回退）。 */
export function GlobeIcon(props: IconProps): JSX.Element {
  return (
    <svg
      {...base(props)}
      viewBox="0 0 24 24"
      fill="none"
      stroke="currentColor"
      strokeWidth="2"
      strokeLinecap="round"
      strokeLinejoin="round"
    >
      <circle cx="12" cy="12" r="10" />
      <path d="M2 12h20M12 2a15.3 15.3 0 0 1 4 10 15.3 15.3 0 0 1-4 10 15.3 15.3 0 0 1-4-10 15.3 15.3 0 0 1 4-10z" />
    </svg>
  )
}

/** 「全部文章」根项图标。 */
export function InboxIcon(props: IconProps): JSX.Element {
  return (
    <svg
      {...base(props)}
      viewBox="0 0 24 24"
      fill="none"
      stroke="currentColor"
      strokeWidth="2"
      strokeLinecap="round"
      strokeLinejoin="round"
    >
      <path d="M22 12h-6l-2 3h-4l-2-3H2" />
      <path d="M5.45 5.11 2 12v6a2 2 0 0 0 2 2h16a2 2 0 0 0 2-2v-6l-3.45-6.89A2 2 0 0 0 16.76 4H7.24a2 2 0 0 0-1.79 1.11z" />
    </svg>
  )
}

export function PlusIcon(props: IconProps): JSX.Element {
  return (
    <svg
      {...base(props)}
      viewBox="0 0 24 24"
      fill="none"
      stroke="currentColor"
      strokeWidth="2"
      strokeLinecap="round"
      strokeLinejoin="round"
    >
      <path d="M12 5v14M5 12h14" />
    </svg>
  )
}

/** 手动刷新（旋转箭头）。 */
export function RefreshIcon(props: IconProps): JSX.Element {
  return (
    <svg
      {...base(props)}
      viewBox="0 0 24 24"
      fill="none"
      stroke="currentColor"
      strokeWidth="2"
      strokeLinecap="round"
      strokeLinejoin="round"
    >
      <path d="M21 12a9 9 0 1 1-2.64-6.36" />
      <path d="M21 3v6h-6" />
    </svg>
  )
}

/** 正在刷新的旋转圆环。 */
export function SpinnerIcon(props: IconProps): JSX.Element {
  return (
    <svg
      {...base(props)}
      viewBox="0 0 24 24"
      fill="none"
      stroke="currentColor"
      strokeWidth="2.5"
      strokeLinecap="round"
    >
      <path d="M12 2a10 10 0 0 1 10 10" />
    </svg>
  )
}

/** 暂停状态标记。 */
export function PauseIcon(props: IconProps): JSX.Element {
  return (
    <svg
      {...base(props)}
      viewBox="0 0 24 24"
      fill="none"
      stroke="currentColor"
      strokeWidth="2"
      strokeLinecap="round"
      strokeLinejoin="round"
    >
      <rect x="6" y="4" width="4" height="16" rx="1" />
      <rect x="14" y="4" width="4" height="16" rx="1" />
    </svg>
  )
}

/** 排序图标（上下箭头，表示可排序）。 */
export function SortIcon(props: IconProps): JSX.Element {
  return (
    <svg
      {...base(props)}
      viewBox="0 0 24 24"
      fill="none"
      stroke="currentColor"
      strokeWidth="2"
      strokeLinecap="round"
      strokeLinejoin="round"
    >
      <path d="m3 16 4 4 4-4" />
      <path d="M7 20V4" />
      <path d="m21 8-4-4-4 4" />
      <path d="M17 4v16" />
    </svg>
  )
}
