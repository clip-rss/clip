import { describe, it, expect } from 'vitest'
import {
  absoluteMediaUrl,
  mediaProxyUrl,
  originOf,
  rewriteSrcsetValue,
} from './Media'

const ARTICLE = 'https://sspai.com/post/114823'
const IMAGE = 'https://cdnfile.sspai.com/2026/09/21/a.png'

describe('mediaProxyUrl', () => {
  it('把远程图片换成代理地址，并带上文章源站', () => {
    const out = mediaProxyUrl(IMAGE, ARTICLE)

    expect(out.startsWith('/__clip/media?u=')).toBe(true)
    expect(out).toContain(encodeURIComponent(IMAGE))
    expect(out).toContain(`r=${encodeURIComponent('https://sspai.com')}`)
  })

  it('相对地址按文章地址解析后再代理', () => {
    const out = mediaProxyUrl('/img/a.png', ARTICLE)

    expect(out).toContain(encodeURIComponent('https://sspai.com/img/a.png'))
  })

  it('协议相对地址（//host/...）也能解析', () => {
    const out = mediaProxyUrl('//cdn.example.com/a.png', ARTICLE)

    expect(out).toContain(encodeURIComponent('https://cdn.example.com/a.png'))
  })

  it('没有文章地址时不代理（更新日志弹窗等无文章上下文的场景）', () => {
    expect(mediaProxyUrl(IMAGE, '')).toBe(IMAGE)
  })

  it('data: 地址原样返回', () => {
    const inline = 'data:image/gif;base64,R0lGODlhAQABAAAAACw='

    expect(mediaProxyUrl(inline, ARTICLE)).toBe(inline)
  })

  it('相对地址但文章地址非法时不代理', () => {
    expect(mediaProxyUrl('/img/a.png', '::not a url::')).toBe('/img/a.png')
  })

  it('幂等：已是代理地址（相对形式）不再套一层', () => {
    const once = mediaProxyUrl(IMAGE, ARTICLE)

    expect(mediaProxyUrl(once, ARTICLE)).toBe(once)
  })

  it('幂等：已是代理地址（绝对形式，视频点击拿到的是这种）不再套一层', () => {
    const absolute = `http://wails.localhost${mediaProxyUrl(IMAGE, ARTICLE)}`

    expect(mediaProxyUrl(absolute, ARTICLE)).toBe(absolute)
  })
})

describe('absoluteMediaUrl', () => {
  it('绝对地址原样返回', () => {
    expect(absoluteMediaUrl(IMAGE, ARTICLE)).toBe(IMAGE)
  })

  it('相对地址按基准解析', () => {
    expect(absoluteMediaUrl('a.png', 'https://site.com/post/1')).toBe(
      'https://site.com/post/a.png',
    )
  })

  it('没有基准时解析不出来，返回空串', () => {
    expect(absoluteMediaUrl('a.png', '')).toBe('')
  })
})

describe('originOf', () => {
  it('取源站并去掉路径', () => {
    expect(originOf(ARTICLE)).toBe('https://sspai.com')
  })

  it('保留非默认端口', () => {
    expect(originOf('http://example.com:8080/a/b')).toBe(
      'http://example.com:8080',
    )
  })

  it('非 http(s) 协议返回空串（data: 的 origin 是字符串 "null"）', () => {
    expect(originOf('data:image/png;base64,AAAA')).toBe('')
    expect(originOf('')).toBe('')
    expect(originOf('不是 URL')).toBe('')
  })
})

describe('rewriteSrcsetValue', () => {
  it('逐个候选改写并保留描述符', () => {
    const out = rewriteSrcsetValue(
      'https://cdn.com/1.png 1x, https://cdn.com/2.png 2x',
      ARTICLE,
    )

    expect(out).toContain(encodeURIComponent('https://cdn.com/1.png'))
    expect(out).toContain(encodeURIComponent('https://cdn.com/2.png'))
    expect(out).toContain('1x')
    expect(out).toContain('2x')
    // 两个候选之间仍是逗号分隔
    expect(out.split(',').length).toBe(2)
  })

  it('没有文章地址时原样返回', () => {
    const srcset = 'https://cdn.com/1.png 1x'

    expect(rewriteSrcsetValue(srcset, '')).toBe(srcset)
  })
})
