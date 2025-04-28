import { useQuery } from '@apollo/client'
import { GetChatSuggestionsDocument } from '@renderer/graphql/generated/graphql'
import { Tabs, TabsContent, TabsList, TabsTrigger } from '@renderer/components/ui/tabs'
import { Button } from '../ui/button'
import { motion } from 'framer-motion'
import { MessageCircleMore, MessageCircleOff } from 'lucide-react'
import { Tooltip, TooltipContent, TooltipProvider, TooltipTrigger } from '../ui/tooltip'

export default function ChatSuggestions({
  chatId,
  visible,
  toggleVisibility,
  onSuggestionClick
}: {
  chatId: string
  visible: boolean
  onSuggestionClick: (suggestion: string) => void
  toggleVisibility: () => void
}) {
  const { data, error, loading } = useQuery(GetChatSuggestionsDocument, {
    variables: { chatId },
    skip: !chatId,
    fetchPolicy: 'network-only'
  })

  const suggestions = data?.getChatSuggestions

  if (!visible) {
    return (
      <motion.div
        initial={{ opacity: 0 }}
        animate={{ opacity: 1 }}
        transition={{ duration: 0.25 }}
        className="absolute right-0 bottom-2"
      >
        <TooltipProvider>
          <Tooltip>
            <TooltipTrigger asChild>
              <Button
                variant="ghost"
                onClick={toggleVisibility}
                size="icon"
                className="rounded-full"
              >
                <MessageCircleMore className="h-5 w-5 opacity-70 hover:opacity-100 transition-opacity" />
              </Button>
            </TooltipTrigger>
            <TooltipContent>
              <p>Show suggestions</p>
            </TooltipContent>
          </Tooltip>
        </TooltipProvider>
      </motion.div>
    )
  }

  if (loading || error || !suggestions || suggestions.length === 0) {
    return null
  }

  return (
    <motion.div
      className="relative w-full pb-4"
      initial={{ opacity: 0, y: -10 }}
      animate={{ opacity: 1, y: 0 }}
      transition={{ duration: 0.25 }}
    >
      <Tabs defaultValue={suggestions[0].category}>
        <TabsList className="mb-2 w-full h-auto p-1 justify-start overflow-x-auto">
          {suggestions.map((category) => (
            <TabsTrigger
              key={category.category}
              value={category.category}
              className="px-3.5 py-2.5 text-sm cursor-pointer capitalize"
            >
              {category.category}
            </TabsTrigger>
          ))}
        </TabsList>
        {suggestions.map((category) => (
          <TabsContent key={category.category} value={category.category}>
            <motion.div
              className="flex flex-col items-start flex-wrap gap-2 min-h-10"
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
        <motion.div
          initial={{ opacity: 0 }}
          animate={{ opacity: 1 }}
          transition={{ duration: 0.25 }}
          className="absolute bottom-2 right-0"
        >
          <TooltipProvider>
            <Tooltip>
              <TooltipTrigger asChild>
                <Button
                  variant="ghost"
                  onClick={toggleVisibility}
                  size="icon"
                  className="rounded-full"
                >
                  <MessageCircleOff className="h-5 w-5 opacity-70 hover:opacity-100 transition-opacity" />
                </Button>
              </TooltipTrigger>
              <TooltipContent>
                <p>Hide suggestions</p>
              </TooltipContent>
            </Tooltip>
          </TooltipProvider>
        </motion.div>
      </Tabs>
    </motion.div>
  )
}
