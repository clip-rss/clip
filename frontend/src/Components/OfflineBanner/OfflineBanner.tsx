import {useTranslation} from 'react-i18next'
import {useOnlineStatus, usePlatform} from '../../Hooks'
import styles from './OfflineBanner.module.scss'

/**
 * 离线横幅：当网络断开时显示在窗口顶部，提示用户当前处于离线模式。
 * 自动订阅 navigator.onLine 状态，在线时自动隐藏。
 */
function OfflineBanner(): JSX.Element | null {
    const {t} = useTranslation()
    const platform = usePlatform()
    const online = useOnlineStatus()

    // 在线时不显示横幅
    if (online) return null

    return (
        <div className={styles.banner} role="alert" aria-live="polite" style={
            {'--wails-draggable': platform === 'mac' ? 'drag' : 'none'} as any
        }>
      <span className={styles.icon} aria-hidden="true">
         😧
      </span>
            <span>{t('offline.banner')}</span>
        </div>
    )
}

export default OfflineBanner
