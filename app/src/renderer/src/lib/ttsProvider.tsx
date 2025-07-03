import { ReactNode, useCallback, useEffect, useMemo, useRef, useState } from 'react'
import { TTSContext } from '@renderer/hooks/useTTS'

export interface TTSAPI {
  speak: (text: string) => Promise<void>
  speakWithEvents: (text: string) => { started: Promise<void>; finished: Promise<void> }
  stop: () => void
  isSpeaking: boolean
  isLoading: boolean
  getFreqData: () => Uint8Array
  getTimeData: () => Uint8Array
}

export function TTSProvider({
  children,
  wsURL = 'ws://localhost:45001/ws'
}: {
  children: ReactNode
  wsURL?: string
}) {
  const [isSpeaking, setIsSpeaking] = useState(false)
  const [isLoading, setIsLoading] = useState(false)

  const audioRef = useRef<HTMLAudioElement | null>(null)
  const srcNodeRef = useRef<MediaElementAudioSourceNode | null>(null)
  const wsRef = useRef<WebSocket | null>(null)
  const pcRef = useRef<RTCPeerConnection | null>(null)
  const mediaSourceRef = useRef<MediaSource | null>(null)
  const sourceBufferRef = useRef<SourceBuffer | null>(null)
  const audioQueueRef = useRef<Uint8Array[]>([])
  const eosReceivedRef = useRef(false)
  const stoppingRef = useRef(false)

  const ctxRef = useRef<AudioContext | null>(null)
  const analyserRef = useRef<AnalyserNode | null>(null)
  const freqBufRef = useRef<Uint8Array>(new Uint8Array(0))
  const timeBufRef = useRef<Uint8Array>(new Uint8Array(0))

  const startedResRef = useRef<(() => void) | null>(null)
  const finishedResRef = useRef<(() => void) | null>(null)

  const getFreqData = useCallback(() => {
    if (!analyserRef.current) return freqBufRef.current
    analyserRef.current.getByteFrequencyData(freqBufRef.current)
    return freqBufRef.current
  }, [])

  const getTimeData = useCallback(() => {
    if (!analyserRef.current) return timeBufRef.current
    analyserRef.current.getByteTimeDomainData(timeBufRef.current)
    return timeBufRef.current
  }, [])

  const stop = useCallback(() => {
    if (stoppingRef.current) return
    stoppingRef.current = true

    wsRef.current?.close()
    wsRef.current = null
    pcRef.current?.close()
    pcRef.current = null

    srcNodeRef.current?.disconnect()
    srcNodeRef.current = null

    if (audioRef.current) {
      audioRef.current.pause()
      audioRef.current.src = ''
      audioRef.current.load()
      audioRef.current.remove()
      audioRef.current = null
    }

    mediaSourceRef.current = null
    sourceBufferRef.current = null
    audioQueueRef.current = []
    eosReceivedRef.current = false

    startedResRef.current?.()
    finishedResRef.current?.()
    startedResRef.current = null
    finishedResRef.current = null

    setIsSpeaking(false)
    setIsLoading(false)
    stoppingRef.current = false
  }, [])

  const ensureAudio = useCallback((): HTMLAudioElement => {
    if (!audioRef.current) {
      const el = document.createElement('audio')
      el.autoplay = true
      el.style.display = 'none'
      el.addEventListener('ended', stop)
      document.body.appendChild(el)
      audioRef.current = el
    }

    if (!ctxRef.current) {
      ctxRef.current = new AudioContext()
      analyserRef.current = ctxRef.current.createAnalyser()
      analyserRef.current.fftSize = 2048
      freqBufRef.current = new Uint8Array(analyserRef.current.frequencyBinCount)
      timeBufRef.current = new Uint8Array(analyserRef.current.fftSize)
    }

    srcNodeRef.current?.disconnect()
    srcNodeRef.current = ctxRef.current.createMediaElementSource(audioRef.current)
    srcNodeRef.current.connect(analyserRef.current!)
    srcNodeRef.current.connect(ctxRef.current.destination)

    return audioRef.current
  }, [stop])

  const waitIceComplete = (pc: RTCPeerConnection) =>
    new Promise<void>((resolve) => {
      if (pc.iceGatheringState === 'complete') return resolve()
      const fn = () => {
        if (pc.iceGatheringState === 'complete') {
          pc.removeEventListener('icegatheringstatechange', fn)
          resolve()
        }
      }
      pc.addEventListener('icegatheringstatechange', fn)
    })

  const tryEndOfStream = useCallback(() => {
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
      } catch {
        stop()
      }
    }
  }, [stop])

  const pump = useCallback(() => {
    const sb = sourceBufferRef.current
    if (!sb || sb.updating || audioQueueRef.current.length === 0) {
      if (audioQueueRef.current.length === 0) tryEndOfStream()
      return
    }
    try {
      sb.appendBuffer(audioQueueRef.current.shift()!)
    } catch {
      stop()
    }
  }, [tryEndOfStream, stop])

  const _createSpeech = useCallback(
    (text: string): { started: Promise<void>; finished: Promise<void> } => {
      let startedRes!: () => void
      let finishedRes!: () => void

      const started = new Promise<void>((res) => (startedRes = res))
      const finished = new Promise<void>((res) => (finishedRes = res))

      startedResRef.current = startedRes
      finishedResRef.current = finishedRes
      ;(async () => {
        if (!text.trim()) {
          startedRes()
          finishedRes()
          return
        }

        if (wsRef.current || pcRef.current || isSpeaking) {
          stop()
          await new Promise((r) => setTimeout(r, 100))
        }

        setIsLoading(true)
        eosReceivedRef.current = false
        audioQueueRef.current = []

        const audio = ensureAudio()
        const ws = new WebSocket(wsURL)
        const pc = new RTCPeerConnection()
        wsRef.current = ws
        pcRef.current = pc
        pc.createDataChannel('audio', { ordered: true })

        const mediaSource = new MediaSource()
        mediaSourceRef.current = mediaSource
        mediaSource.addEventListener('sourceopen', () => {
          try {
            const sb = mediaSource.addSourceBuffer('audio/mpeg')
            sourceBufferRef.current = sb
            sb.addEventListener('updateend', pump)
            pump()
          } catch {
            stop()
          }
        })
        audio.src = URL.createObjectURL(mediaSource)

        let firstChunk = true
        pc.ondatachannel = ({ channel }) => {
          channel.binaryType = 'arraybuffer'
          channel.onmessage = async ({ data }) => {
            if (typeof data !== 'string') {
              if (firstChunk) {
                firstChunk = false
                setIsLoading(false)
                setIsSpeaking(true)
                startedRes()
              }
              const chunk =
                data instanceof ArrayBuffer
                  ? new Uint8Array(data)
                  : new Uint8Array(await (data as Blob).arrayBuffer())
              audioQueueRef.current.push(chunk)
              pump()
              return
            }
            if (data === 'EOS') {
              eosReceivedRef.current = true
              if (audioQueueRef.current.length === 0) tryEndOfStream()
            }
          }
        }

        ws.onopen = async () => {
          try {
            const offer = await pc.createOffer()
            await pc.setLocalDescription(offer)
            await waitIceComplete(pc)
            ws.send(JSON.stringify({ sdp: pc.localDescription!.sdp, text }))
          } catch {
            stop()
          }
        }
        ws.onmessage = async ({ data }) => {
          try {
            await pc.setRemoteDescription(new RTCSessionDescription(JSON.parse(data as string)))
          } catch {
            stop()
          }
        }

        ws.onerror = ws.onclose = () => stop()
      })().catch(() => stop()) // safeguard

      finished.then(() => {
        startedResRef.current = null
        finishedResRef.current = null
      })

      return { started, finished }
    },
    [wsURL, ensureAudio, pump, tryEndOfStream, stop, isSpeaking]
  )

  const speakWithEvents = useCallback((text: string) => _createSpeech(text), [_createSpeech])

  const speak = useCallback((text: string) => _createSpeech(text).finished, [_createSpeech])

  const api = useMemo<TTSAPI>(
    () => ({
      speak,
      speakWithEvents,
      stop,
      isSpeaking,
      isLoading,
      getFreqData,
      getTimeData
    }),
    [speak, speakWithEvents, stop, isSpeaking, isLoading, getFreqData, getTimeData]
  )

  useEffect(() => stop, [stop])

  return <TTSContext.Provider value={api}>{children}</TTSContext.Provider>
}
