import styles from './Skeleton.module.scss'

interface SkeletonProps {
  width?: string | number
  height?: string | number
  className?: string
}

function Skeleton(props: SkeletonProps): JSX.Element {
  const { width, height, className } = props
  return (
    <div
      className={`${styles.skeleton} ${className || ''}`}
      style={{
        width: width ?? '100%',
        height: height ?? '1em',
      }}
      aria-hidden="true"
    />
  )
}

export default Skeleton
