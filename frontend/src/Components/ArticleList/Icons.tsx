// 文章列表内联 SVG 图标。

interface IconProps {
  size?: number
  className?: string
}

export function StarIcon(props: IconProps & { filled?: boolean }): JSX.Element {
  const size = props.size ?? 16
  return (
    <svg
      width={size}
      height={size}
      className={props.className}
      viewBox="0 0 15 15"
      fill="currentColor"
    >
      {props.filled ? (
        <path d="M7.22257 0.665927C7.32508 0.419634 7.67476 0.419617 7.77726 0.665927L9.413 4.6005C9.45615 4.70425 9.55396 4.77497 9.66593 4.78409L13.914 5.12491C14.1799 5.14635 14.2875 5.47869 14.0849 5.65226L10.8485 8.42374C10.7632 8.49693 10.7258 8.61221 10.7519 8.72159L11.7411 12.8661C11.803 13.1256 11.5206 13.331 11.2929 13.1923L7.65616 10.9706C7.56022 10.9121 7.43961 10.9121 7.34366 10.9706L3.70694 13.1923C3.47926 13.3311 3.19681 13.1256 3.2587 12.8661L4.24796 8.72159C4.27405 8.61223 4.23661 8.49693 4.15128 8.42374L0.914951 5.65226C0.712311 5.47867 0.819914 5.1463 1.08585 5.12491L5.3339 4.78409C5.44584 4.77494 5.54368 4.70422 5.58683 4.6005L7.22257 0.665927Z" />
      ) : (
        <path d="M7.22271 0.665773C7.32528 0.419686 7.67484 0.419669 7.77739 0.665773L9.41314 4.60034C9.4563 4.70405 9.55412 4.77481 9.66607 4.78394L13.9141 5.12476C14.1799 5.14619 14.2874 5.47752 14.085 5.65112L13.8555 5.84644L10.8487 8.42358C10.7636 8.49661 10.7263 8.61137 10.752 8.72046L11.6709 12.572L11.7413 12.866C11.7953 13.093 11.5857 13.2787 11.3799 13.2283L11.293 13.1912L11.0352 13.033L7.6563 10.9705C7.56036 10.9119 7.43975 10.9119 7.3438 10.9705L3.70708 13.1912L3.62017 13.2283C3.41445 13.2787 3.20484 13.093 3.25884 12.866L3.32818 12.572L4.2481 8.72046C4.2674 8.63858 4.25134 8.55322 4.20611 8.48511L4.15142 8.42358L0.91509 5.65112C0.712634 5.47752 0.820148 5.14618 1.08599 5.12476L1.38677 5.10034L5.33403 4.78394C5.41796 4.77708 5.49411 4.73573 5.54497 4.67163L5.58696 4.60034L7.22271 0.665773ZM6.50982 4.98413C6.34606 5.37782 6.00173 5.66244 5.59282 5.75366L5.41314 5.78101L2.84282 5.98608L4.80181 7.66382L4.93071 7.79077C5.16843 8.05995 5.28398 8.41668 5.25005 8.77417L5.22075 8.95288L4.62212 11.4597L6.82232 10.1169L6.98345 10.0339C7.31289 9.89117 7.68721 9.89116 8.01665 10.0339L8.17778 10.1169L10.377 11.4597L9.77935 8.95288C9.66641 8.47881 9.82817 7.98087 10.1983 7.66382L12.1563 5.98608L9.58696 5.78101C9.16189 5.74693 8.78445 5.50702 8.57134 5.14624L8.49028 4.98413L7.50005 2.60327L6.50982 4.98413Z" />
      )}
    </svg>
  )
}

export function ExternalLinkIcon(props: IconProps): JSX.Element {
  const size = props.size ?? 16
  return (
    <svg
      width={size}
      height={size}
      className={props.className}
      viewBox="0 0 15 15"
      fill="currentColor"
    >
      <path d="M6.5 2C6.77614 2 7 2.22386 7 2.5C7 2.77614 6.77614 3 6.5 3H3V12H12V8.5C12 8.22386 12.2239 8 12.5 8C12.7761 8 13 8.22386 13 8.5V12C13 12.5523 12.5523 13 12 13H3C2.44772 13 2 12.5523 2 12V3C2 2.44772 2.44772 2 3 2H6.5ZM12.5 2C12.5327 2 12.5655 2.00242 12.5977 2.00879L12.6006 2.00977C12.6596 2.02181 12.7142 2.04529 12.7637 2.07617C12.7759 2.08379 12.7872 2.09279 12.7988 2.10156C12.8133 2.11247 12.8276 2.12336 12.8408 2.13574C12.8448 2.13951 12.8496 2.14255 12.8535 2.14648C12.8579 2.15091 12.861 2.15658 12.8652 2.16113C12.8782 2.17509 12.8901 2.18971 12.9014 2.20508C12.9096 2.21632 12.9176 2.22752 12.9248 2.23926C12.9352 2.25615 12.9438 2.27385 12.9521 2.29199C12.9559 2.30024 12.9605 2.30801 12.9639 2.31641C12.9719 2.33664 12.9781 2.35748 12.9834 2.37891C12.9859 2.38905 12.9893 2.39893 12.9912 2.40918C12.9966 2.43867 13 2.46896 13 2.5V5.5C13 5.77614 12.7761 6 12.5 6C12.2239 6 12 5.77614 12 5.5V3.70703L6.85352 8.85352C6.65825 9.04878 6.34175 9.04878 6.14648 8.85352C5.95122 8.65825 5.95122 8.34175 6.14648 8.14648L11.293 3H9.5C9.22386 3 9 2.77614 9 2.5C9 2.22386 9.22386 2 9.5 2H12.5Z" />
    </svg>
  )
}

export function SortIcon(props: IconProps): JSX.Element {
  const size = props.size ?? 16
  return (
    <svg
      width={size}
      height={size}
      className={props.className}
      viewBox="0 0 24 24"
      fill="none"
      stroke="currentColor"
      strokeWidth="2"
      strokeLinecap="round"
      strokeLinejoin="round"
    >
      <path
        d="M3 6h12M3 12h9M3 18h6M17 8l4-4 4 4M21 4v16"
        transform="translate(-2 0)"
      />
    </svg>
  )
}

export function MoreIcon(props: IconProps): JSX.Element {
  const size = props.size ?? 16
  return (
    <svg
      width={size}
      height={size}
      className={props.className}
      viewBox="0 0 24 24"
      fill="currentColor"
      stroke="none"
    >
      <circle cx="5" cy="12" r="1.6" />
      <circle cx="12" cy="12" r="1.6" />
      <circle cx="19" cy="12" r="1.6" />
    </svg>
  )
}

export function ChevronDownIcon(props: IconProps): JSX.Element {
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
      <path d="m6 9 6 6 6-6" />
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
