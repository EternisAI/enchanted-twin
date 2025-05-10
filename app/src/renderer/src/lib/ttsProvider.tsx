import { TTSContext } from '@renderer/hooks/useTTS'
import { ReactNode, useCallback, useMemo, useRef, useState, useEffect } from 'react'

export interface TTSAPI {
  speak: (text: string) => Promise<void>
  stop: () => void
  isSpeaking: boolean
}

export function TTSProvider({
  children,
  wsURL = 'ws://localhost:45001/ws'
}: {
  children: ReactNode
  wsURL?: string
}) {
  const [isSpeaking, setIsSpeaking] = useState(false)

  const audioRef = useRef<HTMLAudioElement | null>(null)
  const wsRef = useRef<WebSocket | null>(null)
  const pcRef = useRef<RTCPeerConnection | null>(null)
  const mediaSourceRef = useRef<MediaSource | null>(null)
  const sourceBufferRef = useRef<SourceBuffer | null>(null)
  const audioQueueRef = useRef<Uint8Array[]>([])
  const eosReceivedRef = useRef<boolean>(false)
  const isStoppingRef = useRef<boolean>(false) // Flag to prevent multiple stop calls

  const stop = useCallback(() => {
    if (isStoppingRef.current) return // Exit if stop is already in progress
    isStoppingRef.current = true

    console.log('[TTS] stop() called')

    wsRef.current?.close()
    wsRef.current = null

    pcRef.current?.close()
    pcRef.current = null

    if (audioRef.current) {
      audioRef.current.pause()
      audioRef.current.src = ''
      audioRef.current.load()
      audioRef.current.removeEventListener('error', audioErrorListener)
      if (audioRef.current.parentNode) {
        audioRef.current.parentNode.removeChild(audioRef.current)
      }
      audioRef.current = null
    }

    mediaSourceRef.current = null
    sourceBufferRef.current = null
    audioQueueRef.current = []
    eosReceivedRef.current = false

    setIsSpeaking(false)
    isStoppingRef.current = false
  }, [])

  const ensureAudio = useCallback(() => {
    if (!audioRef.current) {
      const el = document.createElement('audio')
      el.autoplay = true
      el.controls = false
      el.style.display = 'none'
      el.addEventListener('ended', () => {
        stop()
      })
      document.body.appendChild(el)
      audioRef.current = el
    }
    return audioRef.current
  }, [stop])

  const waitIceComplete = (pc: RTCPeerConnection): Promise<void> =>
    new Promise((resolve) => {
      if (pc.iceGatheringState === 'complete') {
        resolve()
        return
      }
      pc.addEventListener('icegatheringstatechange', () => {
        if (pc.iceGatheringState === 'complete') {
          resolve()
        }
      })
    })

  const trySignalEndOfStream = useCallback(() => {
    const ms = mediaSourceRef.current
    const sb = sourceBufferRef.current

    if (
      eosReceivedRef.current &&
      ms &&
      ms.readyState === 'open' &&
      sb &&
      !sb.updating &&
      audioQueueRef.current.length === 0
    ) {
      try {
        ms.endOfStream()
      } catch (e) {
        console.error('Error calling endOfStream in trySignalEndOfStream:', e)
        stop()
      }
    }
  }, [stop])

  const pump = useCallback(() => {
    const sb = sourceBufferRef.current

    if (!sb || sb.updating || audioQueueRef.current.length === 0) {
      if (audioQueueRef.current.length === 0) {
        trySignalEndOfStream()
      }
      return
    }

    try {
      sb.appendBuffer(audioQueueRef.current.shift()!)
    } catch (e) {
      console.error('Error appending buffer in pump:', e)
      stop()
    }
  }, [stop, trySignalEndOfStream])

  const audioErrorListener = useCallback(
    (e: Event) => {
      console.error('[TTS] Audio error:', e)
      stop()
    },
    [stop]
  )

  const speak = useCallback(
    async (text: string) => {
      if (!text.trim()) {
        console.warn('[TTS] speak() called with empty text')
        return
      }

      if (wsRef.current || pcRef.current || isSpeaking) {
        console.log('[TTS] Cleaning up previous connection before speaking')
        stop()
        await new Promise((resolve) => setTimeout(resolve, 100))
      }

      eosReceivedRef.current = false
      audioQueueRef.current = []

      const audio = ensureAudio()
      audio.addEventListener('error', audioErrorListener)

      const ws = new WebSocket(wsURL)
      const pc = new RTCPeerConnection()
      pc.createDataChannel('audio', { ordered: true })

      wsRef.current = ws
      pcRef.current = pc

      const mediaSource = new MediaSource()
      mediaSourceRef.current = mediaSource

      mediaSource.addEventListener('sourceopen', () => {
        if (mediaSourceRef.current !== mediaSource) return
        try {
          sourceBufferRef.current = mediaSource.addSourceBuffer('audio/mpeg')
          sourceBufferRef.current.addEventListener('updateend', pump)
          sourceBufferRef.current.addEventListener('error', (ev) => {
            console.error('[TTS] SourceBuffer error:', ev)
            stop()
          })
          pump()
        } catch (e) {
          console.error("[TTS] Error in MediaSource 'sourceopen':", e)
          stop()
        }
      })

      mediaSource.addEventListener('sourceended', () => {
        console.log('[TTS] MediaSource sourceended')
      })

      mediaSource.addEventListener('sourceclose', () => {
        console.log('[TTS] MediaSource sourceclose')
        if (wsRef.current || pcRef.current) {
          stop()
        }
      })

      audio.src = URL.createObjectURL(mediaSource)

      pc.ondatachannel = ({ channel }) => {
        console.log('[TTS] Data channel created')
        channel.binaryType = 'arraybuffer'

        channel.onopen = () => console.log('[TTS] Data channel open')
        channel.onclose = () => console.log('[TTS] Data channel closed') // No stop()
        channel.onerror = (e) => {
          console.error('[TTS] Data channel error:', e)
          stop()
        }

        channel.onmessage = async ({ data }) => {
          if (typeof data === 'string' && data === 'EOS') {
            console.log('[TTS] Received EOS')
            eosReceivedRef.current = true
            if (audioQueueRef.current.length === 0) trySignalEndOfStream()
            return
          }

          const chunk =
            data instanceof ArrayBuffer
              ? new Uint8Array(data)
              : new Uint8Array(await (data as Blob).arrayBuffer())
          audioQueueRef.current.push(chunk)
          console.log('[TTS] Received chunk, queue:', audioQueueRef.current.length)

          if (
            sourceBufferRef.current &&
            !sourceBufferRef.current.updating &&
            mediaSourceRef.current?.readyState === 'open'
          ) {
            pump()
          }
        }
      }

      ws.onopen = async () => {
        console.log('[TTS] WebSocket opened')
        setIsSpeaking(true)
        try {
          const offer = await pc.createOffer()
          await pc.setLocalDescription(offer)
          await waitIceComplete(pc)
          ws.send(JSON.stringify({ sdp: pc.localDescription!.sdp, text }))
          console.log('[TTS] Sent SDP offer')
        } catch (error) {
          console.error('[TTS] Error in offer creation:', error)
          stop()
        }
      }

      ws.onmessage = async ({ data }) => {
        console.log('[TTS] Received SDP answer')
        try {
          await pc.setRemoteDescription(new RTCSessionDescription(JSON.parse(data)))
          console.log('[TTS] Set remote description')
        } catch (error) {
          console.error('[TTS] Error setting remote description:', error)
          stop()
        }
      }

      ws.onerror = (e) => {
        console.error('[TTS] WebSocket error:', e)
        stop()
      }
      ws.onclose = (e) => console.log('[TTS] WebSocket closed', e) // No stop()
    },
    [wsURL, stop, ensureAudio, pump, trySignalEndOfStream, isSpeaking, audioErrorListener]
  )

  const api = useMemo<TTSAPI>(() => ({ speak, stop, isSpeaking }), [speak, stop, isSpeaking])

  useEffect(() => {
    return () => {
      stop()
    }
  }, [stop])

  return <TTSContext.Provider value={api}>{children}</TTSContext.Provider>
}
