import { ApolloClient, InMemoryCache, HttpLink, split } from '@apollo/client'
import { GraphQLWsLink } from '@apollo/client/link/subscriptions'
import { getMainDefinition } from '@apollo/client/utilities'
import { createClient } from 'graphql-ws'

const httpLink = new HttpLink({
  uri: import.meta.env.RENDERER_VITE_API_URL
})

const wsLink = new GraphQLWsLink(
  createClient({
    url: import.meta.env.RENDERER_VITE_API_URL.replace('http', 'ws')
    // Optional: add connectionParams for authentication if needed
    // connectionParams: {
    //   authToken: localStorage.getItem('auth-token')
    // }
  })
)

const splitLink = split(
  ({ query }) => {
    const definition = getMainDefinition(query)
    return definition.kind === 'OperationDefinition' && definition.operation === 'subscription'
  },
  wsLink,
  httpLink
)

export const client = new ApolloClient({
  link: splitLink,
  cache: new InMemoryCache()
})
