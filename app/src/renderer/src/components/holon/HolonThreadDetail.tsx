// import { useQuery } from '@apollo/client'
// import { GetThreadDocument } from '@renderer/graphql/generated/graphql'
import { Button } from '../ui/button'
import { formatDistanceToNow } from 'date-fns'
import { Eye, MoreHorizontal, ArrowLeft } from 'lucide-react'
import { useNavigate } from '@tanstack/react-router'

interface HolonThreadDetailProps {
  threadId: string
}

export default function HolonThreadDetail({ threadId }: HolonThreadDetailProps) {
  const navigate = useNavigate()

  // TODO: Uncomment when backend is ready
  // const { data, loading, error } = useQuery(GetThreadDocument, {
  //   variables: { id: threadId, network: null }
  // })

  // Mock data for development
  const loading = false
  const error = null
  const data = {
    getThread: {
      id: threadId,
      title: 'Hey Bay-Area poker twins!',
      content:
        "My twin and I are putting together a friendly $1/$2 No-Limit Hold'em cash game and we'd love a few more players. Here's the plan:\n\nWhere?\nSan Francisco, Mission District - exact location shared with confirmed players\n\nWhen?\nThis Saturday, January 20th, starting at 7:00 PM\n\nStakes?\n$1/$2 No-Limit Hold'em cash game\n$200-$500 buy-in range (your choice)\n\nWhat to expect?\n- Friendly, social atmosphere\n- No pressure, just good poker and good vibes\n- BYOB welcome, snacks provided\n- Games typically run 4-6 hours\n\nWho's interested? Drop a comment below or DM me directly. Looking to get 6-8 players total, so first come, first served!\n\n#SFPoker #BayAreaPoker #PokerTwins #CashGame",
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
    }
  }

  const handleBack = () => {
    navigate({ to: '/holon' })
  }

  if (loading) {
    return (
      <div className="flex items-center justify-center min-h-screen">
        <div className="text-muted-foreground">Loading thread...</div>
      </div>
    )
  }

  if (error || !data?.getThread) {
    return (
      <div className="flex items-center justify-center min-h-screen">
        <div className="text-destructive">
          {error ? 'Error loading thread' : 'Thread not found'}
        </div>
      </div>
    )
  }

  const thread = data.getThread

  return (
    <div className="flex w-full overflow-y-auto justify-center mb-12">
      <div className="max-w-2xl flex bg-gray-100 p-2 flex-col gap-6 rounded-lg">
        <div className="rounded-lg bg-white">
          <div className="flex flex-col gap-4 px-6 pt-3">
            <div className="flex items-start justify-between border-b border-border pb-3">
              <div className="space-y-2 flex-1">
                <h2 className="text-lg font-semibold text-foreground">{thread.title}</h2>
                <div className="flex items-center gap-2 text-sm text-muted-foreground">
                  <span className="font-medium">
                    {thread.author.alias || thread.author.identity}
                  </span>
                  <span>•</span>
                  <span>
                    {formatDistanceToNow(new Date(thread.createdAt), { addSuffix: true })}
                  </span>
                </div>
              </div>
              <Button variant="ghost" size="icon" onClick={handleBack}>
                <ArrowLeft className="w-4 h-4" />
              </Button>
            </div>
          </div>

          <div>
            <div className="flex flex-col gap-4 p-6">
              <div className="flex flex-col gap-4">
                <p className="text-foreground whitespace-pre-wrap leading-relaxed text-base">
                  {thread.content}
                </p>

                {thread.imageURLs && thread.imageURLs.length > 0 && (
                  <div className="grid gap-4">
                    {thread.imageURLs.length === 1 ? (
                      <img
                        src={thread.imageURLs[0]}
                        alt="Thread image"
                        className="w-full rounded-lg max-h-96 object-cover"
                      />
                    ) : (
                      <div className="grid grid-cols-2 gap-4">
                        {thread.imageURLs.map((imageUrl, index) => (
                          <img
                            key={index}
                            src={imageUrl}
                            alt={`Thread image ${index + 1}`}
                            className="w-full h-48 rounded-lg object-cover"
                          />
                        ))}
                      </div>
                    )}
                  </div>
                )}
              </div>

              <div className="flex items-center gap-6 text-sm text-muted-foreground border-t border-border py-4">
                <div className="flex items-center gap-2">
                  <Eye className="w-4 h-4" />
                  <span>Read by {thread.views}</span>
                </div>
                <div>{thread.messages.length} messages</div>
                {thread.expiresAt && (
                  <div className="text-orange-500">
                    Expires {formatDistanceToNow(new Date(thread.expiresAt), { addSuffix: true })}
                  </div>
                )}
              </div>
            </div>
          </div>
        </div>

        {/* Messages Section */}
        {thread.messages.length > 0 && (
          <div className="space-y-4">
            <h3 className="font-medium text-foreground text-lg">
              Messages ({thread.messages.length})
            </h3>
            <div className="space-y-3">
              {thread.messages.map((message) => (
                <div
                  key={message.id}
                  className="border border-border rounded-lg p-4 bg-card hover:bg-accent/10 transition-colors"
                >
                  <div className="flex items-center gap-2 text-sm text-muted-foreground mb-3">
                    <span className="font-medium">
                      {message.author.alias || message.author.identity}
                    </span>
                    <span>•</span>
                    <span>
                      {formatDistanceToNow(new Date(message.createdAt), {
                        addSuffix: true
                      })}
                    </span>
                  </div>
                  <p className="text-foreground leading-relaxed">{message.content}</p>
                </div>
              ))}
            </div>
          </div>
        )}

        {thread.actions && thread.actions.length > 0 && (
          <div className="flex flex-col gap-3 pb-24">
            <h3 className="font-medium text-foreground">Actions</h3>
            <div className="flex flex-wrap gap-3">
              {thread.actions.map((action, index) => (
                <Button
                  key={index}
                  variant={index === 0 ? 'default' : 'secondary'}
                  className="min-w-fit"
                >
                  {action}
                </Button>
              ))}
            </div>
          </div>
        )}
      </div>
    </div>
  )
}
