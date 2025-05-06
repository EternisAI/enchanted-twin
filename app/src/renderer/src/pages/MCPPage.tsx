import { useState } from 'react'
import { Trash2, ClipboardPaste } from 'lucide-react'
import { toast } from 'sonner'
import { useMutation, useQuery } from '@apollo/client'
import { Input } from '@renderer/components/ui/input'
import { Button } from '@renderer/components/ui/button'
import { Card } from '@renderer/components/ui/card'
import { Textarea } from '@renderer/components/ui/textarea'
import {
  ConnectMcpServerDocument,
  GetMcpServersDocument,
  McpServerType
} from '@renderer/graphql/generated/graphql'
import MCPPanel from '@renderer/components/oauth/MCPPanel'

export default function MCPPage() {
  const { refetch } = useQuery(GetMcpServersDocument)

  return (
    <div className="p-6 gap-6 w-full h-full flex flex-col overflow-y-auto">
      <h2 className="text-4xl mb-6">MCP Servers</h2>
      <div className="flex gap-6">
        <div>
          <MCPPanel allowRemove header={false} />
        </div>
        <MCPConnectionForm onConnect={refetch} />
      </div>
    </div>
  )
}

function MCPConnectionForm({ onConnect }: { onConnect: () => void }) {
  const [connectMcpServer, { loading }] = useMutation(ConnectMcpServerDocument, {
    onCompleted: () => {
      setName('')
      setCommand('')
      setArgumentsList([''])
      setEnvVars([{ key: '', value: '' }])
      setPastedText('')
      toast.success('MCP server connected')
      onConnect()
    },
    onError: (error: Error) => {
      console.error(error)
      toast.error('Failed to connect to MCP server', {
        description: error.message
      })
    }
  })

  const [name, setName] = useState('')
  const [command, setCommand] = useState('')
  const [argumentsList, setArgumentsList] = useState<string[]>([''])
  const [envVars, setEnvVars] = useState<{ key: string; value: string }[]>([{ key: '', value: '' }])
  const [pastedText, setPastedText] = useState('')

  const handleConnect = () => {
    connectMcpServer({
      variables: {
        input: {
          name,
          command,
          args: argumentsList,
          envs: envVars.filter((env) => env.key.trim() !== ''),
          type: McpServerType.Other
        }
      }
    })
  }

  const handleParsePaste = (): void => {
    try {
      const json = JSON.parse(pastedText)
      const serverEntries = Object.entries(json?.mcpServers || {})
      if (serverEntries.length === 0) {
        toast.error('No MCP servers found')
        return
      }

      const [firstName, config] = serverEntries[0]
      // eslint-disable-next-line @typescript-eslint/no-explicit-any
      const { command, args = [], env = {} } = config as any

      setName(firstName)
      setCommand(command)
      setArgumentsList(args)
      setEnvVars(Object.entries(env).map(([key, value]) => ({ key, value: value as string })))
      toast.success(`Prefilled with server "${firstName}"`)
    } catch (err) {
      console.error(err)
      toast.error('Invalid JSON format')
    }
  }

  return (
    <Card className="max-w-xl mx-auto p-6 space-y-6">
      <h1 className="text-xl font-bold">Connect to MCP Server</h1>
      <div className="flex flex-col gap-2">
        <label>Paste JSON Config</label>
        <Textarea
          placeholder="Paste MCP config JSON here..."
          value={pastedText}
          onChange={(e) => setPastedText(e.target.value)}
        />
        <Button onClick={handleParsePaste} className="mt-2" variant="secondary">
          <ClipboardPaste className="w-4 h-4 mr-2" />
          Prefill from JSON
        </Button>
      </div>

      <div className="space-y-2">
        <label htmlFor="name">Name</label>
        <Input
          id="name"
          placeholder="Server Name"
          value={name}
          onChange={(e) => setName(e.target.value)}
        />
      </div>

      <div className="space-y-2">
        <label htmlFor="command">Command</label>
        <Input
          id="command"
          placeholder="e.g. docker"
          value={command}
          onChange={(e) => setCommand(e.target.value)}
        />
      </div>

      <div className="space-y-2">
        <div className="flex justify-between items-center">
          <label>Arguments</label>
          <Button variant="link" onClick={() => setArgumentsList([...argumentsList, ''])}>
            + Add Argument
          </Button>
        </div>
        <div className="flex flex-col gap-2">
          {argumentsList.map((arg, idx) => (
            <div key={idx} className="flex gap-2 items-center">
              <Input
                value={arg}
                placeholder={`Argument ${idx + 1}`}
                onChange={(e) => {
                  const copy = [...argumentsList]
                  copy[idx] = e.target.value
                  setArgumentsList(copy)
                }}
              />
              <Button
                variant="ghost"
                size="icon"
                onClick={() => setArgumentsList(argumentsList.filter((_, i) => i !== idx))}
              >
                <Trash2 className="w-4 h-4 text-red-500" />
              </Button>
            </div>
          ))}
        </div>
      </div>

      <div className="space-y-2">
        <div className="flex justify-between items-center">
          <label>Environment Variables</label>
          <Button variant="link" onClick={() => setEnvVars([...envVars, { key: '', value: '' }])}>
            + Add Env Var
          </Button>
        </div>
        <div className="flex flex-col gap-2">
          {envVars.map((env, idx) => (
            <div key={idx} className="flex gap-2 items-center">
              <Input
                placeholder="Key"
                value={env.key}
                onChange={(e) => {
                  const copy = [...envVars]
                  copy[idx].key = e.target.value
                  setEnvVars(copy)
                }}
              />
              <Input
                placeholder="Value"
                value={env.value}
                onChange={(e) => {
                  const copy = [...envVars]
                  copy[idx].value = e.target.value
                  setEnvVars(copy)
                }}
              />
              <Button
                variant="ghost"
                size="icon"
                onClick={() => setEnvVars(envVars.filter((_, i) => i !== idx))}
              >
                <Trash2 className="w-4 h-4 text-red-500" />
              </Button>
            </div>
          ))}
        </div>
      </div>

      <Button className="w-full mt-4" onClick={handleConnect} disabled={loading}>
        {loading ? 'Connecting...' : 'Connect'}
      </Button>
    </Card>
  )
}
