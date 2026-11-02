// Mirrors the inline script in index.html; applies the stored scheme.
export function applyStoredScheme(): void {
  try {
    const v = localStorage.getItem('lc.scheme')
    if (v === 'light' || v === 'dark') document.documentElement.dataset.scheme = v
    else delete document.documentElement.dataset.scheme
  } catch {
    /* storage blocked: follow the system */
  }
}
