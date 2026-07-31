import { useCallback, type ReactNode } from 'react'
import { useLayoutStore } from '../../Stores'
import { Divider } from '../Divider'
import { OfflineBanner } from '../OfflineBanner'
import styles from './Layout.module.scss'

interface LayoutProps {
  toolbar: ReactNode
  sidebar: ReactNode
  list: ReactNode
  reader: ReactNode
}

function Layout(props: LayoutProps): JSX.Element {
  const { toolbar, sidebar, list, reader } = props
  const sidebarWidth = useLayoutStore((s) => s.sidebarWidth)
  const listWidth = useLayoutStore((s) => s.listWidth)
  const resizeSidebar = useLayoutStore((s) => s.resizeSidebar)
  const resizeList = useLayoutStore((s) => s.resizeList)

  const handleSidebarResize = useCallback(
    (delta: number) => resizeSidebar(delta),
    [resizeSidebar],
  )

  const handleListResize = useCallback(
    (delta: number) => resizeList(delta),
    [resizeList],
  )

  return (
    <div className={styles.layout}>
      <OfflineBanner />
      <div className={styles.toolbar}>{toolbar}</div>
      <div className={styles.main}>
        <div className={styles.sidebar} style={{ width: `${sidebarWidth}px` }}>
          {sidebar}
        </div>
        <Divider onResize={handleSidebarResize} />
        <div className={styles.list} style={{ width: `${listWidth}px` }}>
          {list}
        </div>
        <Divider onResize={handleListResize} />
        <div className={styles.reader}>{reader}</div>
      </div>
    </div>
  )
}

export default Layout
