import { useCallback, type ReactNode } from 'react'
import { useLayoutStore } from '../../Stores'
import { Divider } from '../Divider'
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
  const setSidebarWidth = useLayoutStore((s) => s.setSidebarWidth)
  const setListWidth = useLayoutStore((s) => s.setListWidth)

  const handleSidebarResize = useCallback(
    (delta: number) => {
      setSidebarWidth(sidebarWidth + delta)
    },
    [sidebarWidth, setSidebarWidth],
  )

  const handleListResize = useCallback(
    (delta: number) => {
      setListWidth(listWidth + delta)
    },
    [listWidth, setListWidth],
  )

  return (
    <div className={styles.layout}>
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
