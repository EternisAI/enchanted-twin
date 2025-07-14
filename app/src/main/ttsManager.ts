import OpenAI from 'openai'
import log from 'electron-log/main'

let openaiClient: OpenAI | null = null

async function initTTSClient(firebaseToken: string): Promise<OpenAI | null> {
  try {
    if (!firebaseToken) {
      log.error('[TTS] No Firebase token provided')
      return null
    }

    openaiClient = new OpenAI({
      apiKey: firebaseToken,
      baseURL: 'https://audio-processing.model.tinfoil.sh/v1/'
    })

    log.info('[TTS] OpenAI client initialized successfully')
    return openaiClient
  } catch (error) {
    log.error('[TTS] Failed to initialize OpenAI client:', error)
    return null
  }
}

export async function generateTTS(
  text: string,
  firebaseToken: string
): Promise<ArrayBuffer | null> {
  try {
    if (!firebaseToken) {
      log.error('[TTS] No Firebase token provided for TTS generation')
      return null
    }

    // Initialize client if not already done or if token changed
    if (!openaiClient) {
      openaiClient = await initTTSClient(firebaseToken)
    }

    if (!openaiClient) {
      log.error('[TTS] OpenAI client not available')
      return null
    }

    log.info('[TTS] Generating TTS for text length:', text.length)

    const mp3Response = await openaiClient.audio.speech.create({
      model: 'kokoro', // @TODO: make this basde on .env
      voice: 'af_v0bella',
      input: text
    })

    const arrayBuffer = await mp3Response.arrayBuffer()
    log.info('[TTS] TTS generation successful, size:', arrayBuffer.byteLength)

    return arrayBuffer
  } catch (error) {
    log.error('[TTS] Failed to generate TTS:', error)
    return null
  }
}
