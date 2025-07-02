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

enum MCPConnection {
  STDIO = 'stdio',
  STREAMBLE_HTTP = 'streamable-http'
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
  const [connection, setConnection] = useState<MCPConnection>(MCPConnection.STDIO)

  const handleConnect = () => {
    console.log(connection)
    if (connection === MCPConnection.STREAMBLE_HTTP) {
      connectMcpServer({
        variables: {
          input: {
            name: 'Streamble HTTP',
            command,
            type: McpServerType.Other
          }
        }
      })
      return
    }

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

      console.log(json.type)
      if (json.type === MCPConnection.STREAMBLE_HTTP) {
        setConnection(MCPConnection.STREAMBLE_HTTP)
        setCommand(json.url)
        toast.success(`Prefilled with streamable HTTP server "${json.url}"`)
        return
      } else {
        const serverEntries = Object.entries(json?.mcpServers || {})
        if (serverEntries.length === 0) {
          toast.error('No MCP servers found')
          return
        }

        const [firstName, config] = serverEntries[0]
        // eslint-disable-next-line @typescript-eslint/no-explicit-any
        const { command, args = [], env = {} } = config as any
        setConnection(MCPConnection.STDIO)
        setName(firstName)
        setCommand(command)
        setArgumentsString(Array.isArray(args) ? args.join(' ') : '')
        setEnvVars(Object.entries(env).map(([key, value]) => ({ key, value: value as string })))
        toast.success(`Prefilled with STDIO server "${firstName}"`)
      }

      setPastedText('')
    } catch (err) {
      console.error(err)
      toast.error('Invalid JSON format')
    }
  }

  return (
    <div className="flex flex-col gap-6 ">
      <div className="flex flex-col gap-1">
        <label htmlFor="connection">Connection Type</label>
        <select
          id="connection"
          value={connection}
          onChange={(e) => setConnection(e.target.value as MCPConnection)}
          className="h-10 w-full rounded-md border border-gray-300 bg-white pl-3 pr-8 py-2 text-sm focus:outline-none focus:ring-2 focus:ring-blue-500 focus:border-blue-500 disabled:cursor-not-allowed disabled:opacity-50 appearance-none bg-[url('data:image/svg+xml;base64,PHN2ZyB3aWR0aD0iMTIiIGhlaWdodD0iOCIgdmlld0JveD0iMCAwIDEyIDgiIGZpbGw9Im5vbmUiIHhtbG5zPSJodHRwOi8vd3d3LnczLm9yZy8yMDAwL3N2ZyI+CjxwYXRoIGQ9Ik0xIDFMNiA2TDExIDEiIHN0cm9rZT0iIzM3NDE1MSIgc3Ryb2tlLXdpZHRoPSIyIiBzdHJva2UtbGluZWNhcD0icm91bmQiIHN0cm9rZS1saW5lam9pbj0icm91bmQiLz4KPC9zdmc+')] bg-no-repeat bg-[center_right_0.75rem]"
        >
          <option value={MCPConnection.STDIO}>STDIO</option>
          <option value={MCPConnection.STREAMBLE_HTTP}>Streamble HTTP</option>
        </select>
      </div>
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

      {connection === 'stdio' ? (
        <StdioConnectionForm
          name={name}
          setName={setName}
          command={command}
          setCommand={setCommand}
          argumentsString={argumentsString}
          setArgumentsString={setArgumentsString}
          envVars={envVars}
          setEnvVars={setEnvVars}
        />
      ) : (
        <StreambleHttpConnectionForm command={command} setCommand={setCommand} />
      )}

      <Button className="w-full mt-4" onClick={handleConnect} disabled={loading}>
        {loading ? 'Connecting...' : 'Connect'}
      </Button>
    </div>
  )
}

type StdioConnectionFormProps = {
  name: string
  setName: (name: string) => void
  command: string
  setCommand: (command: string) => void
  argumentsString: string
  setArgumentsString: (argumentsString: string) => void
  envVars: { key: string; value: string }[]
  setEnvVars: (envVars: { key: string; value: string }[]) => void
}

function StdioConnectionForm({
  name,
  setName,
  command,
  setCommand,
  argumentsString,
  setArgumentsString,
  envVars,
  setEnvVars
}: StdioConnectionFormProps) {
  return (
    <div className="flex flex-col gap-6">
      <div className="flex flex-col gap-1">
        <label htmlFor="name">Name</label>
        <Input
          id="name"
          placeholder="Server Name"
          value={name}
          onChange={(e) => setName(e.target.value)}
        />
      </div>

      <div className="flex flex-col gap-1">
        <label htmlFor="command">Command</label>
        <Input
          id="command"
          placeholder="e.g. docker"
          value={command}
          onChange={(e) => setCommand(e.target.value)}
        />
      </div>

      <div className="flex flex-col gap-1">
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

      <div className="flex flex-col gap-1">
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
    </div>
  )
}

function StreambleHttpConnectionForm({
  command,
  setCommand
}: {
  command: string
  setCommand: (command: string) => void
}) {
  return (
    <div className="flex flex-col gap-1">
      <label htmlFor="url">URL</label>
      <Input
        id="url"
        placeholder="https://mcp.example.com"
        value={command}
        onChange={(e) => setCommand(e.target.value)}
      />
    </div>
  )
}
