package mediaproxy

import (
	"container/list"
	"sync"
)

// lruCache 是按**总字节数**（而非条目数）上限淘汰的内存缓存。
//
// 按字节淘汰是必须的：同一篇正文里的图片大小能差三个数量级，按条数限会让
// 「10 张 5 MB 的图」和「10 张 20 KB 的图」占一样多的槽位。
//
// 目的不是省流量，而是让「重开同一篇文章」不再整篇重新抓图 —— WebView 对
// http://wails.localhost 这类本地主机的响应不一定落盘缓存，光靠 HTTP 头靠不住。
type lruCache struct {
	mu    sync.Mutex
	max   int64
	size  int64
	ll    *list.List               // 队首是最新使用的
	items map[string]*list.Element // key → ll 中的元素
}

// cacheEntry 是 ll 元素的值。
type cacheEntry struct {
	key         string
	body        []byte
	contentType string
}

func newLRUCache(maxBytes int64) *lruCache {
	if maxBytes <= 0 {
		maxBytes = defaultCacheBytes
	}
	return &lruCache{
		max:   maxBytes,
		ll:    list.New(),
		items: make(map[string]*list.Element),
	}
}

// get 取一条缓存并把它移到队首（标记为最近使用）。
func (c *lruCache) get(key string) (body []byte, contentType string, ok bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	el, ok := c.items[key]
	if !ok {
		return nil, "", false
	}
	c.ll.MoveToFront(el)
	e := el.Value.(*cacheEntry)
	return e.body, e.contentType, true
}

// add 写入一条缓存，按需从队尾淘汰直到回到上限之内。
func (c *lruCache) add(key string, body []byte, contentType string) {
	// 单条就超过总上限时直接放弃：放进去会把已有条目全部挤掉，得不偿失。
	if int64(len(body)) > c.max {
		return
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	if el, ok := c.items[key]; ok {
		c.ll.MoveToFront(el)
		e := el.Value.(*cacheEntry)
		c.size += int64(len(body)) - int64(len(e.body))
		e.body, e.contentType = body, contentType
	} else {
		c.items[key] = c.ll.PushFront(&cacheEntry{key: key, body: body, contentType: contentType})
		c.size += int64(len(body))
	}

	for c.size > c.max {
		back := c.ll.Back()
		if back == nil {
			break
		}
		c.ll.Remove(back)
		e := back.Value.(*cacheEntry)
		delete(c.items, e.key)
		c.size -= int64(len(e.body))
	}
}
