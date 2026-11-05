import QRCode from 'qrcode'
import { useEffect, useRef, useState } from 'react'
import { useParams } from 'react-router'
import { ApiError, showInfo } from '../api'
import Wordmark from '../components/Wordmark'
import { Dead } from './shared'
import '../styles/print.css'
import './crew.css'

const SIDE = 800
const MARK = 0.12

// QR with error correction H and the accent square over the centre.
async function paint(canvas: HTMLCanvasElement, text: string) {
  await QRCode.toCanvas(canvas, text, {
    errorCorrectionLevel: 'H',
    width: SIDE,
    margin: 2,
    color: { dark: '#000000', light: '#ffffff' },
  })
  // qrcode writes inline px sizes that beat the stylesheet; let CSS size the slide.
  canvas.style.width = ''
  canvas.style.height = ''
  const ctx = canvas.getContext('2d')
  if (!ctx) return
  const size = canvas.width * MARK
  const halo = size * 1.3
  const mid = canvas.width / 2
  ctx.fillStyle = '#ffffff'
  ctx.fillRect(mid - halo / 2, mid - halo / 2, halo, halo)
  ctx.fillStyle = getComputedStyle(canvas).getPropertyValue('--color-accent').trim() || '#4a45d1'
  ctx.fillRect(mid - size / 2, mid - size / 2, size, size)
}

export default function Show() {
  const openToken = useParams().openToken ?? ''
  const [view, setView] = useState<{ title: string; shortUrl: string }>()
  const [error, setError] = useState<ApiError>()
  const [failed, setFailed] = useState(false)
  const canvas = useRef<HTMLCanvasElement>(null)

  useEffect(() => {
    let live = true
    showInfo(openToken)
      .then((v) => live && setView(v))
      .catch((e) => live && setError(e instanceof ApiError ? e : new ApiError(0, 'error', String(e))))
    return () => {
      live = false
    }
  }, [openToken])

  useEffect(() => {
    if (view && canvas.current) paint(canvas.current, view.shortUrl).catch(() => setFailed(true))
  }, [view])

  if (error) return <Dead code={error.code} />

  return (
    <main className="show">
      {view && (
        <>
          <h1 className="show-title">{view.title}</h1>
          <canvas
            ref={canvas}
            className="show-qr"
            width={SIDE}
            height={SIDE}
            role="img"
            aria-label={`QR code for ${view.shortUrl}`}
          />
          {failed && <p className="crew-note">The QR code could not be drawn. Use the link below.</p>}
          <p className="show-link">{view.shortUrl}</p>
        </>
      )}
      <footer className="show-foot">
        <Wordmark />
        <a className="show-download" href={`/api/l/${encodeURIComponent(openToken)}/show.png`} download>
          Download PNG
        </a>
      </footer>
    </main>
  )
}
