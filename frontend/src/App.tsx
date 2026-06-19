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
import { useFocusHotkey } from './Hooks'

function App() {
  useFocusHotkey()
  const [addOpen, setAddOpen] = useState(false)

  function openAddFeed(): void {
    setAddOpen(true)
  }

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
