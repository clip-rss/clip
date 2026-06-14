import { Layout, Toolbar } from './Components'

function App() {
  return (
    <Layout
      toolbar={<Toolbar />}
      sidebar={
        <div className="p-4 text-text-secondary">
          <p>左侧栏 - 源与文件夹</p>
        </div>
      }
      list={
        <div className="p-4 text-text-secondary">
          <p>中间栏 - 文章列表</p>
        </div>
      }
      reader={
        <div className="p-4 text-text-secondary">
          <p>右侧栏 - 阅读视图</p>
        </div>
      }
    />
  )
}

export default App
