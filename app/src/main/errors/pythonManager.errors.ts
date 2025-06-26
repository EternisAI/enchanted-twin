export class PythonManagerError extends Error {
  constructor(message: string, public readonly code?: string) {
    super(message)
    this.name = 'PythonManagerError'
  }
}

export class EnvironmentError extends PythonManagerError {
  constructor(missingVars: string[]) {
    super(`Missing required environment variables: ${missingVars.join(', ')}`, 'ENV_VARS_MISSING')
    this.name = 'EnvironmentError'
  }
}

export class InstallationError extends PythonManagerError {
  constructor(message: string, public readonly step?: string) {
    super(message, 'INSTALLATION_FAILED')
    this.name = 'InstallationError'
  }
}

export class AgentError extends PythonManagerError {
  constructor(message: string) {
    super(message, 'AGENT_ERROR')
    this.name = 'AgentError'
  }
}

export class ProcessError extends PythonManagerError {
  constructor(message: string, public readonly exitCode?: number) {
    super(message, 'PROCESS_ERROR')
    this.name = 'ProcessError'
  }
}