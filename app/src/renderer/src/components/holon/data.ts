// Mock data for Holon components
export const mockThreads = [
  {
    id: '1',
    title: 'Hey Bay-Area poker twins!',
    content: `My twin and I are putting together a friendly $1/$2 No-Limit Hold'em cash game and we'd love a few more players. Here's the plan:

Where?
San Francisco, Mission District - exact location shared with confirmed players

When?
This Saturday, January 20th, starting at 7:00 PM

Stakes?
$1/$2 No-Limit Hold'em cash game
$200-$500 buy-in range (your choice)

What to expect?
- Friendly, social atmosphere
- No pressure, just good poker and good vibes
- BYOB welcome, snacks provided
- Games typically run 4-6 hours

Who's interested? Drop a comment below or DM me directly. Looking to get 6-8 players total, so first come, first served!

#SFPoker #BayAreaPoker #PokerTwins #CashGame`,
    imageURLs: [
      'https://images.unsplash.com/photo-1541278107931-e006523892df?w=600&h=400&fit=crop',
      'https://images.unsplash.com/photo-1606092195730-5d7b9af1efc5?w=600&h=400&fit=crop'
    ],
    author: {
      alias: 'You',
      identity: 'user123'
    },
    createdAt: '2024-01-15T10:37:00Z',
    expiresAt: '2024-01-21T19:00:00Z',
    views: 100,
    messages: [
      {
        id: 'm1',
        content: 'Count me in! What time exactly?',
        author: { alias: 'PokerPro', identity: 'pp1' },
        createdAt: '2024-01-15T11:00:00Z',
        isDelivered: true,
        actions: []
      },
      {
        id: 'm2',
        content: 'Sounds great! I can bring chips if needed.',
        author: { alias: 'CardShark', identity: 'cs1' },
        createdAt: '2024-01-15T11:30:00Z',
        isDelivered: true,
        actions: []
      },
      {
        id: 'm3',
        content: "Is this beginner friendly? I'm still learning.",
        author: { alias: 'Newbie', identity: 'nb1' },
        createdAt: '2024-01-15T12:00:00Z',
        isDelivered: true,
        actions: []
      }
    ],
    actions: ['Join Game', 'Share', 'RSVP by Tomorrow']
  },
  {
    id: '2',
    title: 'Random Meme',
    content: '',
    imageURLs: [
      'https://images.unsplash.com/photo-1606092195730-5d7b9af1efc5?w=400&h=300&fit=crop'
    ],
    author: {
      alias: 'Anonymous User',
      identity: 'anon456'
    },
    createdAt: '2024-01-15T10:37:00Z',
    expiresAt: null,
    views: 100,
    messages: [],
    actions: ['React', 'Share']
  },
  {
    id: '3',
    title: 'SF Poker Night',
    content: `Hey Bay-Area poker twins!

My twin and I are putting together a friendly $1/$2 No-Limit Hold'em cash game and we'd love a few more players. Here's the plan:

📍 Where?`,
    imageURLs: [
      'https://images.unsplash.com/photo-1606092195730-5d7b9af1efc5?w=400&h=300&fit=crop'
    ],
    author: {
      alias: 'Anonymous User',
      identity: 'anon789'
    },
    createdAt: '2024-01-15T10:37:00Z',
    expiresAt: '2024-01-21T19:00:00Z',
    views: 100,
    messages: new Array(12).fill(null).map((_, i) => ({
      id: `m${i}`,
      content: `Message ${i + 1}`,
      author: { alias: `User${i}`, identity: `user${i}` },
      createdAt: '2024-01-15T11:00:00Z',
      isDelivered: true,
      actions: []
    })),
    actions: ['RSVP by Tomorrow']
  },
  {
    id: '4',
    title: 'San Francisco SF Japantown Apartment',
    content: `San Francisco SF Japantown Apartment Available for Short-Term Rental from Today to June 10th

SF Japantown 94115, Apartment, 9th floor, 3B1B, 1 bedroom, short-term rental from now until June 10.

Rent 40/day, including water and electricity, bedroom with full furniture, ready to move in. Discounts available for renting for a week or more. This price is really super affordable in downtown San Francisco!`,
    imageURLs: [],
    author: {
      alias: 'Anonymous User',
      identity: 'anon999'
    },
    createdAt: '2024-01-15T10:37:00Z',
    expiresAt: null,
    views: 100,
    messages: new Array(12).fill(null).map((_, i) => ({
      id: `m${i}`,
      content: `Message ${i + 1}`,
      author: { alias: `User${i}`, identity: `user${i}` },
      createdAt: '2024-01-15T11:00:00Z',
      isDelivered: true,
      actions: []
    })),
    actions: ['Contact', 'Save']
  },
  {
    id: '5',
    title: 'Random Meme',
    content: '',
    imageURLs: [
      'https://images.unsplash.com/photo-1606092195730-5d7b9af1efc5?w=400&h=300&fit=crop',
      'https://images.unsplash.com/photo-1606092195730-5d7b9af1efc5?w=400&h=300&fit=crop'
    ],
    author: {
      alias: 'Anonymous User',
      identity: 'anon111'
    },
    createdAt: '2024-01-15T10:37:00Z',
    expiresAt: null,
    views: 100,
    messages: [],
    actions: []
  }
]

// Helper function to get a thread by ID
export const getThreadById = (threadId: string) => {
  return mockThreads.find((thread) => thread.id === threadId)
}

// Mock data structure for API responses
export const mockHolonData = {
  getThreads: mockThreads,
  getThread: (threadId: string) => getThreadById(threadId)
}
