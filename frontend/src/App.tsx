import { ArticleList, Layout, Sidebar, Toolbar } from './Components'
import { useArticleStore } from './Stores'

function App() {
  return (
    <Layout
      toolbar={<Toolbar />}
      sidebar={<Sidebar />}
      list={<ArticleList />}
      reader={<ReaderPlaceholder />}
    />
  )
}

// 临时占位：展示当前选中的文章，验证「点击文章 → 右侧加载」通路（阶段 10 替换为真实阅读视图）
function ReaderPlaceholder(): JSX.Element {
  const selectedItemId = useArticleStore((s) => s.selectedItemId)
  const item = useArticleStore((s) => s.items.find((it) => it.id === s.selectedItemId))

  if (selectedItemId === null || !item) {
    return (
      <div className="p-4 text-text-secondary">
        <p>右侧栏 - 阅读视图</p>
        <p className="mt-2">选择一篇文章以阅读</p>
      </div>
    )
  }

  return (
    <div className="p-6 max-w-[680px] mx-auto">
      <h1 className="text-2xl font-bold text-text-primary">{item.title}</h1>
      {item.author ? <p className="mt-2 text-sm text-text-secondary">{item.author}</p> : null}
      <p className="mt-4 text-text-primary whitespace-pre-wrap">{item.summary}</p>
    </div>
  )
}

export default App
