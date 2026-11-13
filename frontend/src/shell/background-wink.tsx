import './background-wink.css'

/** Eye-only layer. The shirt mark stays on the logo layer. */
export function BackgroundWink() {
  return (
    <div className="stage-wink" aria-hidden="true">
      <div className="stage-wink-open" />
      <div className="stage-wink-closed" />
    </div>
  )
}
