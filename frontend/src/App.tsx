import { ArticleList, Layout, ReadingView, Sidebar, Toolbar } from './Components'

function App() {
  return (
    <Layout
      toolbar={<Toolbar />}
      sidebar={<Sidebar />}
      list={<ArticleList />}
      reader={<ReadingView />}
    />
  )
}

export default App
