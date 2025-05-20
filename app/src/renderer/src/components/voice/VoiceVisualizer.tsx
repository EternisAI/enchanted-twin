// VoiceVisualizer.tsx - redesigned particle visualiser
import React, { useMemo, useRef } from 'react'
import { Canvas, useFrame } from '@react-three/fiber'
import { OrbitControls } from '@react-three/drei'
import { EffectComposer, Bloom } from '@react-three/postprocessing'
import * as THREE from 'three'

export interface VoiceVisualizerProps {
  visualState: 0 | 1 | 2 // 0=standby,1=loading,2=speaking
  getFreqData: () => Uint8Array
  className?: string
  particleCount?: number
}

export default function VoiceVisualizer({
  visualState,
  getFreqData,
  className,
  particleCount = 15000
}: VoiceVisualizerProps) {
  return (
    <Canvas
      className={className}
      camera={{ position: [0, 0, 6], fov: 55 }}
      gl={{ antialias: true }}
    >
      <Particles
        visualState={visualState}
        particleCount={particleCount}
        getFreqData={getFreqData}
      />
      <EffectComposer>
        <Bloom luminanceThreshold={0.25} intensity={1.5} />
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

  // --- FFT texture ---
  const fftTex = useMemo(() => {
    const data = new Uint8Array(256 * 4)
    const tex = new THREE.DataTexture(data, 256, 1, THREE.RGBAFormat)
    tex.minFilter = THREE.NearestFilter
    tex.magFilter = THREE.NearestFilter
    tex.needsUpdate = true
    return tex
  }, [])

  // --- geometry ---
  const geometry = useMemo(() => {
    const pos = new Float32Array(particleCount * 3)
    const ids = new Float32Array(particleCount)
    for (let i = 0; i < particleCount; i++) {
      // spread inside unit sphere
      const r = Math.cbrt(Math.random()) * 1.2
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

  // color theme
  const isDark = useMemo(
    () => window.matchMedia('(prefers-color-scheme: dark)').matches,
    []
  )
  const idleColDark = new THREE.Color('#2356ff')
  const loadColDark = new THREE.Color('#9b59b6')
  const speakColDark = new THREE.Color('#ffae00')
  const idleColLight = new THREE.Color('#000000')
  const loadColLight = new THREE.Color('#6c2bd9')
  const speakColLight = new THREE.Color('#e63946')

  // --- material ---
  const material = useMemo(() => {
    return new THREE.ShaderMaterial({
      uniforms: {
        uFFT: { value: fftTex },
        uState: { value: 0 },
        uTime: { value: 0 },
        uIdleCol: { value: isDark ? idleColDark : idleColLight },
        uLoadCol: { value: isDark ? loadColDark : loadColLight },
        uSpeakCol: { value: isDark ? speakColDark : speakColLight }
      },
      vertexShader,
      fragmentShader,
      transparent: true,
      depthWrite: false,
      blending: THREE.AdditiveBlending
    })
  }, [fftTex, isDark])

  // --- animation loop ---
  useFrame(({ clock }, delta) => {
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

    material.uniforms.uState.value = THREE.MathUtils.lerp(
      material.uniforms.uState.value,
      visualState,
      delta * 3
    )
    material.uniforms.uTime.value = clock.elapsedTime
  })

  return <points ref={mesh} geometry={geometry} material={material} />
}

// --- shaders ---
const vertexShader = /* glsl */ `
uniform sampler2D uFFT;
uniform float uState;
uniform float uTime;
uniform vec3  uIdleCol;
uniform vec3  uLoadCol;
uniform vec3  uSpeakCol;
attribute float aId;
varying vec3 vColor;
varying float vAlpha;

float interp(float a,float b,float t){return mix(a,b,smoothstep(0.,1.,t));}

void main(){
  vec3 p = position;
  float amp = texture2D(uFFT, vec2(aId/256.0,0.0)).r / 255.0;

  // spin speeds for states
  float s1 = interp(0.1, 1.0, clamp(uState,0.,1.));
  float s2 = interp(0.3, 0.5, clamp(uState-1.,0.,1.));
  float speed = s1 + s2;
  float angle = uTime * speed;
  mat2 rot = mat2(cos(angle), -sin(angle), sin(angle), cos(angle));
  p.xz = rot * p.xz;

  // subtle vertical wave
  p.y += sin(uTime*0.6 + aId*0.05) * 0.1 * (0.5 + uState);

  // expansion when speaking
  if(uState>1.5){
    p += normalize(p) * amp * 2.0;
  }

  gl_PointSize = 1.5 + amp*10.0*step(1.5,uState);
  vAlpha = 0.6 + amp*0.4*step(1.5,uState);

  vec3 cIdle = uIdleCol;
  vec3 cLoad = uLoadCol;
  vec3 cSpeak = uSpeakCol;
  vec3 col = mix(cIdle,cLoad, clamp(uState,0.,1.));
  col = mix(col,cSpeak, clamp(uState-1.,0.,1.));
  col = mix(col,cSpeak, amp*step(1.5,uState));
  vColor = col;

  gl_Position = projectionMatrix * modelViewMatrix * vec4(p,1.0);
}
`

const fragmentShader = /* glsl */ `
varying vec3 vColor;
varying float vAlpha;
void main(){
  float d = length(gl_PointCoord - 0.5);
  float a = smoothstep(0.5,0.0,d) * vAlpha;
  gl_FragColor = vec4(vColor, a);
}
`
