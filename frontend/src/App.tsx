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
import { migrateLegacyPrefs, useSettingsStore, useUpdateStore } from './Stores'
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
    void (async () => {
      await useSettingsStore.getState().load()
      // 主题与阅读排版原存于 localStorage，收归后端后搬一次。
      // 必须在 load() 之后：迁移是把本地旧值覆盖到后端基线上。
      await migrateLegacyPrefs()
      const lang = useSettingsStore.getState().settings?.language
      if (lang) i18next.changeLanguage(lang)
    })()

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

  // 应用 reduce-motion 设置
  useEffect(() => {
    const settings = useSettingsStore.getState().settings
    if (settings?.reduceMotion) {
      document.documentElement.classList.add('reduce-motion')
    } else {
      document.documentElement.classList.remove('reduce-motion')
    }

    const unsubscribe = useSettingsStore.subscribe((state) => {
      if (state.settings?.reduceMotion) {
        document.documentElement.classList.add('reduce-motion')
      } else {
        document.documentElement.classList.remove('reduce-motion')
      }
    })

    return unsubscribe
  }, [])

  // 应用焦点指示器设置
  useEffect(() => {
    const settings = useSettingsStore.getState().settings
    if (settings?.showFocusIndicator === false) {
      document.documentElement.classList.add('hide-focus-indicator')
    } else {
      document.documentElement.classList.remove('hide-focus-indicator')
    }

    const unsubscribe = useSettingsStore.subscribe((state) => {
      if (state.settings?.showFocusIndicator === false) {
        document.documentElement.classList.add('hide-focus-indicator')
      } else {
        document.documentElement.classList.remove('hide-focus-indicator')
      }
    })

    return unsubscribe
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
