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
import { useAppHotkeys } from './Hooks'

function App() {
  const [addOpen, setAddOpen] = useState(false)

  function openAddFeed(): void {
    setAddOpen(true)
  }

  useAppHotkeys({ onAddFeed: openAddFeed })

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
