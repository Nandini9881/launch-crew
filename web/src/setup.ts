import '@testing-library/jest-dom/vitest'

// Always use an in-memory localStorage: Node 26 ships a broken global, and jsdom's
// real Storage cannot be spied on, so one shim keeps tests identical on every Node.
class MemStorage {
  private mem = new Map<string, string>()
  getItem(k: string) {
    return this.mem.get(k) ?? null
  }
  setItem(k: string, v: string) {
    this.mem.set(k, String(v))
  }
  removeItem(k: string) {
    this.mem.delete(k)
  }
  clear() {
    this.mem.clear()
  }
}
Object.defineProperty(globalThis, 'localStorage', { value: new MemStorage(), configurable: true })

beforeEach(() => {
  localStorage.clear()
  delete document.documentElement.dataset.scheme
})
