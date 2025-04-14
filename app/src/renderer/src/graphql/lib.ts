import { ApolloClient, InMemoryCache } from '@apollo/client'

export const client = new ApolloClient({
  uri: import.meta.env.RENDERER_VITE_API_URL,
  cache: new InMemoryCache()
})
