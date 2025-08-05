export interface BrowserSession {
  id: string
  url: string
  title: string
  content: {
    text: string
    html: string
    screenshot?: string
  }
  metadata: {
    timestamp: Date
    scrollPosition: { x: number; y: number }
    viewportSize: { width: number; height: number }
  }
  interactions: BrowserInteraction[]
}

export interface BrowserInteraction {
  id: string
  type: 'navigate' | 'click' | 'input' | 'scroll' | 'extract' | 'screenshot'
  timestamp: Date
  params: Record<string, unknown>
  result?: {
    success: boolean
    data?: unknown
    error?: string
  }
}

export interface BrowserState {
  sessions: Map<string, BrowserSession>
  activeSessionId: string | null
  isLoading: boolean
  error: string | null
}

export interface BrowserControls {
  navigate: (url: string) => Promise<void>
  goBack: () => Promise<void>
  goForward: () => Promise<void>
  refresh: () => Promise<void>
  stop: () => void
}

export interface BrowserContentExtraction {
  extractText: () => Promise<string>
  extractHTML: () => Promise<string>
  captureScreenshot: (fullPage?: boolean) => Promise<string>
  extractMetadata: () => Promise<{
    title: string
    description?: string
    keywords?: string[]
    author?: string
  }>
}

export interface BrowserAction {
  type: 'navigate' | 'click' | 'input' | 'scroll' | 'extract' | 'screenshot'
  params: {
    url?: string
    selector?: string
    text?: string
    x?: number
    y?: number
    fullPage?: boolean
  }
}

export interface BrowserActionResult {
  success: boolean
  data?: unknown
  error?: string
}

export interface BrowserSecuritySettings {
  allowedDomains?: string[]
  blockedDomains?: string[]
  requireUserApproval: boolean
  enableJavaScript: boolean
  enablePlugins: boolean
  enableWebSecurity: boolean
}

export interface BrowserToolDefinition {
  name: string
  description: string
  parameters: {
    type: string
    properties: Record<string, unknown>
    required?: string[]
  }
}
