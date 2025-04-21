/* eslint-disable react-refresh/only-export-components */
import './assets/main.css'

import { StrictMode } from 'react'
import { createRoot } from 'react-dom/client'

import { routeTree } from '@renderer/routeTree.gen'
import { createRouter, RouterProvider } from '@tanstack/react-router'
import { ApolloClientProvider } from './graphql/provider'
import { ThemeProvider } from './lib/theme'

import { createHashHistory } from '@tanstack/react-router'

const router = createRouter({
  routeTree,
  defaultViewTransition: true,
  history: createHashHistory(),
})

// Register the router instance for type safety
declare module '@tanstack/react-router' {
  interface Register {
    router: typeof router
  }
}

// Get the saved theme from localStorage or default to system
const savedTheme = (() => {
  try {
    return (localStorage.getItem('theme') as 'dark' | 'light' | 'system') || 'system'
  } catch {
    return 'system'
  }
})()

export default createRoot(document.getElementById('root')!).render(
  <StrictMode>
    <ThemeProvider defaultTheme={savedTheme}>
      <ApolloClientProvider>
        <RouterProvider router={router} />
      </ApolloClientProvider>
    </ThemeProvider>
  </StrictMode>
)
