import { Message, Role, Chat } from '@renderer/graphql/generated/graphql'

const messages: Message[] = [
  {
    id: '1',
    text: 'Hey! Can you show me a picture of a cat?',
    imageUrls: [],
    role: Role.User,
    toolCalls: [],
    toolResult: null,
    createdAt: new Date().toISOString()
  },
  {
    id: '2',
    text: 'Sure! Here’s a cat for you 🐱',
    imageUrls: [],
    role: Role.Assistant,
    toolCalls: [],
    toolResult: null,
    createdAt: new Date().toISOString()
  },
  {
    id: '3',
    text: 'Can you also fetch me today’s weather in NYC?',
    imageUrls: [],
    role: Role.User,
    toolCalls: [],
    toolResult: null,
    createdAt: new Date().toISOString()
  },
  {
    id: '4',
    text: 'Fetching weather...',
    imageUrls: [],
    role: Role.Assistant,
    toolCalls: [{ id: '1', name: 'getWeather', isCompleted: false }],
    toolResult: [{ temp: '14°C', condition: 'Cloudy' }],
    createdAt: new Date().toISOString()
  }
]

const chats: Chat[] = [
  {
    id: '1',
    name: 'Chat with my friend',
    messages: messages,
    createdAt: new Date().toISOString()
  },
  {
    id: '2',
    name: 'Chat with my family',
    messages: messages.slice(0, 2),
    createdAt: new Date().toISOString()
  },
  {
    id: '3',
    name: 'Chat 3',
    messages: messages.slice(0, 2),
    createdAt: new Date().toISOString()
  },
  {
    id: '4',
    name: 'Chat 4',
    messages: [],
    createdAt: new Date().toISOString()
  }
]

export const mockChats = chats
