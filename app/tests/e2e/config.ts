import dotenv from 'dotenv'
import path from 'path'

// Load environment variables from .env file
dotenv.config({ path: path.join(__dirname, '../../.env') })

// E2E Test Configuration
// Single source of truth for test environment settings

export const E2E_CONFIG = {
  // Backend server ports (different from production to avoid conflicts)
  BACKEND_GRAPHQL_PORT: '44999',
  BACKEND_WEAVIATE_PORT: '51415',

  // Test database and storage paths
  TEST_DB_PATH: './output/sqlite/store_test.db',
  TEST_APP_DATA_PATH: './output_test',

  // Timeouts and intervals
  BACKEND_STARTUP_TIMEOUT: 60000, // 60 seconds
  BACKEND_READY_TIMEOUT: 30000, // 30 seconds
  APP_STARTUP_TIMEOUT: 300000, // 5 minutes (backend + 3min app runtime)
  DEPENDENCY_CHECK_INTERVAL: 1000, // 1 second

  // Test runtime configuration
  APP_RUNTIME_DURATION: 3 * 60 * 1000, // 3 minutes
  SCREENSHOT_INTERVAL: 30000, // 30 seconds

  // API endpoints
  getGraphQLUrl: (port?: string) => `http://localhost:${port || '44999'}/query`,

  getWeaviateUrl: (port?: string) => `http://localhost:${port || '51415'}`
} as const

// Google OAuth test credentials
export const GOOGLE_TEST_CREDENTIALS = {
  EMAIL: 'golemfzco@gmail.com',
  PASSWORD: 'RisitasAhi_808'
} as const

// Firebase configuration (loaded from .env file)
export const FIREBASE_TEST_CONFIG = {
  FIREBASE_API_KEY: process.env.VITE_FIREBASE_API_KEY || '',
  FIREBASE_AUTH_DOMAIN: process.env.VITE_FIREBASE_AUTH_DOMAIN || '',
  FIREBASE_PROJECT_ID: process.env.VITE_FIREBASE_PROJECT_ID || ''
} as const

// Authentication test configuration
export const AUTH_CONFIG = {
  // Path to store authentication state
  AUTH_STATE_PATH: 'test-results/.auth/user.json',

  // OAuth timeouts
  OAUTH_TIMEOUT: 30000, // 30 seconds
  LOGIN_TIMEOUT: 15000, // 15 seconds

  // Authentication retry settings
  AUTH_RETRY_COUNT: 3,
  AUTH_RETRY_DELAY: 2000 // 2 seconds
} as const

// Environment variables for the backend server
export const BACKEND_ENV = {
  COMPLETIONS_API_KEY: '',
  COMPLETIONS_API_URL: 'https://openrouter.ai/api/v1',
  COMPLETIONS_MODEL: 'openai/gpt-4o-mini',
  REASONING_MODEL: 'openai/gpt-4.1',
  EMBEDDINGS_API_KEY: 'your-api-key-here',
  EMBEDDINGS_API_URL: 'https://api.openai.com/v1',
  EMBEDDINGS_MODEL: 'text-embedding-3-small',
  DB_PATH: E2E_CONFIG.TEST_DB_PATH,
  APP_DATA_PATH: E2E_CONFIG.TEST_APP_DATA_PATH,
  GRAPHQL_PORT: E2E_CONFIG.BACKEND_GRAPHQL_PORT,
  WEAVIATE_PORT: E2E_CONFIG.BACKEND_WEAVIATE_PORT,
  CONTAINER_RUNTIME: 'podman',
  TELEGRAM_CHAT_SERVER: 'https://7f496ea30f3a.ngrok-free.app/query',
  HOLON_API_URL: 'http://23.22.67.228:8123',
  ENCHANTED_MCP_URL: 'https://enchanted-proxy-dev.up.railway.app/mcp',
  PROXY_TEE_URL: 'https://enchanted-proxy-dev.up.railway.app',
  INVITE_SERVER_URL: 'http://52.90.4.74:8080',
  MCP_CLIENT_ID: 'your-client-id-here',
  MCP_CLIENT_SECRET: 'your-client-secret-here',
  OPENROUTER_API_KEY: '',
  OPENAI_API_KEY: '',
  TINFOIL_API_KEY: 'your-tinfoil-api-key-here',
  ANONYMIZER_TYPE: 'no-op',
  E2E_TEST_MODE: 'true',
  NODE_ENV: 'production',
  // Add Firebase config for backend
  FIREBASE_API_KEY: FIREBASE_TEST_CONFIG.FIREBASE_API_KEY,
  FIREBASE_AUTH_DOMAIN: FIREBASE_TEST_CONFIG.FIREBASE_AUTH_DOMAIN,
  FIREBASE_PROJECT_ID: FIREBASE_TEST_CONFIG.FIREBASE_PROJECT_ID
} as const
