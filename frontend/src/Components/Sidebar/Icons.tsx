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
    <svg {...base(props)} viewBox="0 0 15 15" fill="none">
      <path
        d="M7.50037 0.850006C10.6644 0.850189 12.2943 3.06869 12.9994 4.31094L13.0004 4.31192V2.5004C13.0004 2.22425 13.2242 2.0004 13.5004 2.0004C13.7763 2.00059 14.0004 2.22438 14.0004 2.5004V5.5004C14.0002 5.77625 13.7762 6.0002 13.5004 6.0004H10.5004C10.2243 6.0004 10.0006 5.77637 10.0004 5.5004C10.0004 5.22425 10.2242 5.0004 10.5004 5.0004H12.2328L12.1215 4.79239C11.4802 3.66597 10.1107 1.85019 7.50037 1.85001C4.06019 1.85001 1.84998 4.665 1.84998 7.5004C1.85018 10.3357 4.06034 13.1498 7.50037 13.1498C9.16525 13.1497 10.5296 12.496 11.5013 11.5072L11.6927 11.3031C12.126 10.8159 12.4715 10.2575 12.7172 9.66055L12.765 9.57071C12.8948 9.37795 13.1462 9.2963 13.3695 9.38809C13.6248 9.49314 13.7468 9.78515 13.642 10.0404L13.5111 10.3373C13.2362 10.9247 12.877 11.4767 12.4398 11.9682L12.2142 12.2084C11.062 13.3807 9.44396 14.1497 7.50037 14.1498C3.43771 14.1498 0.850179 10.8149 0.849976 7.5004C0.849976 4.1858 3.43755 0.850006 7.50037 0.850006Z"
        fill="currentColor"
      />
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
