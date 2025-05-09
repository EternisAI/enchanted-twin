import { useCallback, useRef } from 'react'

export function useTTS(wsURL = 'ws://localhost:8080/ws') {
  const audioRef = useRef<HTMLAudioElement | null>(null)
  const wsRef = useRef<WebSocket | null>(null)
  const pcRef = useRef<RTCPeerConnection | null>(null)

  const ensureAudio = () => {
    if (!audioRef.current) {
      const el = document.createElement('audio')
      el.autoplay = true
      el.controls = false
      el.style.display = 'none'
      document.body.appendChild(el)
      audioRef.current = el
    }
    return audioRef.current
  }

  const waitIceComplete = (pc: RTCPeerConnection) =>
    new Promise<void>((res) => {
      if (pc.iceGatheringState === 'complete') return res()
      pc.addEventListener('icegatheringstatechange', () => {
        if (pc.iceGatheringState === 'complete') res()
      })
    })

  const stop = useCallback(() => {
    wsRef.current?.close()
    wsRef.current = null

    pcRef.current?.close()
    pcRef.current = null

    if (audioRef.current) {
      audioRef.current.pause()
      audioRef.current.src = ''
    }
  }, [])

  const speak = useCallback(
    async (text: string) => {
      if (!text.trim()) return
      if (wsRef.current || pcRef.current) stop()

      const ws = new WebSocket(wsURL)
      const pc = new RTCPeerConnection()
      pc.createDataChannel('audio')
      wsRef.current = ws
      pcRef.current = pc

      const mediaSource = new MediaSource()
      const audio = ensureAudio()
      audio.src = URL.createObjectURL(mediaSource)

      let sb: SourceBuffer | null = null
      const queue: Uint8Array[] = []
      const pump = () => {
        if (!sb || sb.updating || !queue.length) return
        sb.appendBuffer(queue.shift()!)
      }

      mediaSource.addEventListener('sourceopen', () => {
        sb = mediaSource.addSourceBuffer('audio/mpeg')
        sb.addEventListener('updateend', pump)
        pump()
      })

      pc.ondatachannel = ({ channel }) => {
        channel.binaryType = 'arraybuffer'
        channel.onmessage = ({ data }) => {
          queue.push(new Uint8Array(data as ArrayBuffer))
          pump()
        }
      }

      ws.onopen = async () => {
        const offer = await pc.createOffer()
        await pc.setLocalDescription(offer)
        await waitIceComplete(pc)

        ws.send(
          JSON.stringify({
            sdp: pc.localDescription!.sdp,
            text
          })
        )
      }

      ws.onmessage = async ({ data }) => {
        await pc.setRemoteDescription(new RTCSessionDescription(JSON.parse(data as string)))
      }

      ws.onerror = (err) => {
        console.error(err)
        stop()
      }
      ws.onclose = stop
    },
    [wsURL, stop]
  )

  return { speak, stop }
}
