import { ArticleList, FocusMode, Layout, ReadingView, Sidebar, Toolbar } from './Components'
import { useFocusHotkey } from './Hooks'

function App() {
  useFocusHotkey()

  return (
    <>
      <Layout
        toolbar={<Toolbar />}
        sidebar={<Sidebar />}
        list={<ArticleList />}
        reader={<ReadingView />}
      />
      <FocusMode />
    </>
  )
}

export default App
