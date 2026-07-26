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
import { useAppHotkeys, useDockBadge, useNotificationNavigation } from './Hooks'
import './I18n'
import i18next from 'i18next'
import { useSettingsStore, useUpdateStore } from './Stores'
import { CheckForUpdatesSilent } from '../bindings/github.com/clip-rss/clip/api/systemservice'
import { Events } from '@wailsio/runtime'

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
  useDockBadge()

  useEffect(() => {
    const store = useSettingsStore.getState()
    store.load().then(() => {
      const lang = useSettingsStore.getState().settings?.language
      if (lang) i18next.changeLanguage(lang)
    })

    // 监听更新可用事件
    const unsubscribe = Events.On('clip:update:available', () => {
      useUpdateStore.getState().setUpdateAvailable(true)
    })

    // 启动时静默检查更新
    CheckForUpdatesSilent().catch(() => {
      // 静默失败，不影响用户体验
    })

    return () => {
      unsubscribe()
    }
  }, [])

  return (
    <>
      <Layout
        toolbar={
          <Toolbar onAddFeed={openAddFeed} onOpenSettings={openSettings} />
        }
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
