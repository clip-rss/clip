import { Component, type ErrorInfo, type ReactNode, useMemo } from 'react'
import styles from './CrashPage.module.scss'

export interface CrashReport {
  message: string
  stack?: string
  componentStack?: string
  source: 'react' | 'window' | 'promise'
  userAgent: string
  timestamp: string
  platform: 'Windows' | 'macOS'
  arch: string
  version: string
}

interface CrashBoundaryProps {
  children: ReactNode
}

interface CrashBoundaryState {
  report: CrashReport | null
}

export class CrashBoundary extends Component<
  CrashBoundaryProps,
  CrashBoundaryState
> {
  state: CrashBoundaryState = { report: null }

  componentDidMount(): void {
    window.addEventListener('error', this.handleWindowError)
    window.addEventListener('unhandledrejection', this.handleUnhandledRejection)
  }

  componentWillUnmount(): void {
    window.removeEventListener('error', this.handleWindowError)
    window.removeEventListener(
      'unhandledrejection',
      this.handleUnhandledRejection,
    )
  }

  componentDidCatch(error: Error, info: ErrorInfo): void {
    this.setState({
      report: createCrashReport(error, 'react', info.componentStack ?? ''),
    })
  }

  handleWindowError = (event: ErrorEvent): void => {
    const error = event.error instanceof Error ? event.error : event.message
    this.setState({ report: createCrashReport(error, 'window') })
  }

  handleUnhandledRejection = (event: PromiseRejectionEvent): void => {
    this.setState({
      report: createCrashReport(event.reason, 'promise'),
    })
  }

  render(): ReactNode {
    if (this.state.report) {
      return <CrashPage report={this.state.report} />
    }
    return this.props.children
  }
}

function createCrashReport(
  error: unknown,
  source: CrashReport['source'],
  componentStack = '',
): CrashReport {
  const err = normalizeError(error)
  return {
    message: err.message,
    stack: err.stack,
    componentStack,
    source,
    userAgent: window.navigator.userAgent,
    timestamp: new Date().toISOString(),
    platform: 'Windows', // TODO: just test
    arch: 'amd64', // TODO: just test
    version: '1.0.0', // TODO: just test
  }
}

function normalizeError(error: unknown): Error {
  if (error instanceof Error) return error
  if (typeof error === 'string') return new Error(error)
  try {
    return new Error(JSON.stringify(error))
  } catch {
    return new Error(String(error))
  }
}

function CrashPage(props: { report: CrashReport }): JSX.Element {
  const { report } = props
  const reportText = useMemo(() => formatCrashReport(report), [report])

  function handleReload(): void {
    window.location.reload()
  }

  return (
    <main className={styles.page}>
      <section className={styles.panel} aria-labelledby="crash-title">
        <div className={styles.header}>
          <div className={styles.icon} aria-hidden="true">
            !
          </div>
          <div>
            <p className={styles.eyebrow}>Crash report</p>
            <h1 id="crash-title" className={styles.title}>
              Application crashed
            </h1>
          </div>
        </div>

        <div className={styles.summary}>
          <span className={styles.summaryLabel}>Reason</span>
          <span className={styles.summaryText}>{report.message}</span>
        </div>

        <div className={styles.metaGrid}>
          <div>
            <span className={styles.metaLabel}>Source</span>
            <span className={styles.metaValue}>{report.source}</span>
          </div>
          <div>
            <span className={styles.metaLabel}>Time</span>
            <span className={styles.metaValue}>{report.timestamp}</span>
          </div>
        </div>

        <label className={styles.reportLabel} htmlFor="crash-report">
          Crash details
        </label>
        <textarea
          id="crash-report"
          className={styles.reportBox}
          readOnly
          value={reportText}
        />

        <div className={styles.actions}>
          <button
            type="button"
            className={`${styles.button} ${styles.buttonPrimary}`}
            onClick={handleReload}
          >
            Reload app
          </button>
          <button type="button" className={styles.button} disabled>
            Report a bug
          </button>
        </div>
      </section>
    </main>
  )
}

function formatCrashReport(report: CrashReport): string {
  return [
    `Message: ${report.message}`,
    `Source: ${report.source}`,
    `Time: ${report.timestamp}`,
    `User agent: ${report.userAgent}`,
    `Platform: ${report.platform}`,
    `Arch: ${report.arch}`,
    `Version: ${report.version}`,
    '',
    'Stack:',
    report.stack || 'Unavailable',
    '',
    'Component stack:',
    report.componentStack?.trim() || 'Unavailable',
  ].join('\n')
}

export default CrashPage
