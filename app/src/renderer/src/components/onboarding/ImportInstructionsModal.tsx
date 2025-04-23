import { Dialog, DialogContent, DialogHeader, DialogTitle } from '../ui/dialog'
import { ExternalLink } from 'lucide-react'
import { useEffect, useState } from 'react'

interface ImportInstructionsModalProps {
  isOpen: boolean
  onClose: () => void
  dataSource: string
}

const INSTRUCTIONS = {
  X: [
    <span key="x-link">
      To export your data go to{' '}
      <a
        href="https://x.com/settings/download_your_data"
        target="_blank"
        rel="noopener noreferrer"
        className="underline hover:text-primary transition-colors inline-flex items-center gap-1"
      >
        Download your data <ExternalLink className="w-3 h-3" />
      </a>
    </span>,
    "After your export is complete, move the entire folder to the input folder, and rename it to 'x_export'"
  ],
  Slack: [
    'Only the owner or administrator of the Slack workspace can usually export the data.',
    'Select your exported ZIP file'
  ],
  Telegram: ['TBA'],
  Gmail: [
    'Note: Only Gmail data is currently supported.',
    'Use Google Takeout to export your Gmail data.'
  ],
  GoogleAddresses: [
    "You should find the address file in the 'Maps' folder or equivalent of your google export.",
    'Similarly, move the file to input/google_export folder and rename it to addresses.json.'
  ],
  WhatsApp: [
    'Select your WhatsApp SQLITE database file (~/Library/Group Containers/group.net.whatsapp.WhatsApp.shared/ChatStorage.sqlite)'
  ]
}

export function ImportInstructionsModal({
  isOpen,
  onClose,
  dataSource
}: ImportInstructionsModalProps) {
  const [visibleDataSource, setVisibleDataSource] = useState<string>('')
  const instructions = INSTRUCTIONS[visibleDataSource as keyof typeof INSTRUCTIONS] || []

  useEffect(() => {
    if (isOpen) {
      setVisibleDataSource(dataSource)
    }
  }, [isOpen, dataSource])

  return (
    <Dialog open={isOpen} onOpenChange={onClose}>
      <DialogContent className="sm:max-w-[600px]">
        <DialogHeader>
          <DialogTitle>How to export {visibleDataSource} data</DialogTitle>
        </DialogHeader>
        <div className="space-y-4 py-4">
          <ol className="list-decimal list-inside space-y-2">
            {instructions.map((instruction, index) => (
              <li key={index} className="text-sm text-muted-foreground">
                {instruction}
              </li>
            ))}
          </ol>
        </div>
      </DialogContent>
    </Dialog>
  )
}
