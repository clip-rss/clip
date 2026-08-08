import { Component, type ErrorInfo, type ReactNode, useMemo } from 'react'
import { System } from '@wailsio/runtime'
import { openURL, SystemService } from '../../Utils/Api'
import styles from './CrashPage.module.scss'

const BUG_REPORT_URL = 'https://github.com/clip-rss/clip/issues'

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

  private captureError(
    error: unknown,
    source: CrashReport['source'],
    componentStack = '',
  ): void {
    const report = createCrashReport(error, source, componentStack)
    this.setState({ report })

    void enrichCrashReport(report).then((enrichedReport) => {
      this.setState((state) =>
        state.report === report ? { report: enrichedReport } : null,
      )
    })
  }

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
    this.captureError(error, 'react', info.componentStack ?? '')
  }

  handleWindowError = (event: ErrorEvent): void => {
    const error = event.error instanceof Error ? event.error : event.message
    this.captureError(error, 'window')
  }

  handleUnhandledRejection = (event: PromiseRejectionEvent): void => {
    this.captureError(event.reason, 'promise')
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
    platform: inferPlatform(),
    arch: inferArchitecture(),
    version: 'Unavailable',
  }
}

async function enrichCrashReport(report: CrashReport): Promise<CrashReport> {
  const [environmentResult, versionResult] = await Promise.allSettled([
    System.Environment(),
    SystemService.Version(),
  ])

  const environment =
    environmentResult.status === 'fulfilled'
      ? environmentResult.value
      : undefined
  const version =
    versionResult.status === 'fulfilled' ? versionResult.value : undefined

  return {
    ...report,
    platform: environment ? platformName(environment.OS) : report.platform,
    arch: environment?.Arch || report.arch,
    version: version || report.version,
  }
}

function inferPlatform(): CrashReport['platform'] {
  if (System.IsMac()) return 'macOS'
  if (System.IsWindows()) return 'Windows'
  return platformName(window.navigator.userAgent)
}

function platformName(os: string): CrashReport['platform'] {
  return /darwin|mac/i.test(os) ? 'macOS' : 'Windows'
}

function inferArchitecture(): string {
  if (System.IsARM64()) return 'arm64'
  if (System.IsAMD64()) return 'amd64'
  if (System.IsARM()) return 'arm'

  const userAgent = window.navigator.userAgent
  if (/arm64|aarch64/i.test(userAgent)) return 'arm64'
  if (/x86_64|x64|win64|amd64/i.test(userAgent)) return 'amd64'
  if (/i[3-6]86|x86/i.test(userAgent)) return '386'
  return 'unknown'
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
          <button
            type="button"
            className={styles.button}
            onClick={() => openURL(BUG_REPORT_URL)}
          >
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
