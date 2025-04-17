import { useOnboardingStore, DataSourceType } from '@renderer/lib/stores/onboarding'
import { OnboardingLayout } from './OnboardingLayout'
import { useMutation } from '@apollo/client'
import { gql } from '@apollo/client'
import { Folder } from 'lucide-react'

const ADD_DATA_SOURCE = gql`
  mutation AddDataSource($input: AddDataSourceInput!) {
    addDataSource(input: $input)
  }
`

const DATA_SOURCES: {
  type: DataSourceType
  label: string
  description: string
  selectType: 'directory' | 'files'
}[] = [
  {
    type: 'WhatsApp',
    label: 'WhatsApp',
    description: 'Import your WhatsApp chat history',
    selectType: 'files'
  },
  {
    type: 'Telegram',
    label: 'Telegram',
    description: 'Import your Telegram messages and media',
    selectType: 'directory'
  },
  {
    type: 'Slack',
    label: 'Slack',
    description: 'Import your Slack workspace data',
    selectType: 'directory'
  },
  {
    type: 'Gmail',
    label: 'Gmail',
    description: 'Import your Gmail emails and attachments',
    selectType: 'files'
  }
]

export function ImportDataStep() {
  const { addDataSource, dataSources } = useOnboardingStore()
  const [addDataSourceMutation] = useMutation(ADD_DATA_SOURCE)

  const handleFileSelect = async (type: DataSourceType, selectType: 'directory' | 'files') => {
    try {
      // Open file dialog based on selection type
      const result = await (selectType === 'directory'
        ? window.api.selectDirectory()
        : window.api.selectFiles())

      if (result.canceled) return

      // Add data source to store
      addDataSource({
        type,
        path: result.filePaths[0],
        status: 'pending'
      })

      // Call GraphQL mutation to add data source
      await addDataSourceMutation({
        variables: {
          input: {
            dataSourceName: type,
            path: result.filePaths[0],
            username: 'default' // TODO: Get from profile
          }
        }
      })
    } catch (error) {
      console.error('Error selecting files:', error)
      // Update store with error status
      addDataSource({
        type,
        status: 'error'
      })
    }
  }

  return (
    <OnboardingLayout
      title="Import Your Data"
      subtitle="Select the data sources you'd like to import. You can always add more later."
    >
      <div className="space-y-6">
        <div className="grid grid-cols-1 md:grid-cols-2 gap-4">
          {DATA_SOURCES.map(({ type, label, description, selectType }) => {
            const isSelected = dataSources.some((ds) => ds.type === type)
            const source = dataSources.find((ds) => ds.type === type)

            return (
              <button
                key={type}
                onClick={() => handleFileSelect(type, selectType)}
                className={`p-4 rounded-lg border ${
                  isSelected ? 'border-primary bg-primary/5' : 'border-border'
                } hover:border-primary transition-colors text-left`}
              >
                <div className="space-y-2">
                  <div className="flex items-center justify-between">
                    <h3 className="font-medium">{label}</h3>
                    <Folder className="h-4 w-4 text-muted-foreground" />
                  </div>
                  <p className="text-sm text-muted-foreground">{description}</p>
                  {isSelected && (
                    <div className="text-sm">
                      <p className="text-primary">✓ Selected</p>
                      <p className="text-muted-foreground text-xs truncate">{source?.path}</p>
                    </div>
                  )}
                </div>
              </button>
            )
          })}
        </div>
      </div>
    </OnboardingLayout>
  )
}
