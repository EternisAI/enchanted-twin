import { OnboardingLayout } from './OnboardingLayout'
import { useMutation, useQuery } from '@apollo/client'
import { gql } from '@apollo/client'
import {
  CheckCircle2,
  Loader2,
  X,
  MessageSquare,
  Mail,
  Twitter,
  Clock,
  File,
  ExternalLink,
  ArrowRight
} from 'lucide-react'
import { Button } from '../ui/button'
import { useState } from 'react'
import { toast } from 'sonner'
import { useOnboardingStore } from '@renderer/lib/stores/onboarding'
import {
  Dialog,
  DialogContent,
  DialogFooter,
  DialogHeader,
  DialogTitle,
  DialogDescription
} from '../ui/dialog'
import { GetDataSourcesDocument } from '@renderer/graphql/generated/graphql'

const ADD_DATA_SOURCE = gql`
  mutation AddDataSource($name: String!, $path: String!) {
    addDataSource(name: $name, path: $path)
  }
`

const SUPPORTED_DATA_SOURCES: {
  name: string
  label: string
  description: string
  selectType: 'directory' | 'files'
  fileRequirement: string
  icon: React.ReactNode
  fileFilters?: { name: string; extensions: string[] }[]
}[] = [
  {
    name: 'WhatsApp',
    label: 'WhatsApp',
    description: 'Import your WhatsApp chat history',
    selectType: 'files',
    fileRequirement: 'Select WhatsApp SQLITE file',
    icon: <MessageSquare className="h-5 w-5 text-green-500" />,
    fileFilters: [{ name: 'WhatsApp Database', extensions: ['db', 'sqlite'] }]
  },
  {
    name: 'Telegram',
    label: 'Telegram',
    description: 'Import your Telegram messages and media',
    selectType: 'files',
    fileRequirement: 'Select Telegram JSON export file',
    icon: <MessageSquare className="h-5 w-5 text-blue-500" />,
    fileFilters: [{ name: 'Telegram Export', extensions: ['json'] }]
  },
  {
    name: 'Slack',
    label: 'Slack',
    description: 'Import your Slack workspace data',
    selectType: 'files',
    fileRequirement: 'Select Slack ZIP file',
    icon: <MessageSquare className="h-5 w-5 text-purple-500" />,
    fileFilters: [{ name: 'Slack Export', extensions: ['zip'] }]
  },
  {
    name: 'Gmail',
    label: 'Gmail',
    description: 'Import your Gmail emails and attachments',
    selectType: 'files',
    fileRequirement: 'Select Google Takeout ZIP file',
    icon: <Mail className="h-5 w-5 text-red-500" />,
    fileFilters: [{ name: 'Google Takeout', extensions: ['zip'] }]
  },
  {
    name: 'X',
    label: 'X',
    description: 'Import your X tweets and messages',
    selectType: 'files',
    fileRequirement: 'Select X ZIP',
    icon: <Twitter className="h-5 w-5 text-black dark:text-white" />,
    fileFilters: [{ name: 'X Archive', extensions: ['zip'] }]
  }
  // {
  //   name: 'GoogleAddresses',
  //   label: 'Google Addresses',
  //   description: 'Import your Google Addresses from Location History',
  //   selectType: 'files',
  //   fileRequirement: 'Select the folder containing addresses.json files from Google Takeout'
  // }
]

const EXPORT_INSTRUCTIONS: Record<
  string,
  {
    timeEstimate: string
    steps: string[]
    link?: string
  }
> = {
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

const truncatePath = (path: string) => {
  return path.length > 40 ? '...' + path.slice(-37) : path
}

export function ImportDataStep() {
  const { data, refetch } = useQuery(GetDataSourcesDocument)
  const [addDataSource] = useMutation(ADD_DATA_SOURCE)
  const [pendingDataSources, setPendingDataSources] = useState<
    Record<string, { name: string; path: string }>
  >({})
  const [isProcessing, setIsProcessing] = useState(false)
  const [selectedSource, setSelectedSource] = useState<{
    name: string
    selectType: 'directory' | 'files'
  } | null>(null)
  const { nextStep } = useOnboardingStore()

  const handleNext = async () => {
    if (Object.keys(pendingDataSources).length === 0) {
      nextStep()
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
          <div className="">
            <div className="flex flex-wrap justify-center gap-3">
              {SUPPORTED_DATA_SOURCES.map((source) => (
                <Button
                  key={source.name}
                  variant="outline"
                  size="lg"
                  className="h-auto py-4 px-4 flex flex-col items-center gap-2 hover:bg-accent/50 bg-card"
                  onClick={() =>
                    setSelectedSource({ name: source.name, selectType: source.selectType })
                  }
                >
                  <div className="flex items-center gap-2">
                    {source.icon}
                    <span className="font-medium">{source.label}</span>
                  </div>
                </Button>
              ))}
            </div>
          </div>

          {/* Selected and Indexed Sources */}
          <div className="grid grid-cols-2 gap-4">
            {Object.entries(pendingDataSources).map(([name]) => {
              const sourceDetails = SUPPORTED_DATA_SOURCES.find((s) => s.name === name)
              if (!sourceDetails) return null

              return (
                <div
                  key={name}
                  className="p-4 rounded-lg bg-card border h-full flex items-center justify-between gap-3"
                >
                  <div className="flex items-center gap-3">
                    {sourceDetails.icon}
                    <div>
                      <h3 className="font-medium">{name}</h3>
                      <p className="text-xs text-muted-foreground">{sourceDetails.description}</p>
                    </div>
                  </div>
                  <div className="flex items-center gap-2">
                    <div className="flex items-center gap-1 text-xs text-muted-foreground">
                      <CheckCircle2 className="h-3 w-3 text-primary" />
                      <span>Selected</span>
                    </div>
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
              )
            })}

            {/* Indexed Sources */}
            {data?.getDataSources?.map(({ id, name, isIndexed }) => {
              const sourceDetails = SUPPORTED_DATA_SOURCES.find((s) => s.name === name)
              if (!sourceDetails) return null

              return (
                <div
                  key={id}
                  className="p-4 rounded-lg bg-muted/50 border h-full flex items-center justify-between gap-3"
                >
                  <div className="flex items-center gap-3">
                    {sourceDetails.icon}
                    <div>
                      <h3 className="font-medium">{name}</h3>
                      {/* <p className="text-xs text-muted-foreground">{sourceDetails.description}</p> */}
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
            })}
          </div>
        </div>
        <div className="flex justify-end pt-8">
          <Button onClick={handleNext} disabled={isProcessing}>
            {isProcessing ? (
              <>
                <Loader2 className="mr-2 h-4 w-4 animate-spin" />
                Processing...
              </>
            ) : (
              <>
                Next <ArrowRight className="ml-2 h-4 w-4" />
              </>
            )}
          </Button>
        </div>
      </div>

      {/* Data Source Selection Modal */}
      <Dialog open={!!selectedSource} onOpenChange={() => setSelectedSource(null)}>
        <DialogContent className="max-w-3xl">
          <DialogHeader>
            <DialogTitle>Add {selectedSource?.name} Data</DialogTitle>
            <DialogDescription className="text-sm text-muted-foreground flex items-center gap-2">
              <Clock className="h-4 w-4 text-muted-foreground/50" />
              {selectedSource && EXPORT_INSTRUCTIONS[selectedSource.name]?.timeEstimate}
            </DialogDescription>
          </DialogHeader>
          <div className="space-y-8">
            {/* Overview Section */}
            <div className="rounded-lg">
              <div className="flex items-center justify-between mb-2"></div>
            </div>

            {/* Steps Section */}
            <div className="space-y-4 py-4">
              <div className="rounded-lg">
                <ol className="space-y-4 flex flex-col gap-3">
                  {selectedSource &&
                    EXPORT_INSTRUCTIONS[selectedSource.name]?.steps.map((step, index) => (
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
                {selectedSource && EXPORT_INSTRUCTIONS[selectedSource.name]?.link && (
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

            {/* File Selection Section */}
            <div className="flex flex-col gap-2 bg-card p-4 rounded-lg dark text-white">
              <h4 className="font-medium flex items-center gap-2">
                <File className="h-4 w-4 text-muted-foreground/80" /> Add data
              </h4>
              <div className="flex flex-col gap-2">
                <div className="flex items-center gap-2">
                  <div className="flex-1 h-9 px-3 py-1 rounded-md border bg-background text-sm">
                    {pendingDataSources[selectedSource?.name || '']?.path
                      ? truncatePath(pendingDataSources[selectedSource?.name || '']?.path)
                      : SUPPORTED_DATA_SOURCES.find((s) => s.name === selectedSource?.name)
                          ?.fileRequirement}
                  </div>
                  <Button
                    onClick={async () => {
                      if (!selectedSource) return

                      try {
                        const source = SUPPORTED_DATA_SOURCES.find(
                          (s) => s.name === selectedSource.name
                        )
                        const result = await (selectedSource.selectType === 'directory'
                          ? window.api.selectDirectory()
                          : window.api.selectFiles({
                              filters: source?.fileFilters || []
                            }))

                        if (result.canceled) {
                          toast.info('File selection cancelled')
                          return
                        }

                        const path = result.filePaths[0]
                        setSelectedSource({
                          ...selectedSource
                        })
                        setPendingDataSources((prev) => ({
                          ...prev,
                          [selectedSource.name]: { name: selectedSource.name, path }
                        }))
                      } catch (error) {
                        console.error('Error selecting files:', error)
                        toast.error('Failed to select data source. Please try again.')
                      }
                    }}
                  >
                    Browse
                  </Button>
                </div>
              </div>
            </div>
          </div>
          <DialogFooter>
            <Button variant="outline" onClick={() => setSelectedSource(null)}>
              Cancel
            </Button>
            <Button
              onClick={async () => {
                if (!selectedSource) return

                const source = pendingDataSources[selectedSource.name]
                if (!source) return

                try {
                  setSelectedSource(null)
                } catch (error) {
                  console.error('Error adding data source:', error)
                  toast.error('Failed to add data source. Please try again.')
                }
              }}
              disabled={!pendingDataSources[selectedSource?.name || '']}
            >
              Add Source
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>
    </OnboardingLayout>
  )
}
