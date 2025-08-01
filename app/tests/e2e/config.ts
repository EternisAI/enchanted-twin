import dotenv from 'dotenv'
import path from 'path'

const envPath = path.join(__dirname, './.env')
const result = dotenv.config({ path: envPath })

if (result.error) {
  console.warn(`Warning: Could not load .env file from ${envPath}`)
  console.warn(
    'Please create a .env file in the project root with the required environment variables'
  )
}

function getRequiredEnv(key: string, fallback?: string): string {
  const value = process.env[key] || fallback
  console.log('🔍 Value:', value)

  return value || ''
}

function getOptionalEnv(key: string, fallback: string = ''): string {
  return process.env[key] || fallback
}

export const E2E_CONFIG = {
  BACKEND_GRAPHQL_PORT: '44999',
  BACKEND_WEAVIATE_PORT: '51415',

  TEST_DB_PATH: './output/sqlite/store_test.db',
  TEST_APP_DATA_PATH: './output_test',

  BACKEND_STARTUP_TIMEOUT: 60000,
  BACKEND_READY_TIMEOUT: 30000,
  APP_STARTUP_TIMEOUT: 300000,
  DEPENDENCY_CHECK_INTERVAL: 1000,

  APP_RUNTIME_DURATION: 3 * 60 * 1000,
  SCREENSHOT_INTERVAL: 30000,

  getGraphQLUrl: (port?: string) => `http://localhost:${port || '44999'}/query`,
  getWeaviateUrl: (port?: string) => `http://localhost:${port || '51415'}`
} as const

export const GOOGLE_TEST_CREDENTIALS = {
  EMAIL: getRequiredEnv('E2E_TEST_EMAIL'),
  PASSWORD: getRequiredEnv('E2E_TEST_PASSWORD')
} as const

export const FIREBASE_TEST_CONFIG = {
  FIREBASE_API_KEY: getRequiredEnv('VITE_FIREBASE_API_KEY'),
  FIREBASE_AUTH_DOMAIN: getRequiredEnv('VITE_FIREBASE_AUTH_DOMAIN'),
  FIREBASE_PROJECT_ID: getRequiredEnv('VITE_FIREBASE_PROJECT_ID')
} as const

export const AUTH_CONFIG = {
  AUTH_STATE_PATH: 'test-results/.auth/user.json',

  OAUTH_TIMEOUT: 30000,
  LOGIN_TIMEOUT: 15000,

  AUTH_RETRY_COUNT: 3,
  AUTH_RETRY_DELAY: 2000
} as const

export const BACKEND_ENV = {
  DB_PATH: E2E_CONFIG.TEST_DB_PATH,
  APP_DATA_PATH: E2E_CONFIG.TEST_APP_DATA_PATH,
  GRAPHQL_PORT: E2E_CONFIG.BACKEND_GRAPHQL_PORT,
  WEAVIATE_PORT: E2E_CONFIG.BACKEND_WEAVIATE_PORT,
  E2E_TEST_MODE: 'true',
  NODE_ENV: 'production',

  COMPLETIONS_API_KEY: getRequiredEnv('COMPLETIONS_API_KEY', getOptionalEnv('OPENROUTER_API_KEY')),
  COMPLETIONS_API_URL: getOptionalEnv('COMPLETIONS_API_URL', 'https://openrouter.ai/api/v1'),
  COMPLETIONS_MODEL: getOptionalEnv('COMPLETIONS_MODEL', 'openai/gpt-4o-mini'),
  REASONING_MODEL: getOptionalEnv('REASONING_MODEL', 'openai/gpt-4.1'),
  EMBEDDINGS_API_KEY: getRequiredEnv('EMBEDDINGS_API_KEY', getOptionalEnv('OPENAI_API_KEY')),
  EMBEDDINGS_API_URL: getOptionalEnv('EMBEDDINGS_API_URL', 'https://api.openai.com/v1'),
  EMBEDDINGS_MODEL: getOptionalEnv('EMBEDDINGS_MODEL', 'text-embedding-3-small'),
  CONTAINER_RUNTIME: getOptionalEnv('CONTAINER_RUNTIME', 'podman'),
  TELEGRAM_CHAT_SERVER: getOptionalEnv('TELEGRAM_CHAT_SERVER'),
  HOLON_API_URL: getOptionalEnv('HOLON_API_URL'),
  ENCHANTED_MCP_URL: getOptionalEnv('ENCHANTED_MCP_URL'),
  PROXY_TEE_URL: getOptionalEnv('PROXY_TEE_URL'),
  INVITE_SERVER_URL: getOptionalEnv('INVITE_SERVER_URL'),
  MCP_CLIENT_ID: getOptionalEnv('MCP_CLIENT_ID'),
  MCP_CLIENT_SECRET: getOptionalEnv('MCP_CLIENT_SECRET'),
  OPENROUTER_API_KEY: getRequiredEnv('OPENROUTER_API_KEY'),
  OPENAI_API_KEY: getRequiredEnv('OPENAI_API_KEY'),
  TINFOIL_API_KEY: getOptionalEnv('TINFOIL_API_KEY'),
  ANONYMIZER_TYPE: getOptionalEnv('ANONYMIZER_TYPE', 'no-op'),

  FIREBASE_API_KEY: FIREBASE_TEST_CONFIG.FIREBASE_API_KEY,
  FIREBASE_AUTH_DOMAIN: FIREBASE_TEST_CONFIG.FIREBASE_AUTH_DOMAIN,
  FIREBASE_PROJECT_ID: FIREBASE_TEST_CONFIG.FIREBASE_PROJECT_ID
} as const

export const SCREENSHOT_PATH = 'test-results/artifacts/'
