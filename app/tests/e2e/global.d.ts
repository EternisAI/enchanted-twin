import { ChildProcess } from 'child_process'

declare global {
  // eslint-disable-next-line no-var
  var backendProcess: ChildProcess | null
}

export {}
