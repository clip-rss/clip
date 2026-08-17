import { useMemo } from 'react'
import { useTranslation } from 'react-i18next'
import {
  formatRelativeTime,
  parseCategories,
  type ReaderContentStyle,
} from '../../Utils'
import type { Item } from '../../Types'
import ReaderContent from './ReaderContent'
import styles from './ReadingView.module.scss'

interface ReaderArticleProps {
  item: Item
  sourceName: string
  contentStyle: ReaderContentStyle
  onImageClick: (src: string) => void
}

/** 文章正文主体（标题 + 元信息 + 正文 + 结尾提示），供阅读视图与专注模式复用。 */
function ReaderArticle(props: ReaderArticleProps): JSX.Element {
  const { t } = useTranslation()
  const { item, sourceName, contentStyle, onImageClick } = props
  const tags = useMemo(
    () => parseCategories(item.categories),
    [item.categories],
  )

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
        html={item.content}
        style={contentStyle}
        onImageClick={onImageClick}
      />
      <div className={styles.endHint}>{t('reader.endOfContent')}</div>
    </div>
  )
}

export default ReaderArticle
