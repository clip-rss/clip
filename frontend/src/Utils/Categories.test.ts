import { describe, it, expect } from 'vitest'
import { parseCategories } from './Categories'

describe('parseCategories', () => {
  it('解析正常 JSON 数组', () => {
    expect(parseCategories('["Tech","Go"]')).toEqual(['Tech', 'Go'])
  })

  it('空串返回空数组', () => {
    expect(parseCategories('')).toEqual([])
  })

  it('非法 JSON 返回空数组而不抛异常', () => {
    expect(parseCategories('["unclosed')).toEqual([])
    expect(parseCategories('not json at all')).toEqual([])
  })

  it('非数组结构返回空数组', () => {
    expect(parseCategories('"single"')).toEqual([])
    expect(parseCategories('{"a":1}')).toEqual([])
    expect(parseCategories('null')).toEqual([])
    expect(parseCategories('42')).toEqual([])
  })

  it('丢弃非字符串项', () => {
    expect(parseCategories('["Tech",1,null,true,["x"],"Go"]')).toEqual([
      'Tech',
      'Go',
    ])
  })

  it('去除首尾空白并丢弃空白项', () => {
    expect(parseCategories('["  Tech  ","","   ","Go"]')).toEqual([
      'Tech',
      'Go',
    ])
  })

  it('按大小写不敏感去重，保留首次出现的写法与源顺序', () => {
    expect(parseCategories('["Tech","tech","TECH","Go"]')).toEqual([
      'Tech',
      'Go',
    ])
  })

  it('保留中文标签', () => {
    expect(parseCategories('["科技","前端"]')).toEqual(['科技', '前端'])
  })

  it('不解释 HTML，原样返回（渲染层由 React 转义）', () => {
    expect(parseCategories('["<script>alert(1)</script>"]')).toEqual([
      '<script>alert(1)</script>',
    ])
  })
})
