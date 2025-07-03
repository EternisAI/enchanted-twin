import { ApolloClient, InMemoryCache, HttpLink, split } from '@apollo/client'
import { setContext } from '@apollo/client/link/context'
import { GraphQLWsLink } from '@apollo/client/link/subscriptions'
import { getMainDefinition } from '@apollo/client/utilities'
import { createClient } from 'graphql-ws'
import { auth } from '../lib/firebase'

const httpLink = new HttpLink({
  uri: 'http://localhost:44999/query'
})

const authLink = setContext(async (_, { headers }) => {
  try {
    await auth.authStateReady()
    const user = auth.currentUser
    const token = user ? await user.getIdToken() : null
    return {
      headers: {
        ...headers,
        authorization: token ? `Bearer ${token}` : ''
      }
    }
  } catch (error) {
    console.error('Error getting Firebase token:', error)
    return { headers }
  }
})

const wsLink = new GraphQLWsLink(
  createClient({
    url: 'http://localhost:44999/query'.replace('http', 'ws'),
    connectionParams: async () => {
      try {
        const token = await auth.currentUser?.getIdToken()
        return {
          authorization: token ? `Bearer ${token}` : ''
        }
      } catch (error) {
        console.error('Error getting Firebase token for WebSocket:', error)
        return {}
      }
    }
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
  link: authLink.concat(splitLink),
  cache: new InMemoryCache()
})
