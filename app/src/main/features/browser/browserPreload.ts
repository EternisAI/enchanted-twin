import { contextBridge, ipcRenderer } from 'electron'

// Secure API exposed to the webview
const browserAPI = {
  // Send page content to main process
  sendContent: (content: {
    text: string
    html: string
    metadata: {
      title: string
      description?: string
      keywords?: string[]
      author?: string
    }
  }) => {
    ipcRenderer.send('browser:content-update', content)
  },

  // Notify about navigation
  notifyNavigation: (url: string) => {
    ipcRenderer.send('browser:navigation', url)
  },

  // Notify about scroll position
  notifyScroll: (position: { x: number; y: number }) => {
    ipcRenderer.send('browser:scroll', position)
  },

  // Listen for actions from the main process
  onAction: (callback: (action: { type: string; params: Record<string, unknown> }) => void) => {
    ipcRenderer.on('browser:execute-action', (_, action) => callback(action))
  },

  // Send action results back
  sendActionResult: (
    actionId: string,
    result: { success: boolean; data?: unknown; error?: string }
  ) => {
    ipcRenderer.send('browser:action-result', { actionId, result })
  }
}

// Expose the API to the webview
contextBridge.exposeInMainWorld('browserBridge', browserAPI)

// Content extraction utilities
const extractPageContent = () => {
  const text = document.body?.innerText || ''
  const html = document.documentElement?.outerHTML || ''

  const metadata = {
    title: document.title || '',
    description:
      document.querySelector('meta[name="description"]')?.getAttribute('content') || undefined,
    keywords:
      document
        .querySelector('meta[name="keywords"]')
        ?.getAttribute('content')
        ?.split(',')
        .map((k) => k.trim()) || undefined,
    author: document.querySelector('meta[name="author"]')?.getAttribute('content') || undefined
  }

  return { text, html, metadata }
}

// Auto-extract content on page load
window.addEventListener('load', () => {
  const content = extractPageContent()
  browserAPI.sendContent(content)
})

// Monitor navigation
let lastUrl = window.location.href
const checkNavigation = () => {
  if (window.location.href !== lastUrl) {
    lastUrl = window.location.href
    browserAPI.notifyNavigation(lastUrl)

    // Re-extract content after navigation
    setTimeout(() => {
      const content = extractPageContent()
      browserAPI.sendContent(content)
    }, 1000)
  }
}

// Check for navigation changes periodically
setInterval(checkNavigation, 500)

// Monitor scroll position
let scrollTimeout: NodeJS.Timeout
window.addEventListener('scroll', () => {
  clearTimeout(scrollTimeout)
  scrollTimeout = setTimeout(() => {
    browserAPI.notifyScroll({
      x: window.scrollX,
      y: window.scrollY
    })
  }, 100)
})

// Action execution
browserAPI.onAction(async (action) => {
  const actionId = `${action.type}-${Date.now()}`

  try {
    let result: unknown = null

    switch (action.type) {
      case 'click': {
        const element = document.querySelector(action.params.selector as string) as HTMLElement
        if (element) {
          element.click()
          result = { clicked: true }
        } else {
          throw new Error(`Element not found: ${action.params.selector}`)
        }
        break
      }

      case 'input': {
        const element = document.querySelector(action.params.selector as string) as HTMLInputElement
        if (element) {
          element.value = action.params.text as string
          element.dispatchEvent(new Event('input', { bubbles: true }))
          element.dispatchEvent(new Event('change', { bubbles: true }))
          result = { inputSet: true }
        } else {
          throw new Error(`Input element not found: ${action.params.selector}`)
        }
        break
      }

      case 'scroll': {
        window.scrollTo(action.params.x as number, action.params.y as number)
        result = { scrolled: true }
        break
      }

      case 'extract': {
        if (action.params.selector) {
          const element = document.querySelector(action.params.selector as string)
          if (element) {
            result = {
              text: element.textContent,
              html: element.outerHTML
            }
          } else {
            throw new Error(`Element not found: ${action.params.selector}`)
          }
        } else {
          result = extractPageContent()
        }
        break
      }

      case 'screenshot': {
        // Screenshot is handled by the main process
        result = { requestedScreenshot: true }
        break
      }

      default:
        throw new Error(`Unknown action type: ${action.type}`)
    }

    browserAPI.sendActionResult(actionId, { success: true, data: result })
  } catch (error) {
    browserAPI.sendActionResult(actionId, {
      success: false,
      error: error instanceof Error ? error.message : 'Unknown error'
    })
  }
})
