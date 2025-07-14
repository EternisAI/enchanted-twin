import React, { Component, ReactNode } from 'react'

interface Props {
  children: ReactNode
  fallback?: ReactNode
  componentName?: string
}

interface State {
  hasError: boolean
  error?: Error
}

export class ErrorBoundary extends Component<Props, State> {
  constructor(props: Props) {
    super(props)
    this.state = { hasError: false }
  }

  static getDerivedStateFromError(error: Error): State {
    return { hasError: true, error }
  }

  componentDidCatch(error: Error, errorInfo: React.ErrorInfo) {
    console.error('ErrorBoundary caught an error:', error, errorInfo)

    if (window.api?.analytics?.capture) {
      window.api.analytics.capture('error_occurred', {
        error_type: 'component_render_error',
        component: this.props.componentName || 'Unknown',
        message: error.message,
        stack: error.stack,
        error_boundary: true
      })
    }
  }

  render() {
    if (this.state.hasError) {
      if (this.props.fallback) {
        return this.props.fallback
      }

      return (
        <div className="w-full border-none flex flex-col gap-2 items-center text-center">
          <h2 className="text-2xl font-semibold">Something went wrong</h2>
          <p className="text-sm text-muted-foreground">Unable to load this component</p>
        </div>
      )
    }

    return this.props.children
  }
}
