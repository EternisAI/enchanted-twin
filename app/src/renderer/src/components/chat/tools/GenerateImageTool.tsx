import { ToolCall } from '@renderer/graphql/generated/graphql'
import ImagePreview from '../messages/ImagePreview'
import { Badge } from '@renderer/components/ui/badge'
import { CheckCircle } from 'lucide-react'

export default function GenerateImageTool({ toolCall }: { toolCall: ToolCall }) {
  const imageUrls = toolCall.result?.imageUrls || []

  return (
    <div className="flex flex-col gap-2">
      <Badge className="text-green-600 border-green-500" variant="outline">
        <CheckCircle className="h-4 w-4" />
        <span>Image Generated</span>
      </Badge>
      {imageUrls.length > 0 && (
        <div className="grid grid-cols-4 gap-y-4 my-2">
          {imageUrls.map((url, i) => (
            <ImagePreview
              key={i}
              src={url}
              alt={`attachment-${i}`}
              thumbClassName="inline-block h-40 w-40 object-cover rounded"
            />
          ))}
        </div>
      )}
    </div>
  )
}
