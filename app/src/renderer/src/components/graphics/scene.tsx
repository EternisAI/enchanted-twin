import { Suspense } from 'react'
import { Canvas } from '@react-three/fiber'
import { OrbitControls } from '@react-three/drei'
import Scene from './brain-r3f'

export default function App() {
  return (
    <Suspense fallback={null}>
      <Canvas shadows flat linear>
        <Scene />
        <OrbitControls />
      </Canvas>
    </Suspense>
  )
}
