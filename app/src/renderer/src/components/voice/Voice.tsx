'use client'

import { useEffect, useState } from 'react'
import {
  LiveKitRoom,
  RoomAudioRenderer,
  VideoConference,
  useVoiceAssistant,
  BarVisualizer
} from '@livekit/components-react'
const LIVEKIT_URL = 'wss://jarvis-isbvwgjc.livekit.cloud' // or ws://localhost:7880
const TOKEN_ENDPOINT = 'http://localhost:8080/token'

export default function RoomLoader() {
  const [token, setToken] = useState<string>()

  useEffect(() => {
    fetch(`${TOKEN_ENDPOINT}`)
      .then((r) => r.json())
      .then(({ participantToken }) => setToken(participantToken))
      .catch(console.error)
  }, [])

  if (!token) return <p>connecting…</p>

  return <SimpleRoom url={LIVEKIT_URL} token={token} />
}

export function SimpleRoom({ url, token }: { url: string; token: string }) {
  return (
    <LiveKitRoom
      serverUrl={url}
      token={token}
      connectOptions={{ autoSubscribe: true }}
      data-lk-theme="default"
      video
      audio
    >
      <VideoConference />
      <RoomAudioRenderer />
      <Visualizer />
    </LiveKitRoom>
  )
}

function Visualizer() {
  const { state, audioTrack } = useVoiceAssistant() // state = listening | thinking | speaking
  return <BarVisualizer state={state} trackRef={audioTrack} barCount={5} />
}
