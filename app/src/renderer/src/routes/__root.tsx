import Header from '@renderer/components/Header'
import { createRootRoute, Outlet } from '@tanstack/react-router'
import { TanStackRouterDevtools } from '@tanstack/react-router-devtools'

// This is the root layout, components get render in the <Outlet />
export const Route = createRootRoute({
  component: () => {
    return (
      <div className="flex flex-col h-screen w-screen">
        <Header />
        <div className="flex-1 overflow-auto">
          <Outlet />
          <TanStackRouterDevtools />
        </div>
      </div>
    )
  }
})
