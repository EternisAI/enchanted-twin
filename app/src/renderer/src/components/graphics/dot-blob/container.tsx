import { Canvas } from '@react-three/fiber'
import App from './dot-blob-r3f'

export const DotBlobContainer = () => (
  <Canvas camera={{ fov: 25, position: [0, 0, 6] }}>
    <App />
  </Canvas>
)
