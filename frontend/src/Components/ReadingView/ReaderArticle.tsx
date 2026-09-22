import { useMemo } from 'react'
import { useTranslation } from 'react-i18next'
import {
  articleBody,
  formatRelativeTime,
  parseCategories,
  type ReaderContentStyle,
} from '../../Utils'
import { useArticleStore } from '../../Stores'
import type { Item } from '../../Types'
import ReaderContent from './ReaderContent'
import styles from './ReadingView.module.scss'

interface ReaderArticleProps {
  item: Item
  sourceName: string
  contentStyle: ReaderContentStyle
  onImageClick: (src: string) => void
  onVideoClick: (src: string) => void
  onLinkHover?: (url: string | null) => void
}

/** 文章正文主体（标题 + 元信息 + 正文 + 结尾提示），供阅读视图与专注模式复用。 */
function ReaderArticle(props: ReaderArticleProps): JSX.Element {
  const { t } = useTranslation()
  const {
    item,
    sourceName,
    contentStyle,
    onImageClick,
    onVideoClick,
    onLinkHover,
  } = props
  const tags = useMemo(
    () => parseCategories(item.categories),
    [item.categories],
  )
  // 提取过全文就显示全文，手动切回摘要时显示 RSS 那份。判定在 Utils/ArticleBody，
  // 与工具栏按钮共用同一条规则。
  const showSummary = useArticleStore((s) => s.showSummary)
  const body = articleBody(item, showSummary)

  return (
    <div className={styles.article} style={{ maxWidth: contentStyle.maxWidth }}>
      <h1
        className={styles.title}
        style={{ fontFamily: contentStyle.fontFamily }}
      >
        {item.title}
      </h1>
      <div className={styles.meta}>
        {item.author ? <span>{item.author}</span> : null}
        {item.author ? <span className={styles.metaDot}>·</span> : null}
        <span>{formatRelativeTime(item.publishedAt)}</span>
        {sourceName ? <span className={styles.metaDot}>·</span> : null}
        {sourceName ? <span>{sourceName}</span> : null}
      </div>
      {tags.length > 0 ? (
        <div className={styles.tags}>
          {tags.map((tag) => (
            <span key={tag} className={styles.tag} title={tag}>
              {tag}
            </span>
          ))}
        </div>
      ) : null}
      <div className={styles.divider} />
      <ReaderContent
        html={body}
        style={contentStyle}
        articleUrl={item.url}
        onImageClick={onImageClick}
        onVideoClick={onVideoClick}
        onLinkHover={onLinkHover}
      />
      {/* 正在看摘要、而全文已经在库里时，「已是全部内容」是句假话。 */}
      <div className={styles.endHint}>
        {showSummary && item.fullContent
          ? t('reader.fullText.summaryOnly')
          : t('reader.endOfContent')}
      </div>
    </div>
  )
}

export default ReaderArticle
