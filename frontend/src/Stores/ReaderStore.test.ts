import { describe, it, expect, beforeEach } from 'vitest'
import { useReaderStore } from './ReaderStore'

beforeEach(() => {
  useReaderStore.setState({
    fontFamily: 'sans',
    fontSize: 16,
    lineHeight: 1.8,
    width: '640',
    background: 'default',
  })
})

describe('ReaderStore', () => {
  it('默认偏好', () => {
    const s = useReaderStore.getState()
    expect(s.fontFamily).toBe('sans')
    expect(s.fontSize).toBe(16)
    expect(s.lineHeight).toBe(1.8)
    expect(s.width).toBe('640')
    expect(s.background).toBe('default')
  })

  it('setters 更新偏好', () => {
    const st = useReaderStore.getState()
    st.setFontFamily('serif')
    st.setFontSize(18)
    st.setLineHeight(2.0)
    st.setWidth('full')
    st.setBackground('sepia')

    const s = useReaderStore.getState()
    expect(s.fontFamily).toBe('serif')
    expect(s.fontSize).toBe(18)
    expect(s.lineHeight).toBe(2.0)
    expect(s.width).toBe('full')
    expect(s.background).toBe('sepia')
  })
})
