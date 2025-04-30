import { useQuery } from '@apollo/client'
import { GetDataSourcesDocument } from '@renderer/graphql/generated/graphql'
import { Button } from '../ui/button'
import { CheckCircle2, Loader2, X, Clock, File, ExternalLink } from 'lucide-react'
import { useState } from 'react'
import { toast } from 'sonner'
import WhatsAppIcon from '@renderer/assets/icons/whatsapp'
import TelegramIcon from '@renderer/assets/icons/telegram'
import SlackIcon from '@renderer/assets/icons/slack'
import GmailIcon from '@renderer/assets/icons/gmail'
import XformerlyTwitterIcon from '@renderer/assets/icons/x'
import {
  Dialog,
  DialogContent,
  DialogFooter,
  DialogHeader,
  DialogTitle,
  DialogDescription
} from '../ui/dialog'
import { DataSource, DataSourcesPanelProps, PendingDataSource, ExportInstructions } from './types'
import { truncatePath } from './utils'

const SUPPORTED_DATA_SOURCES: DataSource[] = [
  {
    name: 'WhatsApp',
    label: 'WhatsApp',
    description: 'Import your WhatsApp chat history',
    selectType: 'files',
    fileRequirement: 'Select WhatsApp SQLITE file',
    icon: <WhatsAppIcon className="h-5 w-5" />,
    fileFilters: [{ name: 'WhatsApp Database', extensions: ['db', 'sqlite'] }]
  },
  {
    name: 'Telegram',
    label: 'Telegram',
    description: 'Import your Telegram messages and media',
    selectType: 'files',
    fileRequirement: 'Select Telegram JSON export file',
    icon: <TelegramIcon className="h-5 w-5" />,
    fileFilters: [{ name: 'Telegram Export', extensions: ['json'] }]
  },
  {
    name: 'Slack',
    label: 'Slack',
    description: 'Import your Slack workspace data',
    selectType: 'files',
    fileRequirement: 'Select Slack ZIP file',
    icon: <SlackIcon className="h-5 w-5" />,
    fileFilters: [{ name: 'Slack Export', extensions: ['zip'] }]
  },
  {
    name: 'Gmail',
    label: 'Gmail',
    description: 'Import your Gmail emails and attachments',
    selectType: 'files',
    fileRequirement: 'Select Google Takeout ZIP file',
    icon: <GmailIcon className="h-5 w-5" />,
    fileFilters: [{ name: 'Google Takeout', extensions: ['zip'] }]
  },
  {
    name: 'X',
    label: 'X',
    description: 'Import your X tweets and messages',
    selectType: 'files',
    fileRequirement: 'Select X ZIP',
    icon: <XformerlyTwitterIcon className="h-5 w-5" />,
    fileFilters: [{ name: 'X Archive', extensions: ['zip'] }]
  }
]

const EXPORT_INSTRUCTIONS: Record<string, ExportInstructions> = {
  WhatsApp: {
    timeEstimate: '5-10 minutes',
    steps: ['Open Finder', 'Find your ChatStorage.sqlite file']
  },
  Telegram: {
    timeEstimate: '10-30 minutes',
    steps: ['Open Finder', 'Find your Telegram Export file']
  },
  Slack: {
    timeEstimate: '1-2 hours',
    steps: [
      'Open Slack in your browser',
      'Click your workspace name in the top left',
      'Go to Settings & Administration > Workspace Settings',
      'Click "Import/Export Data" in the left sidebar',
      'Click "Export" and follow the prompts'
    ]
  },
  Gmail: {
    timeEstimate: '24-48 hours',
    steps: [
      'Go to Google Takeout (takeout.google.com)',
      'Sign in with your Google account',
      'Deselect all services',
      'Select only "Mail"',
      'Click "Next" and choose your export options',
      'Click "Create Export"'
    ],
    link: 'https://takeout.google.com'
  },
  X: {
    timeEstimate: '24-48 hours',
    steps: [
      'Go to X (Twitter) in your browser',
      'Click your profile picture',
      'Go to Settings and Privacy',
      'Click "Download an archive of your data"',
      'Enter your password and click "Confirm"',
      'Wait for the email with your data archive'
    ],
    link: 'https://x.com/settings/download_your_data'
  }
}

const DataSourceCard = ({ source, onClick }: { source: DataSource; onClick: () => void }) => (
  <Button
    variant="outline"
    size="lg"
    className="h-auto py-4 px-4 flex flex-col items-center gap-2 hover:bg-accent/50 bg-card"
    onClick={onClick}
  >
    <div className="flex items-center gap-2">
      {source.icon}
      <span className="font-medium">{source.label}</span>
    </div>
  </Button>
)

const PendingDataSourceCard = ({
  source,
  onRemove
}: {
  source: PendingDataSource
  onRemove: () => void
}) => {
  const sourceDetails = SUPPORTED_DATA_SOURCES.find((s) => s.name === source.name)
  if (!sourceDetails) return null

  return (
    <div className="p-4 rounded-lg bg-card border h-full flex items-center justify-between gap-3">
      <div className="flex items-center gap-3">
        {sourceDetails.icon}
        <div>
          <h3 className="font-medium">{source.name}</h3>
          <p className="text-xs text-muted-foreground">{sourceDetails.description}</p>
        </div>
      </div>
      <div className="flex items-center gap-2">
        <div className="flex items-center gap-1 text-xs text-muted-foreground">
          <CheckCircle2 className="h-3 w-3 text-primary" />
          <span>Selected</span>
        </div>
        <Button variant="ghost" size="icon" className="h-6 w-6" onClick={onRemove}>
          <X className="h-4 w-4 text-muted-foreground" />
        </Button>
      </div>
    </div>
  )
}

const IndexedDataSourceCard = ({ name, isIndexed }: { name: string; isIndexed: boolean }) => {
  const sourceDetails = SUPPORTED_DATA_SOURCES.find((s) => s.name === name)
  if (!sourceDetails) return null

  return (
    <div className="p-4 rounded-lg bg-muted/50 border h-full flex items-center justify-between gap-3">
      <div className="flex items-center gap-3">
        {sourceDetails.icon}
        <div>
          <h3 className="font-medium">{name}</h3>
        </div>
      </div>
      <div className="flex items-center gap-1 text-xs text-muted-foreground">
        {isIndexed ? (
          <CheckCircle2 className="h-3 w-3 text-green-500" />
        ) : (
          <Loader2 className="h-3 w-3 text-amber-500 animate-spin" />
        )}
        <span>{isIndexed ? 'Indexed' : 'Processing...'}</span>
      </div>
    </div>
  )
}

const DataSourceDialog = ({
  selectedSource,
  onClose,
  pendingDataSources,
  onFileSelect,
  onAddSource
}: {
  selectedSource: DataSource | null
  onClose: () => void
  pendingDataSources: Record<string, PendingDataSource>
  onFileSelect: () => void
  onAddSource: () => void
}) => {
  if (!selectedSource) return null

  return (
    <Dialog open={!!selectedSource} onOpenChange={onClose}>
      <DialogContent className="max-w-3xl">
        <DialogHeader>
          <DialogTitle>Add {selectedSource.name} Data</DialogTitle>
          <DialogDescription className="text-sm text-muted-foreground flex items-center gap-2">
            <Clock className="h-4 w-4 text-muted-foreground/50" />
            {EXPORT_INSTRUCTIONS[selectedSource.name]?.timeEstimate}
          </DialogDescription>
        </DialogHeader>
        <div className="space-y-8">
          <div className="space-y-4 py-4">
            <div className="rounded-lg">
              <ol className="space-y-4 flex flex-col gap-3">
                {EXPORT_INSTRUCTIONS[selectedSource.name]?.steps.map((step, index) => (
                  <li key={index} className="flex gap-3">
                    <div className="flex-shrink-0 w-6 h-6 rounded-full bg-accent flex items-center justify-center text-primary font-medium text-sm">
                      {index + 1}
                    </div>
                    <div className="flex-1">
                      <p className="text-sm">{step}</p>
                    </div>
                  </li>
                ))}
              </ol>
              {EXPORT_INSTRUCTIONS[selectedSource.name]?.link && (
                <Button variant="link" className="mt-4 p-0 h-auto text-primary" asChild>
                  <a
                    href={EXPORT_INSTRUCTIONS[selectedSource.name].link}
                    target="_blank"
                    rel="noopener noreferrer"
                  >
                    Open {selectedSource.name} Export Page <ExternalLink className="h-4 w-4" />
                  </a>
                </Button>
              )}
            </div>
          </div>

          <div className="flex flex-col gap-2 bg-card p-4 rounded-lg dark text-white">
            <h4 className="font-medium flex items-center gap-2">
              <File className="h-4 w-4 text-muted-foreground/80" /> Add data
            </h4>
            <div className="flex flex-col gap-2">
              <div className="flex items-center gap-2">
                <div className="flex-1 h-9 px-3 py-1 rounded-md border bg-background text-sm">
                  {pendingDataSources[selectedSource.name]?.path
                    ? truncatePath(pendingDataSources[selectedSource.name]?.path)
                    : selectedSource.fileRequirement}
                </div>
                <Button onClick={onFileSelect}>Browse</Button>
              </div>
            </div>
          </div>
        </div>
        <DialogFooter>
          <Button variant="outline" onClick={onClose}>
            Cancel
          </Button>
          <Button onClick={onAddSource} disabled={!pendingDataSources[selectedSource.name]}>
            Add Source
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  )
}

export function DataSourcesPanel({
  onDataSourceSelected,
  onDataSourceRemoved
}: DataSourcesPanelProps) {
  const { data } = useQuery(GetDataSourcesDocument)
  const [pendingDataSources, setPendingDataSources] = useState<Record<string, PendingDataSource>>(
    {}
  )
  const [selectedSource, setSelectedSource] = useState<DataSource | null>(null)

  const handleRemoveDataSource = (name: string) => {
    setPendingDataSources((prev) => {
      const newState = { ...prev }
      delete newState[name]
      return newState
    })
    onDataSourceRemoved?.(name)
  }

  const handleSourceSelected = (source: DataSource) => {
    setSelectedSource(source)
    onDataSourceSelected?.(source)
  }

  const handleFileSelect = async () => {
    if (!selectedSource) return

    try {
      const result = await (selectedSource.selectType === 'directory'
        ? window.api.selectDirectory()
        : window.api.selectFiles({
            filters: selectedSource.fileFilters || []
          }))

      if (result.canceled) {
        toast.info('File selection cancelled')
        return
      }

      const path = result.filePaths[0]
      setPendingDataSources((prev) => ({
        ...prev,
        [selectedSource.name]: { name: selectedSource.name, path }
      }))
    } catch (error) {
      console.error('Error selecting files:', error)
      toast.error('Failed to select data source. Please try again.')
    }
  }

  return (
    <>
      <div className="space-y-6">
        <div className="grid grid-cols-1 gap-4">
          <div className="">
            <div className="flex flex-wrap justify-center gap-3">
              {SUPPORTED_DATA_SOURCES.map((source) => (
                <DataSourceCard
                  key={source.name}
                  source={source}
                  onClick={() => handleSourceSelected(source)}
                />
              ))}
            </div>
          </div>

          <div className="grid grid-cols-2 gap-4">
            {Object.entries(pendingDataSources).map(([name]) => (
              <PendingDataSourceCard
                key={name}
                source={pendingDataSources[name]}
                onRemove={() => handleRemoveDataSource(name)}
              />
            ))}

            {data?.getDataSources?.map(({ id, name, isIndexed }) => (
              <IndexedDataSourceCard key={id} name={name} isIndexed={isIndexed} />
            ))}
          </div>
        </div>
      </div>

      <DataSourceDialog
        selectedSource={selectedSource}
        onClose={() => setSelectedSource(null)}
        pendingDataSources={pendingDataSources}
        onFileSelect={handleFileSelect}
        onAddSource={() => setSelectedSource(null)}
      />
    </>
  )
}
