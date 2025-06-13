import { app } from 'electron'
import path from 'node:path'
import fs, { constants as fsc } from 'node:fs'
import { spawn, SpawnOptions } from 'node:child_process'
import log from 'electron-log/main'

export interface DependencyProgress {
  dependency: string
  progress: number
  status: string
  error?: string
}

type RunOptions = SpawnOptions & { label: string }

/* ─────────────────────────────────────────────────────────────────────────── */

export class LiveKitAgentBootstrap {
  private readonly USER_DIR = app.getPath('userData')
  private readonly USER_BIN = path.join(this.USER_DIR, 'bin')
  private readonly UV_PATH = path.join(
    this.USER_BIN,
    process.platform === 'win32' ? 'uv.exe' : 'uv'
  )

  private readonly LIVEKIT_DIR = path.join(this.USER_DIR, 'dependencies', 'livekit-agent')
  private readonly VENV_DIR = path.join(this.LIVEKIT_DIR, '.venv')
  private readonly greetingFile = path.join(this.LIVEKIT_DIR, 'greeting.txt')
  private readonly onboardingStateFile = path.join(this.LIVEKIT_DIR, 'onboarding_state.txt')

  /** absolute path to the python executable in the venv */
  private pythonBin(): string {
    const sub =
      process.platform === 'win32' ? path.join('Scripts', 'python.exe') : path.join('bin', 'python')
    return path.join(this.VENV_DIR, sub)
  }

  private agentProc: import('child_process').ChildProcess | null = null
  private onProgress?: (data: DependencyProgress) => void
  private onSessionReady?: (ready: boolean) => void
  private latestProgress: DependencyProgress = {
    dependency: 'LiveKit Agent',
    progress: 0,
    status: 'Not started'
  }

  constructor(
    onProgress?: (data: DependencyProgress) => void,
    onSessionReady?: (ready: boolean) => void
  ) {
    this.onProgress = onProgress
    this.onSessionReady = onSessionReady
  }

  getLatestProgress() {
    return this.latestProgress
  }

  /* ── helpers ────────────────────────────────────────────────────────────── */
  private async exists(p: string, mode = fsc.F_OK) {
    try {
      await fs.promises.access(p, mode)
      return true
    } catch {
      return false
    }
  }

  private uvEnv(venv: string) {
    const bin = process.platform === 'win32' ? path.join(venv, 'Scripts') : path.join(venv, 'bin')
    return { ...process.env, VIRTUAL_ENV: venv, PATH: `${bin}${path.delimiter}${process.env.PATH}` }
  }

  private run(cmd: string, args: readonly string[], opts: RunOptions) {
    return new Promise<void>((resolve, reject) => {
      log.info(`[LiveKit] [${opts.label}] → ${cmd} ${args.join(' ')}`)
      const p = spawn(cmd, args, { ...opts, stdio: 'pipe' })

      p.stdout?.on('data', (data) => {
        const output = data.toString().trim()
        if (output) {
          log.info(`[LiveKit] [${opts.label}] ${output}`)
        }
      })

      p.stderr?.on('data', (data) => {
        const output = data.toString().trim()
        if (output) {
          log.error(`[LiveKit] [${opts.label}] ${output}`)
        }
      })

      p.once('error', reject)
      p.once('exit', (c) => (c === 0 ? resolve() : reject(new Error(`${opts.label} exit ${c}`))))
    })
  }

  /* ── install steps ──────────────────────────────────────────────────────── */
  private async ensureUv() {
    if (await this.exists(this.UV_PATH, fsc.X_OK)) {
      log.info('[LiveKit] UV already installed')
      return
    }
    log.info('[LiveKit] Installing UV package manager')
    await fs.promises.mkdir(this.USER_BIN, { recursive: true })
    await this.run('sh', ['-c', 'curl -LsSf https://astral.sh/uv/install.sh | sh'], {
      label: 'uv-install',
      env: { ...process.env, UV_INSTALL_DIR: this.USER_BIN }
    })
  }

  private async ensurePython312() {
    await this.run(this.UV_PATH, ['python', 'install', '3.12', '--quiet'], { label: 'py312' })
  }

  private async ensureAgentFiles() {
    log.info('[LiveKit] Setting up agent files')
    await fs.promises.mkdir(this.LIVEKIT_DIR, { recursive: true })

    const agentFile = path.join(this.LIVEKIT_DIR, 'agent.py')
    const requirementsFile = path.join(this.LIVEKIT_DIR, 'requirements.txt')

    // Embed agent files as strings for reliable deployment
    const agentCode = `import asyncio
import logging
import os
import sys
import requests
import aiohttp


from livekit.agents import (
    Agent,
    AgentSession,
    JobContext,
    RunContext,
    WorkerOptions,
    cli,
    function_tool,
)
from livekit.plugins import openai, silero
from livekit.plugins.openai.utils import to_chat_ctx
from livekit.agents import APIConnectionError, llm

# Configure logging with more detailed output
logging.basicConfig(
    level=logging.INFO,
    format='%(asctime)s - %(name)s - %(levelname)s - %(message)s'
)
logger = logging.getLogger(__name__)


# Patch termios for non-TTY environments before importing livekit
if os.getenv('LIVEKIT_DISABLE_TERMIOS'):

    logger.info("Starting termios patching")
    import termios
    original_tcgetattr = termios.tcgetattr
    original_tcsetattr = termios.tcsetattr
    
    def patched_tcgetattr(fd):
        # Check if fd is actually a TTY before attempting termios operations
        if not os.isatty(fd):
            # Return properly formatted dummy terminal attributes for non-TTY
            # Structure: [iflag, oflag, cflag, lflag, ispeed, ospeed, cc]
            # cc needs to be a list with at least 32 elements for VMIN/VTIME access
            cc = [0] * 32  # Create list with 32 zero bytes
            return [0, 0, 0, 0, 0, 0, cc]
        try:
            return original_tcgetattr(fd)
        except OSError:
            # Fallback for edge cases - also needs proper structure
            cc = [0] * 32
            return [0, 0, 0, 0, 0, 0, cc]
    
    def patched_tcsetattr(fd, when, attrs):
        # Check if fd is actually a TTY before attempting termios operations
        if not os.isatty(fd):
            # Silently ignore for non-TTY
            return None
        try:
            return original_tcsetattr(fd, when, attrs)
        except OSError:
            # Silently ignore for non-TTY
            return None
    
    termios.tcgetattr = patched_tcgetattr
    termios.tcsetattr = patched_tcsetattr

    logger.info("Termios patching enabled for non-TTY environment")

    

# Verify required environment variables
required_env_vars = [
    "OPENAI_API_KEY",
    "CHAT_ID",
    "TINFOIL_API_KEY",
    "TINFOIL_AUDIO_URL",
    "TINFOIL_STT_MODEL",
    "TINFOIL_TTS_MODEL",
    "SEND_MESSAGE_URL"
]

missing_vars = [var for var in required_env_vars if not os.getenv(var)]
if missing_vars:
    logger.error(f"Missing required environment variables: {', '.join(missing_vars)}")
    logger.error("Please create a .env file with the required API keys")
    sys.exit(1)

TTS_API_KEY = os.getenv("TINFOIL_API_KEY")
TTS_URL = os.getenv("TINFOIL_AUDIO_URL")
TTS_MODEL = os.getenv("TINFOIL_TTS_MODEL")

STT_API_KEY = os.getenv("TINFOIL_API_KEY")
STT_URL = os.getenv("TINFOIL_AUDIO_URL")
STT_MODEL = os.getenv("TINFOIL_STT_MODEL")
CHAT_ID = os.getenv("CHAT_ID")
SEND_MESSAGE_URL = os.getenv("SEND_MESSAGE_URL")
GREETING = open(os.path.join(os.path.dirname(__file__), 'greeting.txt')).read()

def get_onboarding_state():
    """Read the current onboarding state from file"""
    try:
        with open(os.path.join(os.path.dirname(__file__), 'onboarding_state.txt'), 'r') as f:
            return f.read().strip().lower() == "true"
    except FileNotFoundError:
        return False

def get_chat_history(chat_id: str):
    url = SEND_MESSAGE_URL
    
    query = """
    query getChatHistory($id: ID!) {
        getChat(id: $id) {
            messages {
                role
                text
            }
        }
    }
    """
    
    variables = { "id": chat_id}
    
    resp = requests.post(url, json={"query": query, "variables": variables})
    
    if resp.status_code == 200:
        body = resp.json()
        history = body["data"]["getChat"]["messages"]  
        history = [item for item in history if item["role"].lower() != "system"]
        return history
    
    return []


async def send_message(context, chat_id: str):

    url = SEND_MESSAGE_URL
    is_onboarding = get_onboarding_state()

    query = """
   mutation sendmsg($chatId: ID!, $context: [MessageInput!]!, $isOnboarding: Boolean!) {
    processMessageHistory(
      chatId: $chatId        
      messages: $context    
      isOnboarding: $isOnboarding 
    ) {
        id
        text
        createdAt
      }
    }
    """
    
    variables = { "chatId": chat_id, "context": context, "isOnboarding": is_onboarding}

    async with aiohttp.ClientSession() as session:
        async with session.post(url, json={"query": query, "variables": variables}) as resp:
            if resp.status == 200:
                body = await resp.json()
                print(body)
                return body["data"]["processMessageHistory"]["text"]
    return None
    
    
class APIConnectOptions:
    max_retry: int = 3
    retry_interval: float = 2.0
    timeout: float = 10.0
    
    def __post_init__(self) -> None:
        if self.max_retry < 0:
            raise ValueError("max_retry must be greater than or equal to 0")

        if self.retry_interval < 0:
            raise ValueError("retry_interval must be greater than or equal to 0")

        if self.timeout < 0:
            raise ValueError("timeout must be greater than or equal to 0")

    def _interval_for_retry(self, num_retries: int) -> float:
        if num_retries == 0:
            return 0.1
        return self.retry_interval


class LLMStream(llm.LLMStream):
    def __init__(self, llm, chat_ctx: llm.ChatContext, chat_id: str, chat_history: list) -> None:
        super().__init__(llm, chat_ctx=chat_ctx, tools=None, conn_options=None)
        self._chat_id = chat_id
        self._llm = llm
        self._conn_options = APIConnectOptions()
        self._chat_history = chat_history

    async def _run(self) -> None:
        # current function call that we're waiting for full completion (args are streamed)
        # (defined inside the _run method to make sure the state is reset for each run/attempt)
        self._oai_stream = None
        self._tool_call_id: str | None = None
        self._fnc_name: str | None = None
        self._fnc_raw_arguments: str | None = None
        self._tool_index: int | None = None
        retryable = True

        try:
            context = to_chat_ctx(self._chat_ctx, "1")
            context = [item for item in context if (item["role"] != "system" and item["content"]!="")]
            context = [{'role': item['role'].upper(), 'text': item['content']} for item in context]
            context = self._chat_history + context
            received_message = await send_message(context, self._chat_id)

            if received_message is not None:
              chunk = llm.ChatChunk(id="1",
                                    delta=llm.ChoiceDelta(content=received_message, role="assistant"))
              self._event_ch.send_nowait(chunk)
            
        except Exception as e:
            raise APIConnectionError(retryable=retryable) from e
        

class LLM(llm.LLM):
    def __init__(self, chat_id: str) -> None:
        super().__init__()
        self._chat_id = chat_id
        self._chat_history = get_chat_history(chat_id)
        

    def chat( self, *,
        chat_ctx: llm.ChatContext,
        tools = None,
        conn_options = None,
        parallel_tool_calls = None,
        tool_choice = False,
        response_format = False,
        extra_kwargs = False):
        return LLMStream(llm=self, chat_ctx=chat_ctx, chat_id=self._chat_id, chat_history=self._chat_history)


async def entrypoint(ctx: JobContext):
    """Main entry point for the LiveKit agent"""
    
    logger.info("Starting LiveKit agent entrypoint")
    
    # Connect to the room
    await ctx.connect()
    logger.info(f"Connected to LiveKit room: {ctx.room.name}")
    
    # Create the agent with instructions
    agent = Agent(instructions="")
    logger.info("Agent created with instructions")
    
    # Create the agent session with voice components
    logger.info("Initializing voice components...")
    
    # Initialize components with better configuration
    vad = silero.VAD.load()
    stt = openai.STT(base_url=STT_URL, model=STT_MODEL, api_key=STT_API_KEY)
    llm = LLM(chat_id=os.getenv("CHAT_ID"))
    tts = openai.TTS(base_url=TTS_URL, model=TTS_MODEL, api_key=TTS_API_KEY, voice="af_v0bella")
    
    logger.info("Voice components initialized")
    
    session = AgentSession( vad=vad, stt=stt, llm=llm, tts=tts)
    
    # Start the session
    logger.info("Starting agent session...")
    await session.start(agent=agent, room=ctx.room)
    logger.info("Agent session started successfully")
    
    # Wait a moment for the session to fully initialize
    if GREETING:
        await session.say(GREETING)

    logger.info("Agent is now active and ready for conversation")

if __name__ == "__main__":
    logger.info("Starting LiveKit agent in console mode")
    cli.run_app(WorkerOptions(entrypoint_fnc=entrypoint))`

    const requirementsContent = `livekit==1.0.8
livekit-agents==1.0.23
livekit-plugins-openai==1.0.23
livekit-plugins-deepgram==1.0.23
livekit-plugins-silero==1.0.23
python-dotenv>=1.0.0
requests`

    try {
      await fs.promises.writeFile(agentFile, agentCode)
      await fs.promises.writeFile(requirementsFile, requirementsContent)
      log.info('[LiveKit] Agent files created successfully')
    } catch (error) {
      log.error('[LiveKit] Failed to setup agent files:', error)
      throw error
    }
  }

  private async ensureVenv() {
    const cfg = path.join(this.VENV_DIR, 'pyvenv.cfg')

    let venvIs312 = false
    if (await this.exists(cfg)) {
      const txt = await fs.promises.readFile(cfg, 'utf8')
      venvIs312 = /^version = 3\.12\./m.test(txt)
    }

    if (venvIs312) {
      log.info('[LiveKit] Virtual environment already exists with Python 3.12')
      return
    }

    log.info('[LiveKit] Creating Python 3.12 virtual environment')
    if (await this.exists(this.VENV_DIR)) {
      log.info('[LiveKit] Removing existing virtual environment')
      await fs.promises.rm(this.VENV_DIR, { recursive: true, force: true }).catch(() => {})
    }

    await this.run(this.UV_PATH, ['venv', '--python', '3.12', this.VENV_DIR], {
      label: 'uv-venv'
    })
  }

  private async ensureDeps() {
    const stamp = path.join(this.VENV_DIR, '.livekit-installed')
    if (await this.exists(stamp)) {
      log.info('[LiveKit] Dependencies already installed')
      return
    }

    log.info('[LiveKit] Installing Python dependencies using uv pip install')
    await this.run(this.UV_PATH, ['pip', 'install', '-r', 'requirements.txt'], {
      cwd: this.LIVEKIT_DIR,
      env: this.uvEnv(this.VENV_DIR),
      label: 'uv-pip'
    })

    await fs.promises.writeFile(stamp, '')
    log.info('[LiveKit] Dependencies installation completed')
  }

  async setup() {
    log.info('[LiveKit] Starting LiveKit Agent setup process')
    try {
      this.onProgress?.({
        dependency: 'LiveKit Agent',
        progress: 10,
        status: 'Setting up dependency manager'
      })
      this.latestProgress = {
        dependency: 'LiveKit Agent',
        progress: 10,
        status: 'Setting up dependency manager'
      }
      await this.ensureUv()

      this.onProgress?.({ dependency: 'LiveKit Agent', progress: 20, status: 'Installing Python' })
      this.latestProgress = {
        dependency: 'LiveKit Agent',
        progress: 20,
        status: 'Installing Python'
      }
      await this.ensurePython312()

      this.onProgress?.({
        dependency: 'LiveKit Agent',
        progress: 40,
        status: 'Setting up agent files'
      })
      this.latestProgress = {
        dependency: 'LiveKit Agent',
        progress: 40,
        status: 'Setting up agent files'
      }
      await this.ensureAgentFiles()

      this.onProgress?.({
        dependency: 'LiveKit Agent',
        progress: 60,
        status: 'Creating virtual environment'
      })
      this.latestProgress = {
        dependency: 'LiveKit Agent',
        progress: 60,
        status: 'Creating virtual environment'
      }
      await this.ensureVenv()

      this.onProgress?.({
        dependency: 'LiveKit Agent',
        progress: 80,
        status: 'Installing dependencies'
      })
      this.latestProgress = {
        dependency: 'LiveKit Agent',
        progress: 80,
        status: 'Installing dependencies'
      }
      await this.ensureDeps()

      this.onProgress?.({ dependency: 'LiveKit Agent', progress: 100, status: 'Ready' })
      this.latestProgress = { dependency: 'LiveKit Agent', progress: 100, status: 'Ready' }

      log.info('[LiveKit] LiveKit Agent setup completed successfully')
    } catch (e) {
      const error = e instanceof Error ? e.message : 'Unknown error occurred'
      log.error('[LiveKit] LiveKit Agent setup failed', e)
      this.latestProgress = {
        dependency: 'LiveKit Agent',
        progress: this.latestProgress.progress,
        status: 'Failed',
        error
      }
      this.onProgress?.({ dependency: 'LiveKit Agent', progress: 0, status: 'Failed', error })
      throw e
    }
  }

  async startAgent(chatId: string, isOnboarding: boolean = false) {
    if (this.agentProc) {
      log.warn('[LiveKit] Agent is already running')
      return
    }

    log.info('[LiveKit] Starting LiveKit agent', isOnboarding)

    // Note: Room connection is handled by the LiveKit agent framework via ctx.connect()

    // Check for required environment variables before starting
    if (!process.env.OPENAI_API_KEY) {
      throw new Error(
        'OPENAI_API_KEY environment variable is required but not set. Please add it to your environment or .env file.'
      )
    }

    let greeting = ``
    if (isOnboarding) {
      greeting = `Hello there! Welcome to Enchanted, what is your name?`
    }

    await fs.promises.writeFile(this.greetingFile, greeting)
    await fs.promises.writeFile(this.onboardingStateFile, isOnboarding.toString())

    isOnboarding && console.log('isOnboarding starting', isOnboarding)

    // Start the agent using the virtual environment Python
    this.agentProc = spawn(this.pythonBin(), ['agent.py', 'console'], {
      cwd: this.LIVEKIT_DIR,
      env: {
        ...process.env,
        OPENAI_API_KEY: process.env.OPENAI_API_KEY,
        CHAT_ID: chatId,

        TINFOIL_API_KEY: process.env.TINFOIL_API_KEY,
        TINFOIL_AUDIO_URL: process.env.TINFOIL_AUDIO_URL,
        TINFOIL_STT_MODEL: process.env.TINFOIL_STT_MODEL,
        TINFOIL_TTS_MODEL: process.env.TINFOIL_TTS_MODEL,
        SEND_MESSAGE_URL: `http://localhost:44999/query`,
        TERM: 'dumb', // Use dumb terminal to avoid TTY features
        PYTHONUNBUFFERED: '1', // Ensure immediate output
        NO_COLOR: '1', // Disable color codes
        LIVEKIT_DISABLE_TERMIOS: '1' // Custom flag to disable termios
      },
      stdio: 'pipe' // Use pipe for logging
    })

    this.agentProc.stdout?.on('data', (data) => {
      const output = data.toString().trim()
      if (output) {
        log.info(`[LiveKit] [agent] ${output}`)

        // Check for session ready indicators
        if (output.includes('Agent session started successfully')) {
          this.onSessionReady?.(true)
        }
      }
    })

    this.agentProc.stderr?.on('data', (data) => {
      const output = data.toString().trim()
      if (output) {
        log.error(`[LiveKit] [agent] ${output}`)
      }
    })

    this.agentProc.on('exit', (code) => {
      log.info(`[LiveKit] Agent exited with code ${code}`)
      this.onSessionReady?.(false)
      this.agentProc = null
    })

    log.info('[LiveKit] Agent started successfully')
  }

  async stopAgent() {
    if (!this.agentProc) {
      log.warn('[LiveKit] No agent process to stop')
      return
    }

    log.info('[LiveKit] Stopping LiveKit agent')
    this.onSessionReady?.(false)
    this.agentProc.kill('SIGTERM')

    // Give it a moment to exit gracefully
    await new Promise((resolve) => setTimeout(resolve, 2000))

    if (this.agentProc) {
      log.info('[LiveKit] Force killing agent process')
      this.agentProc.kill('SIGKILL')
    }

    this.agentProc = null
    log.info('[LiveKit] Agent stopped')

    // Clear the greeting and onboarding state files
    await fs.promises.writeFile(this.greetingFile, '')
    await fs.promises.writeFile(this.onboardingStateFile, 'false')
  }

  isAgentRunning(): boolean {
    return this.agentProc !== null
  }

  async updateOnboardingState(isOnboarding: boolean): Promise<void> {
    await fs.promises.writeFile(this.onboardingStateFile, isOnboarding.toString())
    log.info(`[LiveKit] Updated onboarding state to: ${isOnboarding}`)
  }

  async cleanup() {
    await this.stopAgent()
  }
}

/* ─────────────────────────────────────────────────────────────────────────── */

// export class KokoroBootstrap {
//   private readonly USER_DIR = app.getPath('userData')
//   private readonly USER_BIN = path.join(this.USER_DIR, 'bin')
//   private readonly UV_PATH = path.join(
//     this.USER_BIN,
//     process.platform === 'win32' ? 'uv.exe' : 'uv'
//   )

//   private readonly KOKORO_DIR = path.join(this.USER_DIR, 'dependencies', 'kokoro')
//   private readonly VENV_DIR = path.join(this.KOKORO_DIR, '.venv')

//   private readonly ZIP_URL =
//     'https://github.com/remsky/Kokoro-FastAPI/archive/refs/heads/master.zip'

//   /** absolute path to the python executable in the venv */
//   private pythonBin(): string {
//     const sub =
//       process.platform === 'win32' ? path.join('Scripts', 'python.exe') : path.join('bin', 'python')
//     return path.join(this.VENV_DIR, sub)
//   }

//   private kokoroProc: import('child_process').ChildProcess | null = null
//   private onProgress?: (data: DependencyProgress) => void
//   private latestProgress: DependencyProgress = {
//     dependency: 'Kokoro',
//     progress: 0,
//     status: 'Not started'
//   }

//   constructor(onProgress?: (data: DependencyProgress) => void) {
//     this.onProgress = onProgress
//   }

//   getLatestProgress() {
//     return this.latestProgress
//   }

//   /* ── helpers ────────────────────────────────────────────────────────────── */
//   private async exists(p: string, mode = fsc.F_OK) {
//     try {
//       await fs.promises.access(p, mode)
//       return true
//     } catch {
//       return false
//     }
//   }

//   private uvEnv(venv: string) {
//     const bin = process.platform === 'win32' ? path.join(venv, 'Scripts') : path.join(venv, 'bin')
//     return { ...process.env, VIRTUAL_ENV: venv, PATH: `${bin}${path.delimiter}${process.env.PATH}` }
//   }

//   private run(cmd: string, args: readonly string[], opts: RunOptions) {
//     return new Promise<void>((resolve, reject) => {
//       log.info(`[Kokoro] [${opts.label}] → ${cmd} ${args.join(' ')}`)
//       const p = spawn(cmd, args, { ...opts, stdio: 'pipe' })

//       p.stdout?.on('data', (data) => {
//         const output = data.toString().trim()
//         if (output) {
//           log.info(`[Kokoro] [${opts.label}] ${output}`)
//         }
//       })

//       p.stderr?.on('data', (data) => {
//         const output = data.toString().trim()
//         if (output) {
//           log.error(`[Kokoro] [${opts.label}] ${output}`)
//         }
//       })

//       p.once('error', reject)
//       p.once('exit', (c) => (c === 0 ? resolve() : reject(new Error(`${opts.label} exit ${c}`))))
//     })
//   }

//   private download(url: string, dest: string, redirects = 5): Promise<void> {
//     return new Promise((resolve, reject) => {
//       const req = (u: string, r: number) =>
//         https
//           .get(u, { headers: { 'User-Agent': 'kokoro-installer' } }, (res) => {
//             const { statusCode, headers } = res
//             if (statusCode && statusCode >= 300 && statusCode < 400 && headers.location) {
//               return r ? req(headers.location, r - 1) : reject(new Error('Too many redirects'))
//             }
//             if (statusCode !== 200) {
//               res.resume()
//               return reject(new Error(`HTTP ${statusCode} on ${u}`))
//             }
//             pipeline(res, fs.createWriteStream(dest)).then(resolve).catch(reject)
//           })
//           .on('error', reject)
//       req(url, redirects)
//     })
//   }

//   private async extractZipFlattened(src: string, dest: string) {
//     const zip = await unzipper.Open.file(src)
//     for (const e of zip.files) {
//       if (e.type === 'Directory') continue
//       const rel = e.path.split(/[/\\]/).slice(1)
//       if (!rel.length) continue
//       const out = path.join(dest, ...rel)
//       await fs.promises.mkdir(path.dirname(out), { recursive: true })
//       await new Promise<void>((res, rej) =>
//         e.stream().pipe(fs.createWriteStream(out)).on('finish', res).on('error', rej)
//       )
//     }
//   }

//   /* ── install steps ──────────────────────────────────────────────────────── */
//   private async ensureUv() {
//     if (await this.exists(this.UV_PATH, fsc.X_OK)) {
//       log.info('[Kokoro] UV already installed')
//       return
//     }
//     log.info('[Kokoro] Installing UV package manager')
//     await fs.promises.mkdir(this.USER_BIN, { recursive: true })
//     await this.run('sh', ['-c', 'curl -LsSf https://astral.sh/uv/install.sh | sh'], {
//       label: 'uv-install',
//       env: { ...process.env, UV_INSTALL_DIR: this.USER_BIN }
//     })
//   }

//   private async ensurePython312() {
//     await this.run(this.UV_PATH, ['python', 'install', '3.12', '--quiet'], { label: 'py312' })
//   }

//   private async ensureRepo() {
//     if (await this.exists(path.join(this.KOKORO_DIR, 'api'))) {
//       log.info('[Kokoro] Repository already exists')
//       return
//     }
//     log.info('[Kokoro] Downloading Kokoro repository')
//     await fs.promises.rm(this.KOKORO_DIR, { recursive: true, force: true }).catch(() => {})
//     await fs.promises.mkdir(this.KOKORO_DIR, { recursive: true })

//     const zipTmp = path.join(tmpdir(), `kokoro-${Date.now()}.zip`)
//     try {
//       log.info('[Kokoro] Downloading repository archive')
//       await this.download(this.ZIP_URL, zipTmp)
//       log.info('[Kokoro] Extracting repository archive')
//       await this.extractZipFlattened(zipTmp, this.KOKORO_DIR)
//     } finally {
//       await fs.promises.unlink(zipTmp).catch(() => {})
//     }
//   }

//   private async ensureVenv() {
//     const cfg = path.join(this.VENV_DIR, 'pyvenv.cfg')

//     let venvIs312 = false
//     if (await this.exists(cfg)) {
//       const txt = await fs.promises.readFile(cfg, 'utf8')
//       venvIs312 = /^version = 3\.12\./m.test(txt)
//     }

//     if (venvIs312) {
//       log.info('[Kokoro] Virtual environment already exists with Python 3.12')
//       return
//     }

//     log.info('[Kokoro] Creating Python 3.12 virtual environment')
//     if (await this.exists(this.VENV_DIR)) {
//       log.info('[Kokoro] Removing existing virtual environment')
//       await fs.promises.rm(this.VENV_DIR, { recursive: true, force: true }).catch(() => {})
//     }

//     await this.run(this.UV_PATH, ['venv', '--python', '3.12', this.VENV_DIR], {
//       label: 'uv-venv'
//     })
//   }

//   private async ensureDeps() {
//     const stamp = path.join(this.VENV_DIR, '.kokoro-installed')
//     if (await this.exists(stamp)) {
//       log.info('[Kokoro] Dependencies already installed')
//       return
//     }

//     log.info('[Kokoro] Installing Python dependencies')
//     await this.run(this.UV_PATH, ['pip', 'install', '-e', '.'], {
//       cwd: this.KOKORO_DIR,
//       env: this.uvEnv(this.VENV_DIR),
//       label: 'uv-pip'
//     })

//     await fs.promises.writeFile(stamp, '')
//     log.info('[Kokoro] Dependencies installation completed')
//   }

//   private async startTts() {
//     /* print interpreter info */
//     await this.run(
//       this.pythonBin(),
//       [
//         '-c',
//         'import sys;print("\\n=== Kokoro Runtime ===");' +
//           'print("Python:",sys.version);print("Exec:",sys.executable);print("====================\\n")'
//       ],
//       { cwd: this.KOKORO_DIR, env: this.uvEnv(this.VENV_DIR), label: 'python-info' }
//     )

//     const env = {
//       ...this.uvEnv(this.VENV_DIR),
//       USE_GPU: 'true',
//       USE_ONNX: 'false',
//       PYTHONPATH: `${this.KOKORO_DIR}${path.delimiter}${path.join(this.KOKORO_DIR, 'api')}`,
//       MODEL_DIR: 'src/models',
//       VOICES_DIR: 'src/voices/v1_0',
//       WEB_PLAYER_PATH: `${this.KOKORO_DIR}/web`,
//       DEVICE_TYPE: 'mps',
//       PYTORCH_ENABLE_MPS_FALLBACK: '1',
//       TORCHVISION_DISABLE_META_REGISTRATION: '1'
//     }

//     /* download model */
//     await this.run(
//       this.pythonBin(),
//       ['docker/scripts/download_model.py', '--output', 'api/src/models/v1_0'],
//       { cwd: this.KOKORO_DIR, env, label: 'model-dl' }
//     )

//     /* start uvicorn */
//     this.kokoroProc?.kill()
//     log.info('[Kokoro] Starting uvicorn server on port 45000')
//     this.kokoroProc = spawn(
//       this.pythonBin(),
//       ['-m', 'uvicorn', 'api.src.main:app', '--host', '0.0.0.0', '--port', '45000'],
//       { cwd: this.KOKORO_DIR, env, stdio: 'pipe' }
//     )

//     this.kokoroProc.stdout?.on('data', (data) => {
//       const output = data.toString().trim()
//       if (output) {
//         log.info(`[Kokoro] [uvicorn] ${output}`)
//       }
//     })

//     this.kokoroProc.stderr?.on('data', (data) => {
//       const output = data.toString().trim()
//       if (output) {
//         log.error(`[Kokoro] [uvicorn] ${output}`)
//       }
//     })

//     this.kokoroProc.on('exit', (code) => {
//       log.info(`[Kokoro] uvicorn server exited with code ${code}`)
//       this.kokoroProc = null
//     })

//     const checkServer = () =>
//       new Promise<boolean>((resolve) => {
//         const req = http.get('http://localhost:45000/web', (res) => {
//           res.resume()
//           resolve(res.statusCode === 200 || res.statusCode === 307)
//         })
//         req.on('error', () => resolve(false))
//       })

//     log.info('[Kokoro] Waiting for server to become ready...')
//     const start = Date.now()
//     const timeout = 10 * 60 * 1000
//     let checkCount = 0
//     while (Date.now() - start < timeout) {
//       if (await checkServer()) {
//         log.info('[Kokoro] Server is ready and responding!')
//         this.onProgress?.({ dependency: 'Kokoro', progress: 100, status: 'Running' })
//         this.latestProgress = { dependency: 'Kokoro', progress: 100, status: 'Running' }
//         return
//       }
//       checkCount++
//       if (checkCount % 10 === 0) {
//         log.info(
//           `[Kokoro] Still waiting for server... (${Math.round((Date.now() - start) / 1000)}s elapsed)`
//         )
//       }
//       await new Promise((r) => setTimeout(r, 1000))
//     }

//     log.error('[Kokoro] Timed out waiting for server to start')
//     throw new Error('Timed out waiting for Kokoro server to start')
//   }

//   async setup() {
//     log.info('[Kokoro] Starting Kokoro setup process')
//     try {
//       this.onProgress?.({
//         dependency: 'Kokoro',
//         progress: 10,
//         status: 'Setting up dependency manager'
//       })
//       this.latestProgress = {
//         dependency: 'Kokoro',
//         progress: 10,
//         status: 'Setting up dependency manager'
//       }
//       await this.ensureUv()
//       this.onProgress?.({ dependency: 'Kokoro', progress: 20, status: 'Installing Python' })
//       this.latestProgress = { dependency: 'Kokoro', progress: 20, status: 'Installing Python' }
//       await this.ensurePython312()
//       this.onProgress?.({ dependency: 'Kokoro', progress: 30, status: 'Downloading Kokoro' })
//       this.latestProgress = { dependency: 'Kokoro', progress: 30, status: 'Downloading Kokoro' }
//       await this.ensureRepo()
//       this.onProgress?.({
//         dependency: 'Kokoro',
//         progress: 45,
//         status: 'Creating virtual environment'
//       })
//       this.latestProgress = {
//         dependency: 'Kokoro',
//         progress: 45,
//         status: 'Creating virtual environment'
//       }
//       await this.ensureVenv()
//       this.onProgress?.({ dependency: 'Kokoro', progress: 60, status: 'Installing dependencies' })
//       this.latestProgress = {
//         dependency: 'Kokoro',
//         progress: 60,
//         status: 'Installing dependencies'
//       }
//       await this.ensureDeps()
//       this.onProgress?.({ dependency: 'Kokoro', progress: 90, status: 'Starting speech model' })
//       this.latestProgress = { dependency: 'Kokoro', progress: 90, status: 'Starting speech model' }
//       await this.startTts()
//     } catch (e) {
//       const error = e instanceof Error ? e.message : 'Unknown error occurred'
//       log.error('[Kokoro] KokoroBootstrap failed', e)
//       this.latestProgress = {
//         dependency: 'Kokoro',
//         progress: this.latestProgress.progress,
//         status: 'Failed',
//         error
//       }
//       this.onProgress?.({ dependency: 'Kokoro', progress: 0, status: 'Failed', error })
//       throw e
//     }
//   }

//   async cleanup() {
//     try {
//       this.kokoroProc?.kill()
//     } finally {
//       this.kokoroProc = null
//     }
//   }
// }
