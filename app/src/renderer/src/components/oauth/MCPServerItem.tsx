import { useMutation, MutationFunction } from '@apollo/client'
import {
  StartOAuthFlowDocument,
  CompleteOAuthFlowDocument,
  McpServerDefinition,
  StartOAuthFlowMutation,
  StartOAuthFlowMutationVariables,
  McpServerType
} from '@renderer/graphql/generated/graphql'
import { useEffect, useState } from 'react'
import { Switch } from '../ui/switch'
import { Input } from '../ui/input'
import { Button } from '../ui/button'
import { toast } from 'sonner'
import { Card } from '../ui/card'
import Google from '@renderer/assets/icons/google'
import Slack from '@renderer/assets/icons/slack'
import XformerlyTwitter from '@renderer/assets/icons/x'

const PROVIDER_MAP: Record<McpServerType, { provider: string; scope: string }> = {
  GOOGLE: {
    provider: 'google',
    scope:
      'openid email profile https://www.googleapis.com/auth/drive https://mail.google.com/ https://www.googleapis.com/auth/calendar'
  },
  SLACK: {
    provider: 'slack',
    scope: 'channels:read,groups:read,channels:history,groups:history,im:read,mpim:read,search:read'
  },
  TWITTER: { provider: 'twitter', scope: 'like.read tweet.read users.read offline.access' },
  OTHER: { provider: 'other', scope: '' }
}

interface EnvVarsEditorProps {
  envs: Array<{ key: string; value: string }>
  onSave: (values: Record<string, string>) => void
  onCancel: () => void
}

function EnvVarsEditor({ envs, onSave, onCancel }: EnvVarsEditorProps) {
  const [envValues, setEnvValues] = useState<Record<string, string>>({})

  useEffect(() => {
    const initialEnvs: Record<string, string> = {}
    envs?.forEach((env) => {
      if (env) initialEnvs[env.key] = env.value || ''
    })
    setEnvValues(initialEnvs)
  }, [envs])

  const handleEnvValueChange = (key: string, value: string) => {
    setEnvValues((prev) => ({
      ...prev,
      [key]: value
    }))
  }

  return (
    <div className="mt-4 space-y-3 bg-muted/30 p-4 rounded-md">
      <div className="text-sm font-medium mb-2">Environment Variables</div>
      {envs?.map(
        (env) =>
          env && (
            <div key={env.key} className="grid grid-cols-2 gap-3">
              <div className="bg-background border rounded-md px-3 py-2 text-sm text-muted-foreground">
                {env.key}
              </div>
              <Input
                value={envValues[env.key] || ''}
                onChange={(e) => handleEnvValueChange(env.key, e.target.value)}
                placeholder={`Value e.g. ${env.key.includes('TOKEN') ? '1234567890' : '1234567890'}`}
                className="bg-background"
              />
            </div>
          )
      )}
      <div className="flex gap-2 mt-3">
        <Button onClick={() => onSave(envValues)}>Save</Button>
        <Button variant="outline" onClick={onCancel}>
          Cancel
        </Button>
      </div>
    </div>
  )
}

const PROVIDER_ICON_MAP: Record<McpServerType, React.ReactNode> = {
  GOOGLE: <Google />,
  SLACK: <Slack />,
  TWITTER: <XformerlyTwitter />,
  OTHER: <></>
}

interface MCPServerItemProps {
  server: McpServerDefinition
  onConnect: () => void
}

export default function MCPServerItem({ server, onConnect }: MCPServerItemProps) {
  const [showEnvInputs, setShowEnvInputs] = useState(false)
  const [showFileInput] = useState(false)
  const [authStateId, setAuthStateId] = useState<string | null>(null)
  const [startOAuthFlow] = useMutation(StartOAuthFlowDocument)
  const [completeOAuthFlow] = useMutation(CompleteOAuthFlowDocument)

  async function handleOAuthFlow(
    serverType: string,
    startOAuthFlow: MutationFunction<StartOAuthFlowMutation, StartOAuthFlowMutationVariables>
  ) {
    try {
      const providerInfo = PROVIDER_MAP[serverType] || {
        provider: serverType.toLowerCase(),
        scope: ''
      }

      setAuthStateId(server.id)

      const { data } = await startOAuthFlow({
        variables: {
          provider: providerInfo.provider,
          scope: providerInfo.scope
        }
      })

      if (data?.startOAuthFlow) {
        const { authURL, redirectURI } = data.startOAuthFlow
        window.api.openOAuthUrl(authURL, redirectURI)
        return true
      }
    } catch (error) {
      console.error('Failed to start OAuth flow:', error)
      toast.error('Failed to connect to service')
      setAuthStateId(null)
    }
    return false
  }

  const handleEnableToolsToggle = async (enabled: boolean) => {
    if (server.type === 'OTHER') {
      setShowEnvInputs(enabled)
      return
    }

    if (enabled) {
      await handleOAuthFlow(server.type, startOAuthFlow)
    }
  }

  const handleSaveEnvValues = (values: Record<string, string>) => {
    console.log('Saving env values:', values)
    toast.success('Environment variables updated')
    setShowEnvInputs(false)
  }

  const handleFileSelect = (e: React.ChangeEvent<HTMLInputElement>) => {
    const file = e.target.files?.[0]
    if (file) {
      console.log('Selected file:', file.name)
      // Here you would implement file upload logic
    }
  }

  useEffect(() => {
    if (server.type === 'OTHER') return

    window.api.onOAuthCallback(async ({ code, state }) => {
      if (!authStateId) {
        console.log(`${server.name} is Skipping OAuth callback for different server`)
        return
      }

      try {
        const { data } = await completeOAuthFlow({ variables: { state, authCode: code } })
        console.log('OAuth completion data:', data)
        if (data?.completeOAuthFlow) {
          toast.success(`Connected successfully to ${data.completeOAuthFlow}!`)
          onConnect()
        }
      } catch (err) {
        console.error('OAuth completion failed:', err)
      } finally {
        setAuthStateId(null)
      }
    })
  }, [completeOAuthFlow, server.name, server.type, onConnect, authStateId])

  return (
    <Card className="p-4 w-[350px] max-w-full">
      <div className="font-semibold text-lg flex items-center justify-between">
        <div className="flex items-center gap-2">
          {PROVIDER_ICON_MAP[server.type]}
          {server.name}
        </div>
        {server.connected && (
          <span className="text-xs bg-green-500/20 text-green-600 px-2 py-0.5 rounded-full font-medium">
            Connected
          </span>
        )}
      </div>

      <div className="flex flex-col gap-2">
        {/* <div className="flex items-center justify-between">
          <div className="space-y-1">
            <label htmlFor={`import-${server.id}`} className="font-medium">
              Import historical data
            </label>
            <div className="text-xs text-muted-foreground">
              <button className="text-primary underline">How?</button>
            </div>
          </div>
          <Switch
            id={`import-${server.id}`}
            checked={showFileInput}
            onCheckedChange={setShowFileInput}
          />
        </div> */}

        {showFileInput && (
          <div className="border border-dashed rounded-md p-3 bg-muted/20">
            <div className="flex flex-col gap-2">
              <label htmlFor={`file-${server.id}`} className="text-sm text-muted-foreground">
                Select file
              </label>
              <Input
                id={`file-${server.id}`}
                type="file"
                onChange={handleFileSelect}
                className="text-sm"
              />
            </div>
          </div>
        )}

        <div className="flex items-center justify-between gap-4">
          <label htmlFor={`sync-${server.id}`} className="font-medium">
            Sync data automatically
          </label>
          <Switch id={`sync-${server.id}`} checked disabled />
        </div>

        <div className="flex items-center justify-between gap-4">
          <label htmlFor={`enable-${server.id}`} className="font-medium">
            Enable LLM tools
          </label>
          <Switch
            id={`enable-${server.id}`}
            checked={server.enabled || showEnvInputs}
            onCheckedChange={handleEnableToolsToggle}
          />
        </div>

        {showEnvInputs && server.type === 'OTHER' && (
          <EnvVarsEditor
            envs={server.envs?.filter(Boolean) || []}
            onSave={handleSaveEnvValues}
            onCancel={() => setShowEnvInputs(false)}
          />
        )}
      </div>
    </Card>
  )
}
