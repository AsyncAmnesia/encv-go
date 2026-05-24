import { root } from '@lynx-js/react'
import { PlayerApp } from './PlayerApp'
import './player.css'

root.render(<PlayerApp />)

if (import.meta.webpackHot) {
  import.meta.webpackHot.accept()
}
