import { render } from '@testing-library/react'
import { describe, expect, it } from 'vitest'
import { Stage } from '../components/WindowChrome'

describe('background flex', () => {
  it('loops the bicep clip and keeps the peaked still for reduced motion', () => {
    const { container } = render(
      <Stage>
        <p>shell</p>
      </Stage>,
    )
    const video = container.querySelector('video.stage-flex-motion')
    const still = container.querySelector('img.stage-flex-still')
    const photo = container.querySelector('.stage-photo')
    expect(video).toBeTruthy()
    expect(video?.getAttribute('src')).toContain('background-motion')
    expect(video?.hasAttribute('loop')).toBe(true)
    expect(video?.hasAttribute('autoplay')).toBe(true)
    expect((video as HTMLVideoElement).muted).toBe(true)
    expect(still?.getAttribute('src')).toContain('background-flex')
    expect(photo).toBeTruthy()
    expect(photo!.compareDocumentPosition(video!) & Node.DOCUMENT_POSITION_FOLLOWING).toBeTruthy()
  })
})
