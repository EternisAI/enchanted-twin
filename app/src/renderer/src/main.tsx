/* eslint-disable react-refresh/only-export-components */
import './assets/main.css'

import { StrictMode } from 'react'
import { createRoot } from 'react-dom/client'

import { routeTree } from '@renderer/routeTree.gen'
import { createRouter, RouterProvider } from '@tanstack/react-router'
import { ApolloClientProvider } from './graphql/provider'

const router = createRouter({ routeTree, defaultViewTransition: true })

// Register the router instance for type safety
declare module '@tanstack/react-router' {
  interface Register {
    router: typeof router
  }
}

export default createRoot(document.getElementById('root')!).render(
  <StrictMode>
    <ApolloClientProvider>
      <RouterProvider router={router} />
    </ApolloClientProvider>
  </StrictMode>
)
