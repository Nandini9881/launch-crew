import { readFileSync } from 'node:fs'

const css = readFileSync('src/styles/tokens.css', 'utf8')

function vars(block: string): Record<string, string> {
  return Object.fromEntries([...block.matchAll(/--(color-[\w-]+):\s*(#[0-9a-f]{6})/gi)].map((m) => [m[1], m[2]]))
}
const light = vars(css.slice(0, css.indexOf(':root[data-scheme="dark"]')))
const darkBlock = css.slice(css.indexOf(':root[data-scheme="dark"]'), css.indexOf('@media'))
const dark = { ...light, ...vars(darkBlock) }

function lum(hex: string) {
  const [r, g, b] = [1, 3, 5].map((i) => {
    const c = parseInt(hex.slice(i, i + 2), 16) / 255
    return c <= 0.03928 ? c / 12.92 : ((c + 0.055) / 1.055) ** 2.4
  })
  return 0.2126 * r + 0.7152 * g + 0.0722 * b
}
function ratio(a: string, b: string) {
  const [hi, lo] = [lum(a), lum(b)].sort((x, y) => y - x)
  return (hi + 0.05) / (lo + 0.05)
}

test.each([
  ['light', light],
  ['dark', dark],
])('%s: primary and accent buttons hold 4.5:1', (_n, t) => {
  expect(ratio(t['color-primary'], t['color-on-primary'])).toBeGreaterThanOrEqual(4.5)
  expect(ratio(t['color-accent'], t['color-on-accent'])).toBeGreaterThanOrEqual(4.5)
})

test.each([
  ['light', light],
  ['dark', dark],
])('%s: slide text on the card-ink slide holds 4.5:1', (_n, t) => {
  expect(ratio(t['color-slide-text'], t['color-card-ink'])).toBeGreaterThanOrEqual(4.5)
})

test('the media-query dark block matches the data-scheme dark block', () => {
  const media = vars(css.slice(css.indexOf('@media')))
  expect(media).toEqual(vars(darkBlock))
})
