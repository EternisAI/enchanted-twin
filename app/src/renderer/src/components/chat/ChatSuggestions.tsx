import { useEffect, useState } from 'react'
import { useQuery } from '@apollo/client'
import { GetChatSuggestionsDocument } from '@renderer/graphql/generated/graphql'
import { Tabs, TabsContent, TabsList, TabsTrigger } from '@renderer/components/ui/tabs'
import { Button } from '../ui/button'
import { motion } from 'framer-motion'

const MOCK_SUGGESTIONS = {
  getChatSuggestions: [
    {
      category: 'Questions',
      suggestions: [
        'Can you explain this in more detail?',
        'What are the main benefits?',
        'How does this compare to alternatives?'
      ]
    },
    {
      category: 'Actions',
      suggestions: ['Show me an example', 'Generate some code for this', 'Summarize the key points']
    },
    {
      category: 'Follow-ups',
      suggestions: [
        'Tell me more about this topic',
        'What should I know next?',
        'What are common problems with this approach?'
      ]
    }
  ]
}

export default function ChatSuggestions({
  chatId,
  visible,
  onSuggestionClick
}: {
  chatId: string
  visible: boolean
  onSuggestionClick: (suggestion: string) => void
}) {
  const [hideSuggestions, setHideSuggestions] = useState(false)
  const { data, loading, refetch } = useQuery(GetChatSuggestionsDocument, {
    variables: { chatId },
    skip: !visible || !chatId,
    fetchPolicy: 'network-only'
  })

  useEffect(() => {
    if (visible && chatId) {
      refetch()
    }
  }, [visible, chatId, refetch])

  const suggestions = data?.getChatSuggestions || MOCK_SUGGESTIONS.getChatSuggestions

  console.log('suggestions data', data, suggestions)

  if (!visible || loading || !suggestions) {
    return null
  }

  if (hideSuggestions) {
    return (
      <div className="w-full flex justify-end pb-2">
        <Button variant="ghost" onClick={() => setHideSuggestions(false)}>
          Show suggestions
        </Button>
      </div>
    )
  }

  return (
    <div className="relative w-full pb-4">
      <Tabs defaultValue={suggestions[0].category}>
        <TabsList className="mb-2 w-full h-auto p-1 justify-start overflow-x-auto">
          {suggestions.map((category) => (
            <TabsTrigger
              key={category.category}
              value={category.category}
              className="px-3 py-1.5 text-sm cursor-pointer"
            >
              {category.category}
            </TabsTrigger>
          ))}
        </TabsList>
        {suggestions.map((category) => (
          <TabsContent key={category.category} value={category.category}>
            <motion.div
              className="flex flex-wrap gap-2 h-20"
              initial={{ opacity: 0, y: -10 }}
              animate={{ opacity: 1, y: 0 }}
              transition={{ duration: 0.25 }}
            >
              {category.suggestions.slice(0, 5).map((suggestion, index) => (
                <Button
                  variant="ghost"
                  key={`${suggestion}-${index}`}
                  onClick={() => onSuggestionClick(suggestion)}
                  className="px-3 py-1.5 text-sm bg-gray-100 hover:bg-gray-200 rounded-full transition-colors"
                >
                  {suggestion}
                </Button>
              ))}
            </motion.div>
          </TabsContent>
        ))}
        <Button
          variant="ghost"
          onClick={() => setHideSuggestions(true)}
          className="absolute bottom-2 right-0 px-3 py-1.5 text-sm bg-gray-100 hover:bg-gray-200  transition-colors"
        >
          Hide suggestions
        </Button>
      </Tabs>
    </div>
  )
}
