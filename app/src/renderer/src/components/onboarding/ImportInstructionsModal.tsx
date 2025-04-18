import { Dialog, DialogContent, DialogHeader, DialogTitle } from '../ui/dialog'

interface ImportInstructionsModalProps {
  isOpen: boolean
  onClose: () => void
  dataSource: string
}

const INSTRUCTIONS = {
  X: [
    "To export your data go to 'Settings and Privacy'",
    "Then go to 'Your account' → 'Download and archive your data'",
    "After your export is complete, move the entire folder to the input folder, and rename it to 'x_export'"
  ],
  Slack: [
    'Only the owner or administrator of the Slack workspace can usually export the data.',
    "After exporting your data, move the entire folder to the input folder, and rename it to 'slack_export'"
  ],
  Telegram: [
    "Install the Telegram Desktop app if you haven't already.",
    'Open Settings → Advanced.',
    'Scroll down to Export Telegram data.',
    'Select the data you want to export and select Chats and Contacts.',
    'At the bottom, choose JSON as the export format (machine-readable).',
    'Click Export.',
    'Note: Telegram data is contained in a single .json file.',
    "Move this file to the input folder and rename it to 'telegram_export.json'."
  ],
  Gmail: [
    'Note: Only Gmail data is currently supported.',
    'Use Google Takeout to export your Gmail data.',
    'Find the emails.mbox file in your export (it should be in Mail) and move it to input/google_export.'
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
  const instructions = INSTRUCTIONS[dataSource as keyof typeof INSTRUCTIONS] || []

  return (
    <Dialog open={isOpen} onOpenChange={onClose}>
      <DialogContent className="sm:max-w-[600px]">
        <DialogHeader>
          <DialogTitle>How to export {dataSource} data</DialogTitle>
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
