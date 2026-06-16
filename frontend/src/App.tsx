import { Layout, Sidebar, Toolbar } from './Components'
import { useSidebarStore } from './Stores'

function App() {
  return (
    <Layout
      toolbar={<Toolbar />}
      sidebar={<Sidebar />}
      list={<ListPlaceholder />}
      reader={
        <div className="p-4 text-text-secondary">
          <p>右侧栏 - 阅读视图</p>
        </div>
      }
    />
  )
}

// 临时占位：展示当前左侧栏选中项，验证「点击源 → 中间栏更新」通路（阶段 09 替换为真实列表）
function ListPlaceholder(): JSX.Element {
  const selection = useSidebarStore((s) => s.selection)
  const categories = useSidebarStore((s) => s.categories)
  const feeds = useSidebarStore((s) => s.feeds)

  let label = '全部文章'
  if (selection.kind === 'feed') {
    label = feeds.find((f) => f.id === selection.id)?.title ?? `订阅源 #${selection.id}`
  } else if (selection.kind === 'category') {
    label = categories.find((c) => c.id === selection.id)?.name ?? `文件夹 #${selection.id}`
  }

  return (
    <div className="p-4 text-text-secondary">
      <p>中间栏 - 文章列表</p>
      <p className="mt-2 text-text-primary">已选中：{label}</p>
    </div>
  )
}

export default App
