import { useOnboardingStore } from '@renderer/lib/stores/onboarding'
import { OnboardingLayout } from './OnboardingLayout'
import { useMutation } from '@apollo/client'
import { gql } from '@apollo/client'
import { HelpCircle } from 'lucide-react'
import { Button } from '../ui/button'
import { useState } from 'react'
import { ImportInstructionsModal } from './ImportInstructionsModal'

const ADD_DATA_SOURCE = gql`
  mutation AddDataSource($name: String!, $path: String!) {
    addDataSource(name: $name, path: $path)
  }
`

const DATA_SOURCES: {
  name: string
  label: string
  description: string
  selectType: 'directory' | 'files'
  fileRequirement: string
}[] = [
  {
    name: 'WhatsApp',
    label: 'WhatsApp',
    description: 'Import your WhatsApp chat history',
    selectType: 'files',
    fileRequirement: 'Select your WhatsApp chat export file'
  },
  {
    name: 'Telegram',
    label: 'Telegram',
    description: 'Import your Telegram messages and media',
    selectType: 'files',
    fileRequirement: 'Select your Telegram JSON export file'
  },
  {
    name: 'Slack',
    label: 'Slack',
    description: 'Import your Slack workspace data',
    selectType: 'directory',
    fileRequirement: 'Select the folder containing channel folders'
  },
  {
    name: 'Gmail',
    label: 'Gmail',
    description: 'Import your Gmail emails and attachments',
    selectType: 'files',
    fileRequirement: 'Select your Gmail .mbox file from Google Takeout'
  },
  {
    name: 'X',
    label: 'X',
    description: 'Import your X tweets and messages',
    selectType: 'directory',
    fileRequirement:
      'Select the folder containing X .js files (like.js, direct-messages.js, tweets.js)'
  },
  {
    name: 'GoogleAddresses',
    label: 'Google Addresses',
    description: 'Import your Google Addresses from Location History',
    selectType: 'directory',
    fileRequirement: 'Select the folder containing addresses.json files from Google Takeout'
  }
]

export function ImportDataStep() {
  const { addDataSource, dataSources, removeDataSource } = useOnboardingStore()
  const [addDataSourceMutation] = useMutation(ADD_DATA_SOURCE)
  const [selectedDataSource, setSelectedDataSource] = useState<string | null>(null)

  const handleRemoveDataSource = (name: string) => {
    const source = dataSources.find((ds) => ds.name === name)
    if (source) {
      removeDataSource(source.id)
    }
  }

  const handleFileSelect = async (name: string, selectType: 'directory' | 'files') => {
    try {
      // Open file dialog based on selection type
      const result = await (selectType === 'directory'
        ? window.api.selectDirectory()
        : window.api.selectFiles())

      if (result.canceled) return

      // Call GraphQL mutation to add data source
      const { data } = await addDataSourceMutation({
        variables: {
          name,
          path: result.filePaths[0]
        }
      })

      if (data?.addDataSource) {
        // Add data source to store
        addDataSource({
          id: crypto.randomUUID(), // Generate a unique ID
          name,
          path: result.filePaths[0],
          updatedAt: new Date(),
          isProcessed: false,
          isIndexed: false
        })
      }
    } catch (error) {
      console.error('Error selecting files:', error)
    }
  }

  return (
    <OnboardingLayout
      title="Import Your Data"
      subtitle="Select the data sources you'd like to import. You can always add more later."
    >
      <div className="space-y-6">
        <div className="grid grid-cols-1 md:grid-cols-2 gap-4">
          {DATA_SOURCES.map(({ name, label, description, selectType, fileRequirement }) => {
            const isSelected = dataSources.some((ds) => ds.name === name)
            const source = dataSources.find((ds) => ds.name === name)

            return (
              <div key={name} className="relative">
                <div
                  className={`p-4 rounded-lg bg-card border ${
                    isSelected ? 'border-primary bg-primary/5' : 'border-border'
                  } transition-colors`}
                >
                  <div className="space-y-2">
                    <div className="flex items-center justify-between">
                      <h3 className="font-medium">{label}</h3>
                      <Button
                        variant="ghost"
                        size="icon"
                        className="absolute top-2 right-2 h-6 w-6"
                        onClick={(e) => {
                          e.stopPropagation()
                          setSelectedDataSource(name)
                        }}
                      >
                        <HelpCircle className="h-4 w-4 text-muted-foreground" />
                      </Button>
                    </div>
                    <p className="text-sm text-muted-foreground">{description}</p>
                    {isSelected ? (
                      <div className="text-sm">
                        <div className="flex items-center justify-between">
                          <div className="flex gap-2">
                            <Button
                              variant="outline"
                              size="sm"
                              onClick={() => handleFileSelect(name, selectType)}
                            >
                              Change
                            </Button>
                            <Button
                              variant="destructive"
                              size="sm"
                              onClick={() => handleRemoveDataSource(name)}
                            >
                              Remove
                            </Button>
                          </div>
                        </div>
                        <p className="text-muted-foreground text-xs truncate">{source?.path}</p>
                        <p className="text-muted-foreground text-xs mt-1">{fileRequirement}</p>
                      </div>
                    ) : (
                      <div className="space-y-2">
                        <p className="text-muted-foreground text-xs">{fileRequirement}</p>
                        <Button
                          variant="outline"
                          size="sm"
                          className="w-full"
                          onClick={() => handleFileSelect(name, selectType)}
                        >
                          Select {selectType === 'files' ? 'File' : 'Folder'}
                        </Button>
                      </div>
                    )}
                  </div>
                </div>
              </div>
            )
          })}
        </div>
      </div>
      <ImportInstructionsModal
        isOpen={!!selectedDataSource}
        onClose={() => setSelectedDataSource(null)}
        dataSource={selectedDataSource || ''}
      />
    </OnboardingLayout>
  )
}
