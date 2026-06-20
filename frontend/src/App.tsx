import { useState } from 'react'
import {
  AddFeedModal,
  ArticleList,
  FocusMode,
  Layout,
  ReadingView,
  Sidebar,
  Toolbar,
} from './Components'
import { useAppHotkeys, useNotificationNavigation } from './Hooks'

function App() {
  const [addOpen, setAddOpen] = useState(false)

  function openAddFeed(): void {
    setAddOpen(true)
  }

  useAppHotkeys({ onAddFeed: openAddFeed })
  useNotificationNavigation()

  return (
    <>
      <Layout
        toolbar={<Toolbar onAddFeed={openAddFeed} />}
        sidebar={<Sidebar onAddFeed={openAddFeed} />}
        list={<ArticleList />}
        reader={<ReadingView />}
      />
      <FocusMode />
      <AddFeedModal open={addOpen} onOpenChange={setAddOpen} />
    </>
  )
}

export default App
