import { useState } from 'react'
import { useMutation } from '@apollo/client'
import { ConnectMcpServerDocument, McpServerType } from '@renderer/graphql/generated/graphql'
import { Button } from '../ui/button'
import { Input } from '../ui/input'
import { Textarea } from '../ui/textarea'
import { Trash2, ClipboardPaste } from 'lucide-react'
import { toast } from 'sonner'

interface MCPConnectionFormProps {
  onSuccess: () => void
}

export default function MCPConnectionForm({ onSuccess }: MCPConnectionFormProps) {
  const [connectMcpServer, { loading }] = useMutation(ConnectMcpServerDocument, {
    onCompleted: () => {
      setName('')
      setCommand('')
      setArgumentsString('')
      setEnvVars([{ key: '', value: '' }])
      setPastedText('')
      toast.success('MCP server connected')
      onSuccess()
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
  const [argumentsString, setArgumentsString] = useState('')
  const [envVars, setEnvVars] = useState<{ key: string; value: string }[]>([{ key: '', value: '' }])
  const [pastedText, setPastedText] = useState('')

  const handleConnect = () => {
    connectMcpServer({
      variables: {
        input: {
          name,
          command,
          args: argumentsString.split(' ').filter((arg) => arg.trim() !== ''),
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
      setArgumentsString(Array.isArray(args) ? args.join(' ') : '')
      setEnvVars(Object.entries(env).map(([key, value]) => ({ key, value: value as string })))
      setPastedText('')
      toast.success(`Prefilled with server "${firstName}"`)
    } catch (err) {
      console.error(err)
      toast.error('Invalid JSON format')
    }
  }

  return (
    <div className="flex flex-col gap-6 ">
      <div className="flex flex-col gap-2">
        <label>Paste JSON Config</label>
        <Textarea
          placeholder="Paste MCP config JSON here..."
          className="min-h-30"
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
          {/* <Button variant="link" onClick={() => setArgumentsList([...argumentsList, ''])}>
            + Add Argument
          </Button> */}
        </div>
        <div className="flex flex-col gap-2">
          <Input
            value={argumentsString}
            placeholder="Enter arguments separated by spaces"
            onChange={(e) => setArgumentsString(e.target.value)}
          />
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
    </div>
  )
}
