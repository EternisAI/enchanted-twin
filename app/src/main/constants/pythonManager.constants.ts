export const PYTHON_VERSION = '3.12'
export const LIVEKIT_AGENT_PORT = 45000
export const DEFAULT_VOICE = 'af_v0bella'
export const INSTALL_TIMEOUT_MS = 10 * 60 * 1000 // 10 minutes
export const GRACEFUL_SHUTDOWN_TIMEOUT_MS = 2000

export const UV_INSTALL_SCRIPT = 'curl -LsSf https://astral.sh/uv/install.sh | sh'

export const REQUIRED_ENV_VARS = [
  'TINFOIL_API_KEY',
  'TINFOIL_AUDIO_URL',
  'TINFOIL_STT_MODEL',
  'TINFOIL_TTS_MODEL'
] as const

export const PYTHON_REQUIREMENTS = `livekit==1.0.8
livekit-agents==1.0.23
livekit-plugins-openai==1.0.23
livekit-plugins-deepgram==1.0.23
livekit-plugins-silero==1.0.23
python-dotenv>=1.0.0
requests
certifi>=2024.2.2
websockets>=12.0`

export const PROGRESS_STEPS = {
  UV_SETUP: 10,
  PYTHON_INSTALL: 20,
  AGENT_FILES: 40,
  VENV_CREATION: 60,
  DEPENDENCIES: 80,
  COMPLETE: 100
} as const

export const SESSION_READY_INDICATORS = [
  'Agent session started successfully'
] as const