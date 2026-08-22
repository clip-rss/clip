import { describe, it, expect } from 'vitest'
import { markdownToHtml } from './Markdown'

describe('markdownToHtml', () => {
  it('转换各级 ATX 标题', () => {
    expect(markdownToHtml('## 0.2.0')).toBe('<h2>0.2.0</h2>')
    expect(markdownToHtml('### 新增')).toBe('<h3>新增</h3>')
    expect(markdownToHtml('#### 2026-08-21')).toBe('<h4>2026-08-21</h4>')
    expect(markdownToHtml('# T')).toBe('<h1>T</h1>')
    expect(markdownToHtml('###### T')).toBe('<h6>T</h6>')
  })

  it('# 后缺少空格时不当作标题', () => {
    expect(markdownToHtml('#hashtag')).toBe('<p>#hashtag</p>')
  })

  // 旧实现按空行分块并只看首行，这种「标题紧跟列表」会整块退化成纯文本
  it('标题与紧随其后的列表之间没有空行时两者都能解析', () => {
    const out = markdownToHtml('### 新增\n- 甲\n- 乙')
    expect(out).toBe('<h3>新增</h3>\n<ul><li>甲</li><li>乙</li></ul>')
    expect(out).not.toContain('###')
    expect(out).not.toContain('- 甲')
  })

  it('渲染真实 CHANGELOG 片段且不残留 Markdown 标记', () => {
    const md = [
      '## 0.2.0',
      '',
      '#### 2026-08-21',
      '',
      '### 新增',
      '- 新增下载文章内图片',
      '- 新增繁体中文',
      '',
      '### 修复',
      '- 修复某些站点的问题',
    ].join('\n')
    const out = markdownToHtml(md)
    expect(out).toContain('<h2>0.2.0</h2>')
    expect(out).toContain('<h4>2026-08-21</h4>')
    expect(out).toContain('<h3>新增</h3>')
    expect(out).toContain('<h3>修复</h3>')
    expect(out).toContain('<li>新增下载文章内图片</li>')
    expect(out).toContain('<li>修复某些站点的问题</li>')
    // 两个小节各自成表，不会合并
    expect(out.match(/<ul>/g)).toHaveLength(2)
    expect(out).not.toMatch(/^\s*[-#]/m)
  })

  it('相邻的同类列表项合并为一个列表', () => {
    expect(markdownToHtml('- a\n- b\n- c')).toBe(
      '<ul><li>a</li><li>b</li><li>c</li></ul>',
    )
  })

  it('有序列表与无序列表分开成表', () => {
    const out = markdownToHtml('1. a\n2. b')
    expect(out).toBe('<ol><li>a</li><li>b</li></ol>')
    expect(markdownToHtml('- a\n1. b')).toBe(
      '<ul><li>a</li></ul>\n<ol><li>b</li></ol>',
    )
  })

  it('列表项的续行并入该项而非拆成段落', () => {
    expect(markdownToHtml('- 第一行\n  第二行')).toBe(
      '<ul><li>第一行 第二行</li></ul>',
    )
  })

  it('段落内的软换行合并为一行', () => {
    expect(markdownToHtml('一句\n二句')).toBe('<p>一句 二句</p>')
  })

  it('空行分隔多个段落', () => {
    expect(markdownToHtml('甲\n\n乙')).toBe('<p>甲</p>\n<p>乙</p>')
  })

  it('转换行内格式', () => {
    expect(markdownToHtml('**粗**')).toBe('<p><strong>粗</strong></p>')
    expect(markdownToHtml('__粗__')).toBe('<p><strong>粗</strong></p>')
    expect(markdownToHtml('~~删~~')).toBe('<p><del>删</del></p>')
    expect(markdownToHtml('*斜*')).toBe('<p><em>斜</em></p>')
    expect(markdownToHtml('`code`')).toBe('<p><code>code</code></p>')
  })

  it('链接带 target 与 rel', () => {
    const out = markdownToHtml('[清](https://x.com)')
    expect(out).toContain('href="https://x.com"')
    expect(out).toContain('target="_blank"')
    expect(out).toContain('rel="noopener noreferrer"')
    expect(out).toContain('>清</a>')
  })

  it('snake_case 不被误认为斜体', () => {
    expect(markdownToHtml('foo_bar_baz')).toBe('<p>foo_bar_baz</p>')
  })

  it('行内代码里的格式标记按字面保留', () => {
    expect(markdownToHtml('`a **b** c`')).toBe('<p><code>a **b** c</code></p>')
  })

  it('围栏代码块保留换行且不解析内部标记', () => {
    const out = markdownToHtml('```go\nif a > b {\n- x\n}\n```')
    expect(out).toBe('<pre><code>if a &gt; b {\n- x\n}</code></pre>')
    expect(out).not.toContain('<li>')
  })

  it('结尾未闭合的围栏代码块仍然输出内容', () => {
    expect(markdownToHtml('```\nabc')).toBe('<pre><code>abc</code></pre>')
  })

  it('转换引用与分隔线', () => {
    expect(markdownToHtml('> 注意')).toBe('<blockquote>注意</blockquote>')
    expect(markdownToHtml('---')).toBe('<hr />')
  })

  it('转义 HTML 特殊字符', () => {
    expect(markdownToHtml('<script>alert(1)</script>')).toBe(
      '<p>&lt;script&gt;alert(1)&lt;/script&gt;</p>',
    )
    expect(markdownToHtml('a & b')).toBe('<p>a &amp; b</p>')
  })

  it('处理空输入与 CRLF 换行', () => {
    expect(markdownToHtml('')).toBe('')
    expect(markdownToHtml('   \n\n  ')).toBe('')
    expect(markdownToHtml('## T\r\n\r\n- a\r\n')).toBe(
      '<h2>T</h2>\n<ul><li>a</li></ul>',
    )
  })
})
