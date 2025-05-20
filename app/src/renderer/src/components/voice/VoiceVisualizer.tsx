// VoiceVisualizer.tsx
import React, { useMemo, useRef } from 'react'
import { Canvas, useFrame } from '@react-three/fiber'
import { EffectComposer, Bloom } from '@react-three/postprocessing'
import { OrbitControls } from '@react-three/drei'
import * as THREE from 'three'

export interface VoiceVisualizerProps {
  visualState: 0 | 1 | 2
  getFreqData: () => Uint8Array
  className?: string
  particleCount?: number
}

export default function VoiceVisualizer({
  visualState,
  getFreqData,
  className,
  particleCount = 12000
}: VoiceVisualizerProps) {
  return (
    <Canvas
      className={className}
      camera={{ position: [0, 0, 5], fov: 60 }}
      gl={{ antialias: true }}
    >
      <Particles
        visualState={visualState}
        particleCount={particleCount}
        getFreqData={getFreqData}
      />
      <EffectComposer>
        <Bloom luminanceThreshold={0.3} intensity={1.2} />
      </EffectComposer>
      <OrbitControls enableZoom={false} enablePan={false} />
    </Canvas>
  )
}

function Particles({
  visualState,
  particleCount,
  getFreqData
}: {
  visualState: 0 | 1 | 2
  particleCount: number
  getFreqData: () => Uint8Array
}) {
  const mesh = useRef<THREE.Points>(null!)
  const smoothedFFT = useRef(new Float32Array(256))

  // FFT texture
  const fftTex = useMemo(() => {
    const data = new Uint8Array(256 * 4)
    const tex = new THREE.DataTexture(data, 256, 1, THREE.RGBAFormat)
    tex.minFilter = THREE.NearestFilter
    tex.magFilter = THREE.NearestFilter
    tex.needsUpdate = true
    return tex
  }, [])

  // Geometry
  const geometry = useMemo(() => {
    const pos = new Float32Array(particleCount * 3)
    const ids = new Float32Array(particleCount)
    for (let i = 0; i < particleCount; i++) {
      const r = Math.cbrt(Math.random()) * 1.3
      const theta = Math.random() * Math.PI * 2
      const phi = Math.acos(2 * Math.random() - 1)
      const x = r * Math.sin(phi) * Math.cos(theta)
      const y = r * Math.sin(phi) * Math.sin(theta)
      const z = r * Math.cos(phi)
      pos.set([x, y, z], i * 3)
      ids[i] = i % 256
    }
    const g = new THREE.BufferGeometry()
    g.setAttribute('position', new THREE.BufferAttribute(pos, 3))
    g.setAttribute('aId', new THREE.BufferAttribute(ids, 1))
    return g
  }, [particleCount])

  // Theme detection
  const isDark = useMemo(() => window.matchMedia('(prefers-color-scheme: dark)').matches, [])

  // Light-mode contrasting colors
  const idleColDark = new THREE.Color(0.2, 0.4, 0.6)
  const speakColDark = new THREE.Color(1.0, 0.5, 0.1)
  const idleColLight = new THREE.Color(0.0, 0.0, 0.0) // pure black
  const speakColLight = new THREE.Color(0.8, 0.0, 0.0) // dark red

  // Shader material
  const material = useMemo(() => {
    return new THREE.ShaderMaterial({
      uniforms: {
        uFFT: { value: fftTex },
        uState: { value: 0 },
        uTime: { value: 0 },
        uIdleCol: { value: isDark ? idleColDark : idleColLight },
        uSpeakCol: { value: isDark ? speakColDark : speakColLight }
      },
      vertexShader,
      fragmentShader,
      transparent: true,
      depthWrite: false,
      blending: isDark ? THREE.AdditiveBlending : THREE.NormalBlending
    })
  }, [fftTex, isDark])

  // Animation loop
  useFrame(({ clock }, delta) => {
    // update FFT texture
    const fft = getFreqData()
    const sm = smoothedFFT.current
    const data = fftTex.image.data as Uint8Array
    for (let i = 0; i < 256; i++) {
      sm[i] = THREE.MathUtils.lerp(sm[i], fft[i], 0.05)
      const v = sm[i]
      const off = i * 4
      data[off] = data[off + 1] = data[off + 2] = v
      data[off + 3] = 255
    }
    fftTex.needsUpdate = true

    // smoothly lerp state & time
    material.uniforms.uState.value = THREE.MathUtils.lerp(
      material.uniforms.uState.value,
      visualState,
      delta * 2.0
    )
    material.uniforms.uTime.value = clock.elapsedTime
  })

  return <points ref={mesh} geometry={geometry} material={material} />
}

// ——— VERTEX SHADER ———
const vertexShader = /* glsl */ `
uniform sampler2D uFFT;
uniform float uState;
uniform float uTime;
uniform vec3  uIdleCol;
uniform vec3  uSpeakCol;
attribute float aId;
varying vec3 vColor;

float bellSmooth(float x, float c, float w) {
  return 1.0 - smoothstep(0.0, w, abs(x - c));
}

void main() {
  vec3 p0 = position;
  vec3 dir = normalize(p0);

  // gentle swirl mapped 0.1 → 0.2
  float swirl = mix(0.1, 0.2, uState * 0.5);
  float angle = uTime * swirl;
  mat2 rot = mat2(cos(angle), -sin(angle), sin(angle), cos(angle));
  vec3 p = p0;
  p.xz = rot * p.xz;

  // idle breathing
  float wIdle = bellSmooth(uState, 0.0, 1.5);
  p += dir * sin(uTime*0.8 + aId*0.1) * 0.05 * wIdle;

  // speaking expansion
  float amp = texture2D(uFFT, vec2(aId/256.0,0.0)).r;
  float wSpeak = bellSmooth(uState, 2.0, 1.0);
  float expansion = pow(amp,1.5) * 1.0;
  p = mix(p, dir*(length(p0)+expansion), wSpeak);

  // size
  gl_PointSize = 2.0 + wIdle*1.5 + amp*8.0*wSpeak;

  // mix between the passed-in colors
  vColor = mix(uIdleCol, uSpeakCol, wSpeak);

  gl_Position = projectionMatrix * modelViewMatrix * vec4(p,1.0);
}`

// ——— FRAGMENT SHADER ———
const fragmentShader = /* glsl */ `
varying vec3 vColor;
void main() {
  float d = length(gl_PointCoord - 0.5);
  float alpha = smoothstep(0.5, 0.0, d);
  gl_FragColor = vec4(vColor, alpha);
}`
