// The Launch Crew paper plane, nose to the upper right. Rotate it with the `deg` prop.
export const PLANE_PATH =
  'M12 2.2c1.05 0 1.85 1.3 1.85 3v4.1l7 3.9v2.1l-7 -2v3.5l2.35 1.85v1.55L12 19.1l-4.2 1.2v-1.55l2.35 -1.85v-3.5l-7 2v-2.1l7 -3.9V5.2c0 -1.7 0.8 -3 1.85 -3z'

export default function Plane({ deg = 45, className }: { deg?: number; className?: string }) {
  return (
    <svg className={className} viewBox="0 0 24 24" aria-hidden="true">
      <g transform={`rotate(${deg} 12 12)`}>
        <path d={PLANE_PATH} />
      </g>
    </svg>
  )
}
