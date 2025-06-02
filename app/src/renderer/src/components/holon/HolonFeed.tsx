import { Button } from '../ui/button'
import HolonFeedItem from './HolonFeedListItem'

// Mock data for development
const loading = false
const error = null as unknown
const data = {
  getThreads: [
    {
      id: '1',
      title: 'Some Random Post',
      content: `Hey Bay-Area poker twins!

My twin and I are putting together a friendly $1/$2 No-Limit Hold'em cash game and we'd love a few more players. Here's the plan:

📍 Where?`,
      imageURLs: [
        'https://images.unsplash.com/photo-1541278107931-e006523892df?w=400&h=300&fit=crop'
      ],
      author: {
        alias: 'You',
        identity: 'user123'
      },
      createdAt: '2024-01-15T10:37:00Z',
      expiresAt: null,
      views: 100,
      messages: [
        {
          id: 'm1',
          content: 'Great idea!',
          author: { alias: 'Player1', identity: 'p1' },
          createdAt: '2024-01-15T11:00:00Z',
          isDelivered: true,
          actions: []
        },
        {
          id: 'm2',
          content: 'Count me in',
          author: { alias: 'Player2', identity: 'p2' },
          createdAt: '2024-01-15T11:30:00Z',
          isDelivered: true,
          actions: []
        }
      ],
      actions: ['Join Game', 'Share']
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
}

export default function HolonFeed() {
  // TODO: Uncomment when backend is ready
  // const { data, loading, error } = useQuery(GetThreadsDocument, {
  //   variables: { network: null }
  // })

  if (loading) {
    return (
      <div className="flex flex-col items-center justify-center min-h-screen">
        <div className="text-muted-foreground">Loading threads...</div>
      </div>
    )
  }

  if (error) {
    return (
      <div className="flex flex-col items-center justify-center min-h-screen">
        <div className="text-destructive">Error loading threads: {error.message}</div>
      </div>
    )
  }

  const threads = data?.getThreads || []

  return (
    <div className="flex w-full justify-center overflow-y-auto">
      <div className="max-w-2xl mx-auto p-6 flex flex-col gap-16">
        <div className="flex items-center justify-between">
          <h1 className="text-4xl font-bold text-foreground">Discover & Connect</h1>
          <Button size="sm" className="text-md font-semibold">
            Create
          </Button>
        </div>

        <div className="flex flex-col gap-6">
          {threads.length === 0 ? (
            <div className="text-center py-12 text-muted-foreground">
              No threads available yet. Be the first to create one!
            </div>
          ) : (
            threads.map((thread) => <HolonFeedItem key={thread.id} thread={thread} />)
          )}
        </div>
      </div>
    </div>
  )
}
