import '@testing-library/jest-dom'

// Mock GraphQL operations
global.matchMedia =
  global.matchMedia ||
  function (query) {
    return {
      matches: false,
      media: query,
      onchange: null,
      addListener: () => {},
      removeListener: () => {},
      addEventListener: () => {},
      removeEventListener: () => {},
      dispatchEvent: () => {}
    }
  }

// Mock IntersectionObserver
global.IntersectionObserver = class IntersectionObserver {
  root: Element | null = null
  rootMargin: string = ''
  thresholds: ReadonlyArray<number> = []

  constructor() {
    // Required for IntersectionObserver
  }

  observe() {
    return null
  }

  disconnect() {
    return null
  }

  unobserve() {
    return null
  }

  takeRecords(): IntersectionObserverEntry[] {
    return []
  }
}

// Mock ResizeObserver
global.ResizeObserver = class ResizeObserver {
  constructor() {
    // Required for ResizeObserver
  }

  observe() {
    return null
  }

  disconnect() {
    return null
  }

  unobserve() {
    return null
  }
}
