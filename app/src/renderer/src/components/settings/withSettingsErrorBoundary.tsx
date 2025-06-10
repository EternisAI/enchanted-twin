import { ComponentType } from 'react'
import { ErrorBoundary } from '@renderer/components/ui/error-boundary'

export function withSettingsErrorBoundary<P extends object>(
  Component: ComponentType<P>,
  componentName?: string
) {
  const WrappedComponent = (props: P) => {
    return (
      <ErrorBoundary
        onError={(error, errorInfo) => {
          console.error(
            `Error in ${componentName || Component.displayName || 'Settings Component'}:`,
            error,
            errorInfo
          )
        }}
      >
        <Component {...props} />
      </ErrorBoundary>
    )
  }

  WrappedComponent.displayName = `withSettingsErrorBoundary(${componentName || Component.displayName || 'Component'})`

  return WrappedComponent
}
