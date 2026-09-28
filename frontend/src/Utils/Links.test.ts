import { describe, it, expect } from 'vitest'
import { resolveLink } from './Links'

const ARTICLE = 'https://www.appinn.com/10-useful-github-tools-26919/'

describe('resolveLink', () => {
  it('绝对地址原样交给浏览器', () => {
    expect(resolveLink('https://x.com/a', ARTICLE)).toEqual({
      kind: 'external',
      url: 'https://x.com/a',
    })
  })

  it('相对地址按文章地址解析（issue #7 根因的另一半）', () => {
    expect(resolveLink('/tools/1', ARTICLE)).toEqual({
      kind: 'external',
      url: 'https://www.appinn.com/tools/1',
    })
    expect(resolveLink('../other/2', ARTICLE)).toEqual({
      kind: 'external',
      url: 'https://www.appinn.com/other/2',
    })
  })

  it('协议相对地址（//host/...）也能解析', () => {
    expect(resolveLink('//x.com/a', ARTICLE)).toEqual({
      kind: 'external',
      url: 'https://x.com/a',
    })
  })

  it('纯锚点是页内跳转，不是外链', () => {
    expect(resolveLink('#toc-1', ARTICLE)).toEqual({
      kind: 'anchor',
      id: 'toc-1',
    })
  })

  it('锚点的 fragment 要解码成 DOM 里 id 的原文', () => {
    expect(resolveLink('#%E7%9B%AE%E5%BD%95', ARTICLE)).toEqual({
      kind: 'anchor',
      id: '目录',
    })
    expect(resolveLink('#a%20b', ARTICLE)).toEqual({
      kind: 'anchor',
      id: 'a b',
    })
  })

  it('指回本文档自身的地址同样算页内锚点', () => {
    expect(resolveLink(`${ARTICLE}#toc-1`, ARTICLE)).toEqual({
      kind: 'anchor',
      id: 'toc-1',
    })
  })

  it('同源但不同文章的地址仍是外链', () => {
    expect(resolveLink('/other-post#toc-1', ARTICLE)).toEqual({
      kind: 'external',
      url: 'https://www.appinn.com/other-post#toc-1',
    })
  })

  it('非 http(s) 协议一律判废（javascript: 在清洗阶段已被剥掉，这里是第二道）', () => {
    for (const href of [
      'javascript:alert(1)',
      'data:text/html,<h1>x</h1>',
      'mailto:a@b.com?subject=hi there',
      'file:///C:/a.txt',
      'ftp://x.com/a',
    ]) {
      expect(resolveLink(href, ARTICLE)).toEqual({ kind: 'invalid' })
    }
  })

  it('空 href、空 fragment 判废', () => {
    expect(resolveLink('', ARTICLE)).toEqual({ kind: 'invalid' })
    expect(resolveLink('   ', ARTICLE)).toEqual({ kind: 'invalid' })
    expect(resolveLink('#', ARTICLE)).toEqual({ kind: 'invalid' })
    expect(resolveLink('#%20', ARTICLE)).toEqual({ kind: 'invalid' })
  })

  it('没有基准时相对地址解析不出来，判废', () => {
    expect(resolveLink('/a/b', '')).toEqual({ kind: 'invalid' })
    expect(resolveLink('a.png', '')).toEqual({ kind: 'invalid' })
  })

  it('基准非法时相对地址判废，但绝对地址照常放行', () => {
    expect(resolveLink('/a/b', '::not a url::')).toEqual({ kind: 'invalid' })
    expect(resolveLink('https://x.com/a', '::not a url::')).toEqual({
      kind: 'external',
      url: 'https://x.com/a',
    })
  })

  it('external 永远是带 host 的绝对 http(s) 地址', () => {
    const hrefs = [
      'https://x.com/a',
      '/a/b',
      '//x.com/a',
      '?page=2',
      'a b.html',
      ARTICLE,
      '#toc',
      'javascript:alert(1)',
    ]

    for (const href of hrefs) {
      const out = resolveLink(href, ARTICLE)
      if (out.kind !== 'external') continue
      const parsed = new URL(out.url)
      expect(['http:', 'https:']).toContain(parsed.protocol)
      expect(parsed.host).not.toBe('')
    }
  })

  it('入参前后空白不影响判定（属性值里带换行的正文不少见）', () => {
    expect(resolveLink('  #toc-1\n', ARTICLE)).toEqual({
      kind: 'anchor',
      id: 'toc-1',
    })
    expect(resolveLink(' https://x.com/a ', ARTICLE)).toEqual({
      kind: 'external',
      url: 'https://x.com/a',
    })
  })
})
