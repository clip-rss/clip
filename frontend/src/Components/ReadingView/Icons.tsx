// 阅读视图内联 SVG 图标。

interface IconProps {
  size?: number
  className?: string
}

function svgProps(props: IconProps, fill = 'none') {
  const size = props.size ?? 18
  return {
    width: size,
    height: size,
    className: props.className,
    viewBox: '0 0 24 24',
    fill,
    stroke: 'currentColor',
    strokeWidth: 2,
    strokeLinecap: 'round' as const,
    strokeLinejoin: 'round' as const,
  }
}

/** 已读状态（实心勾选圆）——点击可标记为未读。 */
export function ReadIcon(props: IconProps): JSX.Element {
  return (
    <svg {...svgProps(props)}>
      <circle cx="12" cy="12" r="9" />
      <path d="m8.5 12 2.5 2.5 4.5-4.5" />
    </svg>
  )
}

/** 未读状态（空心圆点）——点击可标记为已读。 */
export function UnreadIcon(props: IconProps): JSX.Element {
  return (
    <svg {...svgProps(props)}>
      <circle cx="12" cy="12" r="9" />
      <circle cx="12" cy="12" r="3" fill="currentColor" stroke="none" />
    </svg>
  )
}

export function StarIcon(props: IconProps & { filled?: boolean }): JSX.Element {
  return (
    <svg {...svgProps(props, props.filled ? 'currentColor' : 'none')}>
      <path d="M12 2l3.09 6.26L22 9.27l-5 4.87 1.18 6.88L12 17.77l-6.18 3.25L7 14.14 2 9.27l6.91-1.01L12 2z" />
    </svg>
  )
}

export function NoteIcon(props: IconProps): JSX.Element {
  return (
    <svg {...svgProps(props)}>
      <path d="M14 3v4a1 1 0 0 0 1 1h4" />
      <path d="M17 21H7a2 2 0 0 1-2-2V5a2 2 0 0 1 2-2h7l5 5v11a2 2 0 0 1-2 2z" />
      <path d="M9 13h6M9 17h4" />
    </svg>
  )
}

export function ExternalLinkIcon(props: IconProps): JSX.Element {
  return (
    <svg {...svgProps(props)}>
      <path d="M18 13v6a2 2 0 0 1-2 2H5a2 2 0 0 1-2-2V8a2 2 0 0 1 2-2h6" />
      <path d="M15 3h6v6M10 14 21 3" />
    </svg>
  )
}

export function MoreIcon(props: IconProps): JSX.Element {
  return (
    <svg {...svgProps(props, 'currentColor')} stroke="none">
      <circle cx="5" cy="12" r="1.6" />
      <circle cx="12" cy="12" r="1.6" />
      <circle cx="19" cy="12" r="1.6" />
    </svg>
  )
}

export function CheckIcon(props: IconProps): JSX.Element {
  const size = props.size ?? 14
  return (
    <svg
      width={size}
      height={size}
      className={props.className}
      viewBox="0 0 24 24"
      fill="none"
      stroke="currentColor"
      strokeWidth="2.5"
      strokeLinecap="round"
      strokeLinejoin="round"
    >
      <path d="M20 6 9 17l-5-5" />
    </svg>
  )
}

export function CloseIcon(props: IconProps): JSX.Element {
  return (
    <svg {...svgProps(props)}>
      <path d="M18 6 6 18M6 6l12 12" />
    </svg>
  )
}

/** 返回箭头（专注模式「退出专注」按钮）。 */
export function BackIcon(props: IconProps): JSX.Element {
  return (
    <svg {...svgProps(props)}>
      <path d="M19 12H5" />
      <path d="m12 19-7-7 7-7" />
    </svg>
  )
}
