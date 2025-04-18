import { Message, Role, Chat } from '@renderer/graphql/generated/graphql'

const messages: Message[] = [
  {
    id: '1',
    text: 'Hey! Can you show me a picture of a cat?',
    imageUrls: [],
    role: Role.User,
    toolCalls: [],
    toolResults: [],
    createdAt: new Date().toISOString()
  },
  {
    id: '2',
    text: "Sure! Here's a cat for you 🐱",
    imageUrls: [],
    role: Role.Assistant,
    toolCalls: [],
    toolResults: [],
    createdAt: new Date().toISOString()
  },
  {
    id: '3',
    text: "Can you also fetch me today's weather in NYC?",
    imageUrls: [],
    role: Role.User,
    toolCalls: [],
    toolResults: [],
    createdAt: new Date().toISOString()
  },
  {
    id: '4',
    text: 'Fetching weather...',
    imageUrls: [],
    role: Role.Assistant,
    toolCalls: [{ id: '1', name: 'getWeather', isCompleted: false }],
    toolResults: ['{"temp":"14°C","condition":"Cloudy"}'],
    createdAt: new Date().toISOString()
  },
  {
    id: '5',
    text: 'Here is a cute cat picture for you! 🐱',
    imageUrls: [],
    role: Role.Assistant,
    toolCalls: [],
    toolResults: [],
    createdAt: new Date().toISOString()
  },
  {
    id: '6',
    text: 'Cats are great companions! 🐾',
    imageUrls: [],
    role: Role.Assistant,
    toolCalls: [],
    toolResults: [],
    createdAt: new Date().toISOString()
  },
  {
    id: '7',
    text: 'Did you know cats can jump up to six times their body length? 🐈',
    imageUrls: [],
    role: Role.Assistant,
    toolCalls: [],
    toolResults: [],
    createdAt: new Date().toISOString()
  },
  {
    id: '8',
    text: "Here's another fun fact: cats sleep for about 70% of their lives! 😺",
    imageUrls: [],
    role: Role.Assistant,
    toolCalls: [],
    toolResults: [],
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
    messages: messages.slice(0, 3),
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
