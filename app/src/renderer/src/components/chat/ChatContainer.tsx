/* eslint-disable @typescript-eslint/no-explicit-any */

import { MessageBubble } from './Message'

const messages = [
  {
    id: '1',
    text: 'Hey! Can you show me a picture of a cat?',
    imageUrls: [],
    role: 'user',
    toolCalls: [],
    toolResult: null,
    createdAt: new Date().toISOString()
  },
  {
    id: '2',
    text: 'Sure! Here’s a cat for you 🐱',
    imageUrls: [],
    role: 'assistant',
    toolCalls: [],
    toolResult: null,
    createdAt: new Date().toISOString()
  },
  {
    id: '3',
    text: 'Can you also fetch me today’s weather in NYC?',
    imageUrls: [],
    role: 'user',
    toolCalls: [],
    toolResult: null,
    createdAt: new Date().toISOString()
  },
  {
    id: '4',
    text: 'Fetching weather...',
    imageUrls: [],
    role: 'assistant',
    toolCalls: [{ name: 'getWeather', args: { city: 'New York' } }],
    toolResult: { temp: '14°C', condition: 'Cloudy' },
    createdAt: new Date().toISOString()
  }
]

export default function ChatContainer() {
  return (
    <div className="pb-10">
      <style>
        {`
          :root {
            --direction: -1;
          }
        `}
      </style>
      <div className="py-6 flex flex-col gap-4 items-center ">
        <div className="flex max-w-2xl w-full flex-col gap-4">
          <h1 className="text-xl font-bold">Mock Chat</h1>
          <div className="flex flex-col gap-4">
            {messages.map((msg) => (
              <MessageBubble key={msg.id} message={msg as any} />
            ))}
          </div>
        </div>
      </div>
    </div>
  )
}
