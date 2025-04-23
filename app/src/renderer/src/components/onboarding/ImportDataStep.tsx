import { OnboardingLayout } from './OnboardingLayout'
import { useMutation, useQuery } from '@apollo/client'
import { gql } from '@apollo/client'
import { HelpCircle, CheckCircle2, Loader2, X, File, Folder } from 'lucide-react'
import { Button } from '../ui/button'
import { useState } from 'react'
import { ImportInstructionsModal } from './ImportInstructionsModal'
import { toast } from 'sonner'
import { useOnboardingStore } from '@renderer/lib/stores/onboarding'
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle
} from '../ui/dialog'

const ADD_DATA_SOURCE = gql`
  mutation AddDataSource($name: String!, $path: String!) {
    addDataSource(name: $name, path: $path)
  }
`

const GET_DATA_SOURCES = gql`
  query GetDataSources {
    getDataSources {
      id
      name
      path
      updatedAt
      isProcessed
      isIndexed
      hasError
    }
  }
`

const SUPPORTED_DATA_SOURCES: {
  name: string
  label: string
  description: string
  selectType: 'directory' | 'files'
  fileRequirement: string
  icon: React.ReactNode
}[] = [
  {
    name: 'WhatsApp',
    label: 'WhatsApp',
    description: 'Import your WhatsApp chat history',
    selectType: 'files',
    fileRequirement: 'Select your WhatsApp SQLITE database file',
    icon: <File className="h-5 w-5" />
  },
  {
    name: 'Telegram',
    label: 'Telegram',
    description: 'Import your Telegram messages and media',
    selectType: 'files',
    fileRequirement: 'Select your Telegram JSON export file',
    icon: <File className="h-5 w-5" />
  },
  {
    name: 'Slack',
    label: 'Slack',
    description: 'Import your Slack workspace data',
    selectType: 'files',
    fileRequirement: 'Select your exported Slack ZIP file',
    icon: <File className="h-5 w-5" />
  },
  {
    name: 'Gmail',
    label: 'Gmail',
    description: 'Import your Gmail emails and attachments',
    selectType: 'files',
    fileRequirement: 'Select your Google Takeout ZIP file',
    icon: <File className="h-5 w-5" />
  },
  {
    name: 'X',
    label: 'X',
    description: 'Import your X tweets and messages',
    selectType: 'files',
    fileRequirement:
      'Select the ZIP file containing X .js files (like.js, direct-messages.js, tweets.js)',
    icon: <Folder className="h-5 w-5" />
  }
  // {
  //   name: 'GoogleAddresses',
  //   label: 'Google Addresses',
  //   description: 'Import your Google Addresses from Location History',
  //   selectType: 'files',
  //   fileRequirement: 'Select the folder containing addresses.json files from Google Takeout'
  // }
]

const EXPORT_INSTRUCTIONS: Record<string, { steps: string[]; link?: string }> = {
  WhatsApp: {
    steps: [
      'Open WhatsApp on your phone',
      'Go to Settings > Chats > Chat Backup',
      'Tap "Back Up" to create a backup',
      'Connect your phone to your computer',
      'Navigate to the WhatsApp backup folder on your phone',
      'Copy the msgstore.db.crypt12 file to your computer'
    ]
  },
  Telegram: {
    steps: [
      'Open Telegram Desktop',
      'Click the menu button (three lines)',
      'Go to Settings > Advanced > Export Telegram Data',
      'Select the data you want to export',
      'Click "Export" and save the file'
    ]
  },
  Slack: {
    steps: [
      'Open Slack in your browser',
      'Click your workspace name in the top left',
      'Go to Settings & Administration > Workspace Settings',
      'Click "Import/Export Data" in the left sidebar',
      'Click "Export" and follow the prompts'
    ]
  },
  Gmail: {
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
    steps: [
      'Go to X (Twitter) in your browser',
      'Click your profile picture',
      'Go to Settings and Privacy',
      'Click "Download an archive of your data"',
      'Enter your password and click "Confirm"',
      'Wait for the email with your data archive'
    ]
  }
}

export function ImportDataStep() {
  const { data, refetch } = useQuery(GET_DATA_SOURCES)
  const [addDataSource] = useMutation(ADD_DATA_SOURCE)
  const [showImportInstructions, setShowImportInstructions] = useState<string | null>(null)
  const [isSelecting, setIsSelecting] = useState<string | null>(null)
  const [pendingDataSources, setPendingDataSources] = useState<
    Record<string, { name: string; path: string }>
  >({})
  const [isProcessing, setIsProcessing] = useState(false)
  const [selectedSource, setSelectedSource] = useState<{
    name: string
    selectType: 'directory' | 'files'
  } | null>(null)
  const { nextStep } = useOnboardingStore()

  const handleFileSelect = async (name: string, selectType: 'directory' | 'files') => {
    try {
      setIsSelecting(name)
      const result = await (selectType === 'directory'
        ? window.api.selectDirectory()
        : window.api.selectFiles())

      if (result.canceled) {
        toast.info('File selection cancelled')
        return
      }

      const path = result.filePaths[0]
      setPendingDataSources((prev) => ({
        ...prev,
        [name]: { name, path }
      }))
      setSelectedSource(null)
    } catch (error) {
      console.error('Error selecting files:', error)
      toast.error('Failed to select data source. Please try again.')
    } finally {
      setIsSelecting(null)
    }
  }

  const handleNext = async () => {
    if (Object.keys(pendingDataSources).length === 0) {
      toast.error('Please select at least one data source')
      return
    }

    setIsProcessing(true)
    try {
      // Add all pending data sources
      for (const source of Object.values(pendingDataSources)) {
        await addDataSource({
          variables: {
            name: source.name,
            path: source.path
          }
        })
      }

      await refetch()
      toast.success('Data sources added successfully')
      nextStep()
    } catch (error) {
      console.error('Error adding data sources:', error)
      toast.error('Failed to add data sources. Please try again.')
    } finally {
      setIsProcessing(false)
    }
  }

  const handleRemoveDataSource = (name: string) => {
    setPendingDataSources((prev) => {
      const newState = { ...prev }
      delete newState[name]
      return newState
    })
  }

  return (
    <OnboardingLayout
      title="Import Your Data"
      subtitle="Select the data sources you'd like to import. You can always add more later."
      onClose={nextStep}
    >
      <div className="space-y-6">
        <div className="grid grid-cols-1 gap-4">
          {/* Add Source Card */}
          <div className="p-4 rounded-lg bg-card border h-full flex flex-col justify-between gap-3">
            <div className="flex items-center justify-between">
              <h3 className="font-semibold">Add Source</h3>
            </div>
            <div className="flex flex-col gap-1 flex-1">
              <p className="text-sm text-muted-foreground">Select a data source to import</p>
            </div>
            <div className="grid grid-cols-2 md:grid-cols-4 gap-2">
              {SUPPORTED_DATA_SOURCES.map((source) => (
                <Button
                  key={source.name}
                  variant="outline"
                  size="sm"
                  className="w-full"
                  onClick={() =>
                    setSelectedSource({ name: source.name, selectType: source.selectType })
                  }
                  disabled={isSelecting === source.name}
                >
                  {source.name}
                </Button>
              ))}
            </div>
          </div>

          {/* Selected and Indexed Sources */}
          <div className="grid grid-cols-1 md:grid-cols-2 gap-4">
            {Object.entries(pendingDataSources).map(([name, source]) => {
              const sourceDetails = SUPPORTED_DATA_SOURCES.find((s) => s.name === name)
              if (!sourceDetails) return null

              return (
                <div
                  key={name}
                  className="p-4 rounded-lg bg-card border h-full flex flex-col justify-between gap-3"
                >
                  <div className="flex items-center justify-between">
                    <div className="flex items-center gap-2">
                      <h3 className="font-semibold">{name}</h3>
                    </div>
                    <div className="flex items-center gap-1">
                      <Button
                        variant="ghost"
                        size="icon"
                        className="h-6 w-6"
                        onClick={() => setShowImportInstructions(name)}
                      >
                        <HelpCircle className="h-4 w-4 text-muted-foreground" />
                      </Button>
                      <Button
                        variant="ghost"
                        size="icon"
                        className="h-6 w-6"
                        onClick={() => handleRemoveDataSource(name)}
                      >
                        <X className="h-4 w-4 text-muted-foreground" />
                      </Button>
                    </div>
                  </div>
                  <div className="flex flex-col gap-1 flex-1">
                    <p className="text-sm text-muted-foreground">{sourceDetails.description}</p>
                    <p className="text-muted-foreground text-xs">{sourceDetails.fileRequirement}</p>
                  </div>
                  <div className="space-y-2">
                    <div className="flex items-center gap-2">
                      <CheckCircle2 className="h-4 w-4 text-primary" />
                      <span className="text-sm">Selected</span>
                    </div>
                    <p className="text-muted-foreground text-xs truncate">{source.path}</p>
                  </div>
                </div>
              )
            })}

            {/* Indexed Sources */}
            {data?.getDataSources?.map((source) => {
              const sourceDetails = SUPPORTED_DATA_SOURCES.find((s) => s.name === source.name)
              if (!sourceDetails) return null

              return (
                <div
                  key={source.id}
                  className="p-4 rounded-lg bg-card border h-full flex flex-col justify-between gap-3"
                >
                  <div className="flex items-center justify-between">
                    <div className="flex items-center gap-2">
                      <h3 className="font-semibold">{source.name}</h3>
                    </div>
                    <div className="flex items-center gap-1">
                      <Button
                        variant="ghost"
                        size="icon"
                        className="h-6 w-6"
                        onClick={() => setShowImportInstructions(source.name)}
                      >
                        <HelpCircle className="h-4 w-4 text-muted-foreground" />
                      </Button>
                    </div>
                  </div>
                  <div className="flex flex-col gap-1 flex-1">
                    <p className="text-sm text-muted-foreground">{sourceDetails.description}</p>
                  </div>
                  <div className="space-y-2">
                    <div className="flex items-center gap-2">
                      <CheckCircle2 className="h-4 w-4 text-green-500" />
                      <span className="text-sm">Indexed</span>
                    </div>
                    <p className="text-muted-foreground text-xs truncate">{source.path}</p>
                  </div>
                </div>
              )
            })}
          </div>
        </div>
        <Button onClick={handleNext} disabled={isProcessing}>
          {isProcessing ? (
            <>
              <Loader2 className="mr-2 h-4 w-4 animate-spin" />
              Processing...
            </>
          ) : (
            'Next'
          )}
        </Button>
      </div>

      {/* Data Source Selection Modal */}
      <Dialog open={!!selectedSource} onOpenChange={() => setSelectedSource(null)}>
        <DialogContent className="max-w-2xl">
          <DialogHeader>
            <DialogTitle>Add {selectedSource?.name} Data</DialogTitle>
            <DialogDescription>
              {SUPPORTED_DATA_SOURCES.find((s) => s.name === selectedSource?.name)?.description}
            </DialogDescription>
          </DialogHeader>
          <div className="space-y-6">
            <div className="space-y-4">
              <h4 className="font-medium">How to export your data:</h4>
              <ol className="list-decimal list-inside space-y-2 text-sm text-muted-foreground">
                {selectedSource &&
                  EXPORT_INSTRUCTIONS[selectedSource.name]?.steps.map((step, index) => (
                    <li key={index}>{step}</li>
                  ))}
              </ol>
              {selectedSource && EXPORT_INSTRUCTIONS[selectedSource.name]?.link && (
                <Button
                  variant="link"
                  className="p-0 h-auto"
                  onClick={() =>
                    window.electron.ipcRenderer.send(
                      'open-external-url',
                      EXPORT_INSTRUCTIONS[selectedSource.name].link
                    )
                  }
                >
                  Open {selectedSource.name} Export Page
                </Button>
              )}
            </div>

            <div className="space-y-4">
              <h4 className="font-medium">Select your exported data:</h4>
              <p className="text-sm text-muted-foreground">
                {
                  SUPPORTED_DATA_SOURCES.find((s) => s.name === selectedSource?.name)
                    ?.fileRequirement
                }
              </p>
              <Button
                className="w-full"
                onClick={() =>
                  selectedSource && handleFileSelect(selectedSource.name, selectedSource.selectType)
                }
                disabled={isSelecting === selectedSource?.name}
              >
                {isSelecting === selectedSource?.name ? (
                  <>
                    <Loader2 className="mr-2 h-4 w-4 animate-spin" />
                    Selecting...
                  </>
                ) : (
                  `Select ${selectedSource?.selectType === 'files' ? 'File' : 'Folder'}`
                )}
              </Button>
            </div>
          </div>
          <DialogFooter>
            <Button variant="outline" onClick={() => setSelectedSource(null)}>
              Cancel
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>

      <ImportInstructionsModal
        isOpen={!!showImportInstructions}
        onClose={() => setShowImportInstructions(null)}
        dataSource={showImportInstructions || ''}
      />
    </OnboardingLayout>
  )
}
