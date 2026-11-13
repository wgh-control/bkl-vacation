import type { ReactNode } from 'react'
import logo from '../assets/brand/logo-dark.png'
import { StageFlex } from '../shell/background-flex'

export function Stage({ children }: { children: ReactNode }) {
  return (
    <div className="stage">
      <div className="stage-photo" aria-hidden="true" />
      <StageFlex />
      <div className="stage-wash" aria-hidden="true" />
      <div className="stage-ascii" aria-hidden="true" />
      {children}
    </div>
  )
}

export function TrafficLights() {
  return (
    <div className="traffic" aria-hidden="true">
      <i className="tl-close" />
      <i className="tl-min" />
      <i className="tl-zoom" />
    </div>
  )
}

export function BrandMark({ alt = 'LEON' }: { alt?: string }) {
  return <img className="brand-logo" src={logo} alt={alt} />
}
