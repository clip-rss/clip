import { describe, it, expect } from 'vitest'
import { clampRatio, scrollFraction, restoreScrollFraction } from './Scroll'

// 本版 jsdom 不提供 window.matchMedia，恢复/快照内部会调 prefersReducedMotion，必须自备。
window.matchMedia = (() => ({
  media: '(prefers-reduced-motion: reduce)',
  matches: false,
  onchange: null,
  addEventListener: () => {},
  removeEventListener: () => {},
  addListener: () => {},
  removeListener: () => {},
  dispatchEvent: () => false,
})) as unknown as typeof window.matchMedia

/** 伪造滚动容器：只暴露快照/恢复用到、且 jsdom 未实现的成员。 */
function fakeContainer(props: {
  scrollTop?: number
  scrollHeight?: number
  clientHeight?: number
  scrollTo?: (options: ScrollToOptions) => void
}): HTMLElement {
  return {
    scrollTop: props.scrollTop ?? 0,
    scrollHeight: props.scrollHeight ?? 0,
    clientHeight: props.clientHeight ?? 0,
    scrollTo: props.scrollTo ?? (() => {}),
  } as unknown as HTMLElement
}

describe('clampRatio', () => {
  it('范围内原样返回', () => {
    expect(clampRatio(0)).toBe(0)
    expect(clampRatio(0.5)).toBe(0.5)
    expect(clampRatio(1)).toBe(1)
  })

  it('越界截断到 [0,1]', () => {
    expect(clampRatio(-0.5)).toBe(0)
    expect(clampRatio(1.5)).toBe(1)
  })

  it('非有限值用 fallback 兜底（默认 0）', () => {
    expect(clampRatio(NaN)).toBe(0)
    expect(clampRatio(Infinity)).toBe(0)
    expect(clampRatio(NaN, 0.4)).toBe(0.4)
  })
})

describe('scrollFraction', () => {
  it('容器不可滚动（scrollHeight <= clientHeight）时返回 null', () => {
    expect(
      scrollFraction(
        fakeContainer({ scrollTop: 0, scrollHeight: 800, clientHeight: 800 }),
      ),
    ).toBeNull()
  })

  it('按进度比例返回，越界截断到 [0,1]', () => {
    // 可滚动量 = scrollHeight - clientHeight = 400；scrollTop 200 即一半。
    expect(
      scrollFraction(
        fakeContainer({
          scrollTop: 200,
          scrollHeight: 1200,
          clientHeight: 800,
        }),
      ),
    ).toBe(0.5)
    expect(
      scrollFraction(
        fakeContainer({
          scrollTop: 9999,
          scrollHeight: 1200,
          clientHeight: 800,
        }),
      ),
    ).toBe(1)
  })
})

describe('restoreScrollFraction', () => {
  it('把截断后的比例换算成像素滚动位置', () => {
    const tops: number[] = []
    const el = fakeContainer({
      scrollHeight: 1200,
      clientHeight: 800,
      scrollTo: (o) => tops.push(o.top ?? 0),
    })
    restoreScrollFraction(el, 0.5)
    expect(tops).toEqual([200])
  })

  it('越界比例被 clamp 后再换算', () => {
    const tops: number[] = []
    const el = fakeContainer({
      scrollHeight: 1200,
      clientHeight: 800,
      scrollTo: (o) => tops.push(o.top ?? 0),
    })
    restoreScrollFraction(el, 3)
    expect(tops).toEqual([400])
  })

  it('不可滚动时静默不动', () => {
    const el = fakeContainer({ scrollHeight: 800, clientHeight: 800 })
    expect(() => restoreScrollFraction(el, 1)).not.toThrow()
  })
})
