import Skeleton from '../Skeleton/Skeleton'
import styles from './ArticleList.module.scss'

function ArticleListSkeleton(): JSX.Element {
  return (
    <div
      className={styles.skeletonContainer}
      aria-busy="true"
      aria-label="加载中"
    >
      {Array.from({ length: 8 }).map((_, i) => (
        <div key={i} className={styles.skeletonRow}>
          <Skeleton width={8} height={8} className={styles.skeletonDot} />
          <div className={styles.skeletonBody}>
            <Skeleton width="80%" height={18} />
            <Skeleton
              width="95%"
              height={14}
              className={styles.skeletonSummary}
            />
            <Skeleton width="40%" height={12} />
          </div>
        </div>
      ))}
    </div>
  )
}

export default ArticleListSkeleton
