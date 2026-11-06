import { ApiError, OFFLINE, getMe, listLaunches } from './api'

afterEach(() => vi.unstubAllGlobals())

test('a rejected fetch becomes an offline ApiError with a next step in the message', async () => {
  vi.stubGlobal('fetch', vi.fn(() => Promise.reject(new TypeError('Failed to fetch'))))
  const err = await getMe().catch((e) => e)
  expect(err).toBeInstanceOf(ApiError)
  expect(err.status).toBe(0)
  expect(err.code).toBe('offline')
  expect(err.message).toBe(OFFLINE)
  expect(err.message).not.toMatch(/Failed to fetch/)
})

test('a non-JSON error body falls back to the status text, a non-JSON success body to an empty object', async () => {
  vi.stubGlobal('fetch', vi.fn(() => Promise.resolve(new Response('<html>Bad gateway</html>', { status: 502, statusText: 'Bad Gateway' }))))
  const err = await getMe().catch((e) => e)
  expect(err).toBeInstanceOf(ApiError)
  expect(err.status).toBe(502)
  expect(err.code).toBe('error')
  expect(err.message).toBe('Bad Gateway')

  vi.stubGlobal('fetch', vi.fn(() => Promise.resolve(new Response(null, { status: 204 }))))
  expect(await getMe()).toEqual({})
})

test('a captain call that answers 401 sign_in sends the browser to the Buffer sign-in and back here', async () => {
  const assign = vi.fn()
  vi.stubGlobal('location', { ...window.location, pathname: '/captain/repos', search: '?x=1', assign })
  vi.stubGlobal('fetch', vi.fn(() => Promise.resolve(new Response(JSON.stringify({ error: 'sign_in', message: 'Sign in' }), { status: 401 }))))
  const err = await listLaunches().catch((e) => e)
  expect(err).toBeInstanceOf(ApiError)
  expect(err.status).toBe(401)
  expect(assign).toHaveBeenCalledWith('/auth/buffer/start?return_to=%2Fcaptain%2Frepos%3Fx%3D1')
})

test('a 401 with another code does not redirect', async () => {
  const assign = vi.fn()
  vi.stubGlobal('location', { ...window.location, assign })
  vi.stubGlobal('fetch', vi.fn(() => Promise.resolve(new Response(JSON.stringify({ error: 'reconnect' }), { status: 401 }))))
  await expect(listLaunches()).rejects.toMatchObject({ status: 401, code: 'reconnect' })
  expect(assign).not.toHaveBeenCalled()
})
