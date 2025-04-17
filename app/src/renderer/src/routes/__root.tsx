import { AppNav } from '@renderer/components/AppNav'
import { createRootRoute, Outlet } from '@tanstack/react-router'

// This is the root layout, components get render in the <Outlet />
export const Route = createRootRoute({
  component: () => {
    return (
      <div className="flex h-screen w-screen">
        <AppNav />
        <div className="flex-1 overflow-auto">
          <Outlet />
          {/* <TanStackRouterDevtools /> */}
        </div>
      </div>
    )
  }
})
