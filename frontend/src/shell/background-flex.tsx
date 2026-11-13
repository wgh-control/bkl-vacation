import { useEffect, useRef } from 'react'
import peak from './background-flex.png'
import loop from './background-motion.webm'
import './background-flex.css'

/**
 * Stage loop: bicep flex, one wink at the peak, Leon mark on the shirt.
 * Reduced motion keeps the flexed still. Wink or shirt layers can sit beside this.
 */
export function StageFlex() {
  const videoRef = useRef<HTMLVideoElement>(null)

  useEffect(() => {
    const video = videoRef.current
    if (!video) return
    video.muted = true
    const media = window.matchMedia?.('(prefers-reduced-motion: reduce)')
    const sync = () => {
      if (media?.matches) {
        video.pause()
        return
      }
      if (/jsdom/i.test(navigator.userAgent)) return
      const pending = video.play?.()
      if (pending && typeof pending.catch === 'function') void pending.catch(() => undefined)
    }
    sync()
    media?.addEventListener('change', sync)
    return () => media?.removeEventListener('change', sync)
  }, [])

  return (
    <div className="stage-flex" aria-hidden="true">
      <video
        ref={videoRef}
        className="stage-flex-motion"
        src={loop}
        autoPlay
        muted
        loop
        playsInline
        preload="auto"
        tabIndex={-1}
      />
      <img className="stage-flex-still" src={peak} alt="" />
    </div>
  )
}
