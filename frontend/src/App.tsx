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
import './I18n'
import i18next from 'i18next'
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

  useEffect(() => {
    const store = useSettingsStore.getState()
    store.load().then(() => {
      const lang = useSettingsStore.getState().settings?.language
      if (lang) i18next.changeLanguage(lang)
    })
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
