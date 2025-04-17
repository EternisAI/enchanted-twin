import { AppNav } from '@renderer/components/AppNav'
import { createRootRoute, Outlet } from '@tanstack/react-router'

// This is the root layout, components get render in the <Outlet />
export const Route = createRootRoute({
  component: () => {
    return (
      <div className="flex flex-col h-screen w-screen text-foreground">
        <div className="titlebar text-center fixed top-0 left-0 right-0 text-muted-foreground text-xs h-6"></div>
        <div className="flex flex-1 pt-6 ">
          <AppNav />
          <div className="flex-1 overflow-auto rounded-lg h-full">
            <Outlet />
            {/* <TanStackRouterDevtools /> */}
          </div>
        </div>
      </div>
    )
  }
})
