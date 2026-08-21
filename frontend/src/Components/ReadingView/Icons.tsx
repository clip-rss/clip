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

/**
 * 阅读设置入口
 */
export function LetterCaseIcon(props: IconProps): JSX.Element {
  const size = props.size ?? 18
  return (
    <svg
      width={size}
      height={size}
      className={props.className}
      viewBox="0 0 15 15"
      fill="none"
    >
      <path
        d="M3.68946 2.75C3.89633 2.74979 4.08178 2.87732 4.15626 3.07031L7.36622 11.3896L7.39356 11.4873C7.43224 11.7167 7.30549 11.9491 7.08009 12.0361C6.8546 12.1231 6.60494 12.0358 6.4795 11.8398L6.4336 11.75L5.38282 9.02539H2.01075L0.966806 11.749L0.920908 11.8398C0.795759 12.0354 0.546617 12.1233 0.321299 12.0371C0.0635625 11.9382 -0.0655997 11.6484 0.0332127 11.3906L3.22364 3.07129L3.25587 3.00195C3.34371 2.84806 3.50847 2.75031 3.68946 2.75ZM10.8984 5.20703C11.792 5.20703 12.6044 5.60223 13.1533 6.22461V5.71973C13.1535 5.47158 13.3554 5.26994 13.6035 5.26953C13.852 5.26953 14.0536 5.47132 14.0537 5.71973V11.5293C14.0537 11.7778 13.8521 11.9795 13.6035 11.9795C13.3553 11.9791 13.1533 11.7776 13.1533 11.5293V11.0195C12.5733 11.6675 11.7213 12.0127 10.8984 12.0127C9.3579 12.0124 8.0088 10.6332 8.0088 8.60938C8.00895 6.68571 9.25803 5.20734 10.8984 5.20703ZM11.0859 6.05762C10.1083 6.05812 9.03524 6.96621 9.03517 8.60938C9.03517 10.1527 10.0084 11.1606 11.0859 11.1611C11.9702 11.1611 12.7714 10.4926 13.1533 9.79492V7.30566C12.7635 6.58343 11.9416 6.05762 11.0859 6.05762ZM2.33692 8.17578H5.0547L3.69142 4.64258L2.33692 8.17578Z"
        fill="currentColor"
      />
    </svg>
  )
}

/**
 * 全屏/专注模式入口图标。
 * 15x15 实心填充路径（源自 public/enter-full-screen.svg），与 LetterCaseIcon 同风格。
 */
export function EnterFullScreenIcon(props: IconProps): JSX.Element {
  const size = props.size ?? 18
  return (
    <svg
      width={size}
      height={size}
      className={props.className}
      viewBox="0 0 15 15"
      fill="none"
    >
      <path
        d="M2.5 9C2.77614 9 3 9.22386 3 9.5V12H5.5C5.77614 12 6 12.2239 6 12.5C6 12.7761 5.77614 13 5.5 13H2.5C2.22386 13 2 12.7761 2 12.5V9.5C2 9.22386 2.22386 9 2.5 9ZM12.5 9C12.7761 9 13 9.22386 13 9.5V12.5C13 12.7761 12.7761 13 12.5 13H9.5C9.22386 13 9 12.7761 9 12.5C9 12.2239 9.22386 12 9.5 12H12V9.5C12 9.22386 12.2239 9 12.5 9ZM5.5 2C5.77614 2 6 2.22386 6 2.5C6 2.77614 5.77614 3 5.5 3H3V5.5C3 5.77614 2.77614 6 2.5 6C2.22386 6 2 5.77614 2 5.5V2.5L2.00977 2.39941C2.05629 2.17145 2.25829 2 2.5 2H5.5ZM12.6006 2.00977C12.8286 2.05629 13 2.25829 13 2.5V5.5C13 5.77614 12.7761 6 12.5 6C12.2239 6 12 5.77614 12 5.5V3H9.5C9.22386 3 9 2.22386 9 2.5C9 2.22386 9.22386 2 9.5 2H12.5L12.6006 2.00977Z"
        fill="currentColor"
      />
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
