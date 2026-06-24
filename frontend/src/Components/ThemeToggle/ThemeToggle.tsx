import { useTranslation } from 'react-i18next'
import { useThemeStore } from '../../Stores'
import type { ThemePreference } from '../../Types'
import styles from './ThemeToggle.module.scss'

const CYCLE_ORDER: ThemePreference[] = ['light', 'dark', 'sepia', 'system']

function ThemeToggle(): JSX.Element {
  const { t } = useTranslation()
  const preference = useThemeStore((s) => s.preference)
  const setPreference = useThemeStore((s) => s.setPreference)

  const labels: Record<ThemePreference, string> = {
    light: t('theme.light'),
    dark: t('theme.dark'),
    sepia: t('theme.sepia'),
    system: t('theme.system'),
  }

  function handleClick(): void {
    const currentIndex = CYCLE_ORDER.indexOf(preference)
    const nextIndex = (currentIndex + 1) % CYCLE_ORDER.length
    setPreference(CYCLE_ORDER[nextIndex])
  }

  return (
    <button
      className={styles.toggle}
      onClick={handleClick}
      title={labels[preference]}
      aria-label={t('theme.ariaLabel', { label: labels[preference] })}
    >
      <ThemeIcon preference={preference} />
    </button>
  )
}

function ThemeIcon(props: { preference: ThemePreference }): JSX.Element {
  const { preference } = props

  if (preference === 'dark') {
    return (
      <svg
        width="20"
        height="20"
        viewBox="0 0 24 24"
        fill="none"
        stroke="currentColor"
        strokeWidth="2"
        strokeLinecap="round"
        strokeLinejoin="round"
      >
        <path d="M21 12.79A9 9 0 1 1 11.21 3 7 7 0 0 0 21 12.79z" />
      </svg>
    )
  }

  if (preference === 'sepia') {
    return (
      <svg
        width="20"
        height="20"
        viewBox="0 0 24 24"
        fill="none"
        stroke="currentColor"
        strokeWidth="2"
        strokeLinecap="round"
        strokeLinejoin="round"
      >
        <path d="M2 12h2M6.31 6.31l1.42 1.42M12 2v2M17.69 6.31l-1.42 1.42M22 12h-2" />
        <circle cx="12" cy="12" r="4" fill="currentColor" opacity="0.3" />
      </svg>
    )
  }

  if (preference === 'system') {
    return (
      <svg
        width="20"
        height="20"
        viewBox="0 0 24 24"
        fill="none"
        stroke="currentColor"
        strokeWidth="2"
        strokeLinecap="round"
        strokeLinejoin="round"
      >
        <rect x="2" y="3" width="20" height="14" rx="2" />
        <path d="M8 21h8M12 17v4" />
      </svg>
    )
  }

  return (
    <svg
      width="20"
      height="20"
      viewBox="0 0 24 24"
      fill="none"
      stroke="currentColor"
      strokeWidth="2"
      strokeLinecap="round"
      strokeLinejoin="round"
    >
      <circle cx="12" cy="12" r="5" />
      <path d="M12 1v2M12 21v2M4.22 4.22l1.42 1.42M18.36 18.36l1.42 1.42M1 12h2M21 12h2M4.22 19.78l1.42-1.42M18.36 5.64l1.42-1.42" />
    </svg>
  )
}

export default ThemeToggle
