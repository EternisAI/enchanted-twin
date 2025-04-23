import { OnboardingLayout } from './OnboardingLayout'
import { useMutation, useQuery } from '@apollo/client'
import { gql } from '@apollo/client'
import { HelpCircle, CheckCircle2, AlertCircle, File, Folder, Loader2, X, Plus } from 'lucide-react'
import { Button } from '../ui/button'
import { useState } from 'react'
import { ImportInstructionsModal } from './ImportInstructionsModal'
import { DataSource } from '@renderer/graphql/generated/graphql'
import { cn } from '@renderer/lib/utils'
import { toast } from 'sonner'
import { Skeleton } from '@renderer/components/ui/skeleton'
import { useOnboardingStore } from '@renderer/lib/stores/onboarding'

const ADD_DATA_SOURCE = gql`
  mutation AddDataSource($name: String!, $path: String!) {
    addDataSource(name: $name, path: $path)
  }
`

const DELETE_DATA_SOURCE = gql`
  mutation DeleteDataSource($id: ID!) {
    deleteDataSource(id: $id)
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

export function ImportDataStep() {
  const { data, refetch, loading } = useQuery(GET_DATA_SOURCES)
  const [addDataSource] = useMutation(ADD_DATA_SOURCE)
  const [deleteDataSource] = useMutation(DELETE_DATA_SOURCE)
  const [showImportInstructions, setShowImportInstructions] = useState<string | null>(null)
  const [isSelecting, setIsSelecting] = useState<string | null>(null)
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
      await addDataSource({
        variables: {
          name,
          path
        }
      })

      await refetch()
      toast.success(`${name} data source added successfully`)
    } catch (error) {
      console.error('Error selecting files:', error)
      toast.error('Failed to add data source. Please try again.')
    } finally {
      setIsSelecting(null)
    }
  }

  const handleNext = async () => {
    const isValid = await validateDataSources()
    if (isValid) {
      nextStep()
    }
  }

  const handleRemoveDataSource = async (id: string) => {
    try {
      await deleteDataSource({
        variables: { id }
      })
      await refetch()
      toast.success('Data source removed successfully')
    } catch (error) {
      console.error('Error removing data source:', error)
      toast.error('Failed to remove data source. Please try again.')
    }
  }

  const validateDataSources = async () => {
    if (!data?.getDataSources?.length) {
      toast.error('Please select at least one data source')
      return false
    }
    return true
  }

  // Group data sources by type
  const dataSourcesByType =
    data?.getDataSources?.reduce(
      (acc, ds) => {
        if (!acc[ds.name]) {
          acc[ds.name] = []
        }
        acc[ds.name].push(ds)
        return acc
      },
      {} as Record<string, DataSource[]>
    ) || {}

  return (
    <OnboardingLayout
      title="Import Your Data"
      subtitle="Select the data sources you'd like to import. You can always add more later."
    >
      <div className="space-y-6">
        <div className="grid grid-cols-1 md:grid-cols-2 gap-4">
          {loading
            ? // Skeleton loading state
              Array.from({ length: SUPPORTED_DATA_SOURCES.length }).map((_, index) => (
                <div key={index} className="relative h-full">
                  <div className="p-4 rounded-lg bg-card border h-full flex flex-col justify-between gap-3">
                    <div className="flex items-center justify-between">
                      <div className="flex items-center gap-2">
                        <Skeleton className="h-5 w-5" />
                        <Skeleton className="h-5 w-24" />
                      </div>
                      <Skeleton className="h-6 w-6" />
                    </div>
                    <div className="flex flex-col gap-1 flex-1">
                      <Skeleton className="h-4 w-full" />
                      <Skeleton className="h-3 w-3/4" />
                    </div>
                    <Skeleton className="h-9 w-full" />
                  </div>
                </div>
              ))
            : SUPPORTED_DATA_SOURCES.map((source) => {
                const existingSources = dataSourcesByType[source.name] || []
                const isSelectingThisSource = isSelecting === source.name

                return (
                  <div key={source.name} className="flex flex-col">
                    <div className="flex-1">
                      <DataSourceCard
                        dataSource={existingSources[0]}
                        sourceDetails={source}
                        isSelecting={isSelectingThisSource}
                        onBeginSelection={handleFileSelect}
                        onShowInstructions={() => setShowImportInstructions(source.name)}
                        onRemove={handleRemoveDataSource}
                      />
                    </div>
                    {existingSources.length > 0 && (
                      <div className="mt-2">
                        <Button
                          variant="outline"
                          size="sm"
                          className="w-full"
                          onClick={() => handleFileSelect(source.name, source.selectType)}
                          disabled={isSelectingThisSource}
                        >
                          {isSelectingThisSource ? (
                            <>
                              <Loader2 className="mr-2 h-4 w-4 animate-spin" />
                              Selecting...
                            </>
                          ) : (
                            <>
                              <Plus className="mr-2 h-4 w-4" />
                              Add Another {source.name} Source
                            </>
                          )}
                        </Button>
                      </div>
                    )}
                  </div>
                )
              })}
        </div>
        <Button onClick={handleNext}>Next</Button>
      </div>
      <ImportInstructionsModal
        isOpen={!!showImportInstructions}
        onClose={() => setShowImportInstructions(null)}
        dataSource={showImportInstructions || ''}
      />
    </OnboardingLayout>
  )
}

function DataSourceCard({
  dataSource,
  sourceDetails,
  isSelecting,
  onBeginSelection,
  onShowInstructions,
  onRemove
}: {
  dataSource?: DataSource
  sourceDetails: (typeof SUPPORTED_DATA_SOURCES)[0]
  isSelecting: boolean
  onBeginSelection: (name: string, selectType: 'files' | 'directory') => void
  onShowInstructions: () => void
  onRemove: (id: string) => void
}) {
  const isSelected = !!dataSource
  const { name, description, fileRequirement, selectType, icon } = sourceDetails

  return (
    <div
      className={cn(
        'relative h-full transition-all duration-200',
        isSelected && 'ring-2 ring-primary ring-offset-2 rounded-lg'
      )}
    >
      <div
        className={cn(
          'p-4 rounded-lg bg-card border h-full flex flex-col justify-between gap-3 transition-colors',
          isSelected && 'border-primary'
        )}
      >
        <div className="flex items-center justify-between">
          <div className="flex items-center gap-2">
            {icon}
            <h3 className="font-medium">{name}</h3>
          </div>
          <div className="flex items-center gap-1">
            <Button
              variant="ghost"
              size="icon"
              className="h-6 w-6"
              onClick={(e) => {
                e.stopPropagation()
                onShowInstructions()
              }}
            >
              <HelpCircle className="h-4 w-4 text-muted-foreground" />
            </Button>
            {isSelected && !dataSource.isIndexed && (
              <Button
                variant="ghost"
                size="icon"
                className="h-6 w-6"
                onClick={(e) => {
                  e.stopPropagation()
                  onRemove(dataSource.id)
                }}
              >
                <X className="h-4 w-4 text-muted-foreground" />
              </Button>
            )}
          </div>
        </div>

        <div className="flex flex-col gap-1 flex-1">
          <p className="text-sm text-muted-foreground">{description}</p>
          <p className="text-muted-foreground text-xs">{fileRequirement}</p>
        </div>

        {isSelected ? (
          <div className="space-y-2">
            <div className="flex items-center justify-between">
              <div className="flex items-center gap-2">
                {dataSource.hasError ? (
                  <AlertCircle className="h-4 w-4 text-destructive" />
                ) : dataSource.isIndexed ? (
                  <CheckCircle2 className="h-4 w-4 text-green-500" />
                ) : (
                  <CheckCircle2 className="h-4 w-4 text-primary" />
                )}
                <span className="text-sm">
                  {dataSource.hasError ? 'Error' : dataSource.isIndexed ? 'Indexed' : 'Selected'}
                </span>
              </div>
              {!dataSource.isIndexed && (
                <Button
                  variant="outline"
                  size="sm"
                  onClick={() => onBeginSelection(name, selectType)}
                  disabled={isSelecting}
                >
                  Change
                </Button>
              )}
            </div>
            <p className="text-muted-foreground text-xs truncate">{dataSource.path}</p>
          </div>
        ) : (
          <Button
            variant="outline"
            size="sm"
            className="w-full"
            onClick={() => onBeginSelection(name, selectType)}
            disabled={isSelecting}
          >
            {isSelecting ? (
              <>
                <Loader2 className="mr-2 h-4 w-4 animate-spin" />
                Selecting...
              </>
            ) : (
              `Select ${selectType === 'files' ? 'File' : 'Folder'}`
            )}
          </Button>
        )}
      </div>
    </div>
  )
}
