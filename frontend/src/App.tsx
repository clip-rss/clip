import { useEffect, useState } from 'react'
import {
  AddFeedModal,
  ArticleList,
  FocusMode,
  Layout,
  ReadingView,
  SettingsModal,
  Sidebar,
  Toolbar,
} from './Components'
import { useAppHotkeys, useNotificationNavigation } from './Hooks'
import { useSettingsStore } from './Stores'

function App() {
  const [addOpen, setAddOpen] = useState(false)
  const [settingsOpen, setSettingsOpen] = useState(false)

  function openAddFeed(): void {
    setAddOpen(true)
  }

  function openSettings(): void {
    setSettingsOpen(true)
  }

  useAppHotkeys({ onAddFeed: openAddFeed, onOpenSettings: openSettings })
  useNotificationNavigation()

  // 应用启动时拉取全局设置（自动标记已读延迟等功能依赖）。
  useEffect(() => {
    void useSettingsStore.getState().load()
  }, [])

  return (
    <>
      <Layout
        toolbar={<Toolbar onAddFeed={openAddFeed} onOpenSettings={openSettings} />}
        sidebar={<Sidebar onAddFeed={openAddFeed} />}
        list={<ArticleList />}
        reader={<ReadingView />}
      />
      <FocusMode />
      <AddFeedModal open={addOpen} onOpenChange={setAddOpen} />
      <SettingsModal open={settingsOpen} onOpenChange={setSettingsOpen} />
    </>
  )
}

export default App
