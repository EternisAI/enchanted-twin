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
  // Simple fill container without affecting layout
  return (
    <div
      className={className}
      style={{ position: 'relative', width: '100%', height: '100%', overflow: 'hidden' }}
    >
      <Canvas
        style={{ position: 'absolute', top: 0, left: 0, right: 0, bottom: 0 }}
        camera={{ position: [0, 0, 6], fov: 60 }}
        gl={{ antialias: true }}
      >
        <CubeParticles
          visualState={visualState}
          particleCount={particleCount}
          getFreqData={getFreqData}
        />
        <EffectComposer>
          <Bloom luminanceThreshold={0.3} intensity={1.2} />
        </EffectComposer>
        <OrbitControls enableZoom={false} enablePan={false} />
      </Canvas>
    </div>
  )
}

function CubeParticles({
  visualState,
  particleCount,
  getFreqData
}: {
  visualState: 0 | 1 | 2
  particleCount: number
  getFreqData: () => Uint8Array
}) {
  const mesh = useRef<THREE.Points>(null!)
  const prevState = useRef(visualState)
  const rotating = useRef(false)
  const rotStart = useRef(0)
  const smoothedFFT = useRef(new Float32Array(256))

  const fftTex = useMemo(() => {
    const data = new Uint8Array(256 * 4)
    const tex = new THREE.DataTexture(data, 256, 1, THREE.RGBAFormat)
    tex.minFilter = THREE.NearestFilter
    tex.magFilter = THREE.NearestFilter
    tex.needsUpdate = true
    return tex
  }, [])

  const geometry = useMemo(() => {
    const home = new Float32Array(particleCount * 3)
    const ids = new Float32Array(particleCount)
    for (let i = 0; i < particleCount; i++) {
      const half = 1.0
      const x = (Math.random() * 2 - 1) * half
      const y = (Math.random() * 2 - 1) * half
      const z = (Math.random() * 2 - 1) * half
      home.set([x, y, z], i * 3)
      ids[i] = i % 256
    }
    const g = new THREE.BufferGeometry()
    g.setAttribute('position', new THREE.BufferAttribute(home, 3))
    g.setAttribute('aHome', new THREE.BufferAttribute(home, 3))
    g.setAttribute('aId', new THREE.BufferAttribute(ids, 1))
    return g
  }, [particleCount])

  const isDark = useMemo(() => window.matchMedia('(prefers-color-scheme: dark)').matches, [])
  const idleCol = isDark ? new THREE.Color(0.2, 0.4, 0.6) : new THREE.Color(0, 0, 0)
  const loadCol = isDark ? new THREE.Color(0.5, 0.7, 1.0) : new THREE.Color(0.3, 0.6, 0.9)
  const speakCol = isDark ? new THREE.Color(1.0, 0.5, 0.1) : new THREE.Color(0.8, 0, 0)

  const material = useMemo(
    () =>
      new THREE.ShaderMaterial({
        uniforms: {
          uFFT: { value: fftTex },
          uState: { value: 0 },
          uTime: { value: 0 },
          uIdleCol: { value: idleCol },
          uLoadCol: { value: loadCol },
          uSpeakCol: { value: speakCol }
        },
        vertexShader,
        fragmentShader,
        transparent: true,
        depthWrite: false,
        blending: isDark ? THREE.AdditiveBlending : THREE.NormalBlending
      }),
    [fftTex, idleCol, loadCol, speakCol, isDark]
  )

  useFrame(({ clock }, delta) => {
    const fft = getFreqData()
    const sm = smoothedFFT.current
    const data = fftTex.image.data as Uint8Array
    for (let i = 0; i < 256; i++) {
      sm[i] = THREE.MathUtils.lerp(sm[i], fft[i], 0.1)
      const v = sm[i]
      const idx = i * 4
      data[idx] = data[idx + 1] = data[idx + 2] = v
      data[idx + 3] = 255
    }
    fftTex.needsUpdate = true

    if (visualState === 2 && prevState.current !== 2) {
      rotating.current = true
      rotStart.current = clock.elapsedTime
    }
    prevState.current = visualState

    if (rotating.current) {
      const t = (clock.elapsedTime - rotStart.current) / 0.3
      const prog = THREE.MathUtils.clamp(t, 0, 1)
      mesh.current.rotation.z = prog * Math.PI * 2
      material.uniforms.uState.value = THREE.MathUtils.damp(
        material.uniforms.uState.value,
        1,
        5,
        delta
      )
      if (prog >= 1) rotating.current = false
    } else {
      mesh.current.rotation.z = 0
      material.uniforms.uState.value = THREE.MathUtils.damp(
        material.uniforms.uState.value,
        visualState,
        5,
        delta
      )
    }

    material.uniforms.uTime.value = clock.elapsedTime
  })

  return <points ref={mesh} geometry={geometry} material={material} />
}

const vertexShader = /* glsl */ `
uniform sampler2D uFFT;
uniform float uState;
uniform float uTime;
uniform vec3 uIdleCol;
uniform vec3 uLoadCol;
uniform vec3 uSpeakCol;
attribute vec3 aHome;
attribute float aId;
varying vec3 vColor;

void main() {
  float wLoad = smoothstep(0.0, 1.0, uState) - smoothstep(1.0, 2.0, uState);
  float wSpeak = smoothstep(1.0, 2.0, uState);

  vec3 pBase = aHome;
  vec3 pSpeak = aHome + normalize(aHome) * (texture2D(uFFT, vec2(aId/256.0,0)).r * 1.0);
  vec3 p = mix(pBase, pSpeak, wSpeak);

  float swirlW = 1.0 - wSpeak;
  float ang = atan(p.y,p.x) + uTime*0.2*swirlW;
  float r = length(p.xy);
  p.xy = vec2(cos(ang),sin(ang))*r;

  float amp = texture2D(uFFT, vec2(aId/256.0,0)).r;
  gl_PointSize = 2.0 + amp*5.0*wSpeak;
  vColor = mix(mix(uIdleCol,uLoadCol,wLoad),uSpeakCol,wSpeak);
  gl_Position = projectionMatrix*modelViewMatrix*vec4(p,1.0);
}`

const fragmentShader = /* glsl */ `
varying vec3 vColor;
void main(){
  float d = length(gl_PointCoord-0.5);
  float a = smoothstep(0.5,0.0,d);
  gl_FragColor = vec4(vColor,a);
}`
