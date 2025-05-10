import { TTSContext } from '@renderer/hooks/useTTS'
import { ReactNode, useCallback, useMemo, useRef, useState, useEffect } from 'react'

export interface TTSAPI {
  speak: (text: string) => Promise<void>
  stop: () => void
  isSpeaking: boolean
}

export function TTSProvider({
  children,
  wsURL = 'ws://localhost:8080/ws'
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

  const stop = useCallback(() => {
    wsRef.current?.close()
    wsRef.current = null

    pcRef.current?.close()
    pcRef.current = null

    if (audioRef.current) {
      audioRef.current.pause()
      audioRef.current.src = ''
    }

    mediaSourceRef.current = null
    sourceBufferRef.current = null
    audioQueueRef.current = []
    eosReceivedRef.current = false

    setIsSpeaking(false)
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

  const speak = useCallback(
    async (text: string) => {
      if (!text.trim()) return

      if (wsRef.current || pcRef.current || isSpeaking) {
        stop()
        await new Promise((resolve) => setTimeout(resolve, 100))
      }

      eosReceivedRef.current = false
      audioQueueRef.current = []

      const audio = ensureAudio()
      const ws = new WebSocket(wsURL)
      const pc = new RTCPeerConnection()
      pc.createDataChannel('audio', { ordered: true }) // Create data channel, dc variable removed as unused

      wsRef.current = ws
      pcRef.current = pc

      const mediaSource = new MediaSource()
      mediaSourceRef.current = mediaSource

      mediaSource.addEventListener('sourceopen', () => {
        if (mediaSourceRef.current !== mediaSource) {
          return
        }
        try {
          sourceBufferRef.current = mediaSource.addSourceBuffer('audio/mpeg')

          sourceBufferRef.current.addEventListener('updateend', pump)
          sourceBufferRef.current.addEventListener('error', (ev) => {
            console.error('SourceBuffer error:', ev)
            stop()
          })
          pump()
        } catch (e) {
          console.error("Error in MediaSource 'sourceopen' (addSourceBuffer):", e)
          stop()
        }
      })

      mediaSource.addEventListener('sourceended', () => {
        // Logic for source ended if needed
      })

      mediaSource.addEventListener('sourceclose', () => {
        if (wsRef.current || pcRef.current) {
          stop()
        }
      })

      audio.src = URL.createObjectURL(mediaSource)

      pc.ondatachannel = ({ channel }) => {
        channel.binaryType = 'arraybuffer'

        channel.onmessage = async ({ data }) => {
          if (typeof data === 'string' && data === 'EOS') {
            eosReceivedRef.current = true
            if (audioQueueRef.current.length === 0) {
              trySignalEndOfStream()
            }
            return
          }

          const chunk =
            data instanceof ArrayBuffer
              ? new Uint8Array(data)
              : new Uint8Array(await (data as Blob).arrayBuffer())
          audioQueueRef.current.push(chunk)

          if (
            sourceBufferRef.current &&
            !sourceBufferRef.current.updating &&
            mediaSourceRef.current &&
            mediaSourceRef.current.readyState === 'open'
          ) {
            pump()
          }
        }

        channel.onclose = () => {
          stop()
        }
        channel.onerror = (e) => {
          console.error('DataChannel error:', e)
          stop()
        }
      }

      ws.onopen = async () => {
        setIsSpeaking(true)
        try {
          const offer = await pc.createOffer()
          await pc.setLocalDescription(offer)
          await waitIceComplete(pc)

          ws.send(
            JSON.stringify({
              sdp: pc.localDescription!.sdp,
              text
            })
          )
        } catch (error) {
          console.error('Error during WebRTC offer creation/sending:', error)
          stop()
        }
      }

      ws.onmessage = async ({ data }) => {
        try {
          await pc.setRemoteDescription(new RTCSessionDescription(JSON.parse(data as string)))
        } catch (error) {
          console.error('Error setting remote SDP description:', error)
          stop()
        }
      }

      ws.onerror = (e) => {
        console.error('WebSocket error:', e)
        stop()
      }
      ws.onclose = () => {
        stop()
      }
    },
    [wsURL, stop, ensureAudio, pump, trySignalEndOfStream, isSpeaking]
  )

  const api = useMemo<TTSAPI>(() => ({ speak, stop, isSpeaking }), [speak, stop, isSpeaking])

  useEffect(() => {
    const currentAudioRef = audioRef.current
    return () => {
      if (currentAudioRef) {
        currentAudioRef.pause()
        currentAudioRef.src = ''
        if (currentAudioRef.parentNode) {
          currentAudioRef.parentNode.removeChild(currentAudioRef)
        }
      }
      stop()
    }
  }, [stop])

  return <TTSContext.Provider value={api}>{children}</TTSContext.Provider>
}
