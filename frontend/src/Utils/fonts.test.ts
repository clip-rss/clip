import { describe, it, expect } from 'vitest'
import {
  FONT_ASSET_PREFIX,
  fontFaceFamily,
  fontAssetURL,
  fontFormat,
  buildFontFaceCss,
} from './fonts'
import type { FontCatalog, InstalledFont } from '../Types'

function catalogWith(
  fonts: Array<{
    id: string
    files: Array<{ weight: number; file: string }>
  }>,
): FontCatalog {
  return {
    version: 1,
    release: 'fonts-v1',
    fonts: fonts.map((f) => ({
      id: f.id,
      name: f.id,
      family: f.id,
      version: '1',
      license: 'SIL OFL 1.1',
      licenseFile: `${f.id}/OFL.txt`,
      url: 'https://example.invalid/releases/latest/download',
      files: f.files.map((x) => ({
        style: '',
        weight: x.weight,
        format: 'ttf',
        file: x.file,
        size: 1,
        sha256: 'a',
      })),
    })),
  }
}

function installed(ids: string[]): InstalledFont[] {
  return ids.map((id) => ({
    id,
    files: [{ name: `${id}.ttf`, size: 1 }],
    totalSize: 1,
  }))
}

describe('字体资产命名', () => {
  it('合成家族名与下载地址前缀与后端契约一致', () => {
    expect(fontFaceFamily('lxgw-wenkai')).toBe('clip-font-lxgw-wenkai')
    expect(FONT_ASSET_PREFIX).toBe('/__clip/font')
    expect(fontAssetURL('lxgw-wenkai', 'LXGWWenKai-Regular.ttf')).toBe(
      '/__clip/font/lxgw-wenkai/LXGWWenKai-Regular.ttf',
    )
  })

  it('扩展名 → format()', () => {
    expect(fontFormat('a.ttf')).toBe('truetype')
    expect(fontFormat('a.otf')).toBe('opentype')
    expect(fontFormat('a.woff2')).toBe('woff2')
    expect(fontFormat('a.woff')).toBe('woff')
    expect(fontFormat('a.bin')).toBe('')
  })
})

describe('buildFontFaceCss', () => {
  it('清单缺失时返回空串（不注入）', () => {
    expect(buildFontFaceCss(null, installed(['a']))).toBe('')
  })

  it('未下载的字体族不生成规则', () => {
    const css = buildFontFaceCss(
      catalogWith([
        { id: 'lxgw-wenkai', files: [{ weight: 400, file: 'x.ttf' }] },
      ]),
      installed([]),
    )
    expect(css).toBe('')
  })

  it('已下载字体族按字重各生成一条，家族名用合成名', () => {
    const css = buildFontFaceCss(
      catalogWith([
        {
          id: 'lxgw-wenkai',
          files: [
            { weight: 300, file: 'LXGWWenKai-Light.ttf' },
            { weight: 400, file: 'LXGWWenKai-Regular.ttf' },
          ],
        },
      ]),
      installed(['lxgw-wenkai']),
    )
    expect(css).toContain('font-family:"clip-font-lxgw-wenkai"')
    expect(css).toContain('font-weight:300')
    expect(css).toContain('font-weight:400')
    expect(css).toContain('font-display:swap')
    expect(css.split('@font-face').length - 1).toBe(2)
  })

  // release 清单的 file 带目录前缀，落盘是裸名 —— URL 必须用 basename。
  it('清单里的目录前缀被剥掉（URL 用裸文件名）', () => {
    const css = buildFontFaceCss(
      catalogWith([
        {
          id: 'lxgw-wenkai',
          files: [{ weight: 400, file: 'lxgw-wenkai/LXGWWenKai-Regular.ttf' }],
        },
      ]),
      installed(['lxgw-wenkai']),
    )
    expect(css).toContain(
      'url("/__clip/font/lxgw-wenkai/LXGWWenKai-Regular.ttf")',
    )
    expect(css).toContain('format("truetype")')
  })

  // 清单内容来自远端，绝不能让它破坏 CSS（引号 / 斜杠 / 空格一律丢弃该条）。
  it('非法 id 或文件名被丢弃，不生成规则', () => {
    const css = buildFontFaceCss(
      catalogWith([
        {
          id: 'bad"id',
          files: [{ weight: 400, file: 'x.ttf' }],
        },
        {
          id: 'ok',
          files: [
            { weight: 400, file: 'a"b.ttf' },
            { weight: 400, file: 'has space.ttf' },
          ],
        },
      ]),
      installed(['bad"id', 'ok']),
    )
    expect(css).toBe('')
  })
})
