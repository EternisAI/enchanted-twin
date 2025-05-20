import { ReactNode, useCallback, useEffect, useMemo, useRef, useState } from 'react'
import { TTSContext } from '@renderer/hooks/useTTS'
/* ────────────────────────────────────  Public API  ──────────────────────────────────── */

export interface TTSAPI {
  speak: (text: string) => Promise<void>
  stop: () => void
  isSpeaking: boolean
  isLoading: boolean
  /** frequency-domain data (0-255 per FFT bin) */
  getFreqData: () => Uint8Array
  /** time-domain waveform data (0-255) */
  getTimeData: () => Uint8Array
}

/* ────────────────────────────────────  Provider  ──────────────────────────────────── */

export function TTSProvider({
  children,
  wsURL = 'ws://localhost:45001/ws'
}: {
  children: ReactNode
  wsURL?: string
}) {
  /* ───────────── state ───────────── */
  const [isSpeaking, setIsSpeaking] = useState(false)
  const [isLoading, setIsLoading] = useState(false)

  /* ───────────── refs ───────────── */
  const audioRef = useRef<HTMLAudioElement | null>(null)
  const srcNodeRef = useRef<MediaElementAudioSourceNode | null>(null)
  const wsRef = useRef<WebSocket | null>(null)
  const pcRef = useRef<RTCPeerConnection | null>(null)
  const mediaSourceRef = useRef<MediaSource | null>(null)
  const sourceBufferRef = useRef<SourceBuffer | null>(null)
  const audioQueueRef = useRef<Uint8Array[]>([])
  const eosReceivedRef = useRef(false)
  const stoppingRef = useRef(false)

  /* web-audio (visualiser) */
  const ctxRef = useRef<AudioContext | null>(null)
  const analyserRef = useRef<AnalyserNode | null>(null)
  const freqBufRef = useRef<Uint8Array>(new Uint8Array(0))
  const timeBufRef = useRef<Uint8Array>(new Uint8Array(0))

  /* ───────────── helpers ───────────── */
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
    pcRef.current?.close()
    wsRef.current = pcRef.current = null

    /* disconnect previous source node */
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

    /* (re-)create Web-Audio graph every time we have a *new* element */
    if (!ctxRef.current) {
      ctxRef.current = new AudioContext()
      analyserRef.current = ctxRef.current.createAnalyser()
      analyserRef.current.fftSize = 2048
      freqBufRef.current = new Uint8Array(analyserRef.current.frequencyBinCount)
      timeBufRef.current = new Uint8Array(analyserRef.current.fftSize)
    }

    /* disconnect old source and connect current element */
    srcNodeRef.current?.disconnect()
    srcNodeRef.current = ctxRef.current.createMediaElementSource(audioRef.current!)
    srcNodeRef.current.connect(analyserRef.current!)
    srcNodeRef.current.connect(ctxRef.current.destination)

    return audioRef.current
  }, [stop])

  /* Wait until the peer-connection has gathered all ICE candidates */
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

  /* ───────────── speak() ───────────── */
  const speak = useCallback(
    async (text: string) => {
      if (!text.trim()) return

      /* clean previous session */
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

      /* MediaSource */
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

      /* data channel → audio queue */
      pc.ondatachannel = ({ channel }) => {
        channel.binaryType = 'arraybuffer'
        channel.onmessage = async ({ data }) => {
          if (isLoading) setIsLoading(false)
          if (typeof data === 'string' && data === 'EOS') {
            eosReceivedRef.current = true
            if (audioQueueRef.current.length === 0) tryEndOfStream()
            return
          }
          const chunk =
            data instanceof ArrayBuffer
              ? new Uint8Array(data)
              : new Uint8Array(await (data as Blob).arrayBuffer())
          audioQueueRef.current.push(chunk)
          pump()
        }
      }

      /* signalling */
      ws.onopen = async () => {
        try {
          const offer = await pc.createOffer()
          await pc.setLocalDescription(offer)
          await waitIceComplete(pc)
          ws.send(JSON.stringify({ sdp: pc.localDescription!.sdp, text }))
          setIsSpeaking(true)
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
    },
    [wsURL, ensureAudio, pump, tryEndOfStream, stop, isSpeaking, isLoading]
  )

  /* ───────────── public value ───────────── */
  const api = useMemo<TTSAPI>(
    () => ({
      speak,
      stop,
      isSpeaking,
      isLoading,
      getFreqData,
      getTimeData
    }),
    [speak, stop, isSpeaking, isLoading, getFreqData, getTimeData]
  )

  /* cleanup on unmount */
  useEffect(() => stop, [stop])

  return <TTSContext.Provider value={api}>{children}</TTSContext.Provider>
}
