import { ApolloClient, InMemoryCache, HttpLink, split } from '@apollo/client'
import { GraphQLWsLink } from '@apollo/client/link/subscriptions'
import { getMainDefinition } from '@apollo/client/utilities'
import { createClient } from 'graphql-ws'

const httpLink = new HttpLink({
  uri: 'http://localhost:3000/query'
})

const wsLink = new GraphQLWsLink(
  createClient({
    url: 'http://localhost:3000/query'.replace('http', 'ws')
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
