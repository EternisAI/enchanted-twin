import { ComponentType } from 'react'
import { ErrorBoundary } from '@renderer/components/ui/error-boundary'

export function withSettingsErrorBoundary<P extends object>(
  Component: ComponentType<P>,
  componentName?: string
) {
  const WrappedComponent = (props: P) => {
    return (
      <ErrorBoundary componentName={componentName || Component.displayName || 'Settings Component'}>
        <Component {...props} />
      </ErrorBoundary>
    )
  }

  WrappedComponent.displayName = `withSettingsErrorBoundary(${componentName || Component.displayName || 'Component'})`

  return WrappedComponent
}
