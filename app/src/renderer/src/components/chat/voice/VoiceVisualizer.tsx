import { useMemo, useRef, useState } from 'react'
import { Canvas, useFrame } from '@react-three/fiber'
import { EffectComposer, Bloom } from '@react-three/postprocessing'
import { OrbitControls } from '@react-three/drei'
import * as THREE from 'three'

export interface VoiceVisualizerProps {
  visualState: 0 | 1 | 2
  getFreqData: () => Uint8Array
  className?: string
  particleCount?: number
  assistantTextMessage?: string
}

export default function VoiceVisualizer({
  visualState,
  getFreqData,
  className,
  particleCount = 12_000,
  assistantTextMessage
}: VoiceVisualizerProps) {
  return (
    <div
      className={className}
      style={{ position: 'relative', width: '100%', height: '100%', overflow: 'hidden' }}
    >
      <Canvas
        style={{ position: 'absolute', inset: 0 }}
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

      {/* floating assistant transcript */}
      {assistantTextMessage && (
        <div className="absolute bottom-28 left-1/2 -translate-x-1/2 text-center text-primary text-md">
          {assistantTextMessage}
        </div>
      )}
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
  const prev = useRef(visualState)
  const rotRef = useRef(false)
  const rotT0 = useRef(0)
  const sm = useRef(new Float32Array(256))
  const noChangeCounter = useRef(0)

  const fftTex = useMemo(() => {
    const tex = new THREE.DataTexture(new Uint8Array(256 * 4), 256, 1, THREE.RGBAFormat)
    tex.minFilter = tex.magFilter = THREE.NearestFilter
    tex.needsUpdate = true // Initialize with needsUpdate = true
    return tex
  }, [])

  /* ---------- particle geometry ---------- */
  const geometry = useMemo(() => {
    const home = new Float32Array(particleCount * 3)
    const ids = new Float32Array(particleCount)
    for (let i = 0; i < particleCount; i++) {
      const x = (Math.random() * 2 - 1) * 0.5
      const y = (Math.random() * 2 - 1) * 0.5
      const z = (Math.random() * 2 - 1) * 0.5
      home.set([x, y, z], i * 3)
      ids[i] = i % 256
    }
    const g = new THREE.BufferGeometry()
    g.setAttribute('position', new THREE.BufferAttribute(home, 3))
    g.setAttribute('aHome', new THREE.BufferAttribute(home, 3))
    g.setAttribute('aId', new THREE.BufferAttribute(ids, 1))
    return g
  }, [particleCount])

  /* ---------- colours ---------- */
  const { isDarkTheme, idleCol, loadCol, speakCol } = useMemo(() => {
    const dark = window.matchMedia('(prefers-color-scheme: dark)').matches
    return {
      isDarkTheme: dark,
      idleCol: dark ? new THREE.Color(0.2, 0.4, 0.6) : new THREE.Color(0, 0, 0),
      loadCol: dark ? new THREE.Color(0.5, 0.7, 1.0) : new THREE.Color(0.3, 0.6, 0.9),
      speakCol: dark ? new THREE.Color(1.0, 0.5, 0.1) : new THREE.Color(0.8, 0.0, 0.0)
    }
  }, [])

  /* ---------- shader material ---------- */
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
        blending: isDarkTheme ? THREE.AdditiveBlending : THREE.NormalBlending
      }),
    [fftTex, idleCol, loadCol, speakCol, isDarkTheme]
  )

  /* ---------- per-frame update ---------- */
  const STUCK_THRESHOLD = 5.0 // Sum of smoothed FFT data (0-255 range per bin)
  const STUCK_FRAMES_LIMIT = 60 // Approx 1 second at 60fps

  useFrame(({ clock }, delta) => {
    /* update FFT texture */
    const fft = getFreqData()
    // console.log(fft) // DEBUG: Keep this for now if user wants to debug FFT data
    const img = fftTex.image.data as Uint8Array
    let currentFftSum = 0
    for (let i = 0; i < 256; i++) {
      // Ensure fft[i] is a number before using it in lerp
      const fftValue = typeof fft[i] === 'number' ? fft[i] : 0
      sm.current[i] = THREE.MathUtils.lerp(sm.current[i], fftValue, 0.25)
      const v = sm.current[i]
      currentFftSum += v
      const j = i * 4
      img[j] = img[j + 1] = img[j + 2] = v
      img[j + 3] = 255
    }
    fftTex.needsUpdate = true

    /* one-off spin when we first enter state 2 */
    if (visualState === 2 && prev.current !== 2) {
      rotRef.current = true
      rotT0.current = clock.elapsedTime
      noChangeCounter.current = 0 // Reset counter on state transition
    }
    prev.current = visualState

    if (rotRef.current) {
      const t = (clock.elapsedTime - rotT0.current) / 0.3
      const p = THREE.MathUtils.clamp(t, 0, 1)
      mesh.current.rotation.z = p * Math.PI * 2
      material.uniforms.uState.value = THREE.MathUtils.damp(
        material.uniforms.uState.value,
        2,
        5,
        delta
      )
      if (p >= 1) rotRef.current = false
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

    // Check for "stuck" state and attempt recovery
    if (
      visualState === 2 &&
      !rotRef.current && // After initial spin
      Math.abs(material.uniforms.uState.value - 2.0) < 0.01 // And shader is in state 2
    ) {
      if (currentFftSum < STUCK_THRESHOLD) {
        noChangeCounter.current++
      } else {
        noChangeCounter.current = 0 // Reset if we get good data
      }

      if (noChangeCounter.current > STUCK_FRAMES_LIMIT) {
        // console.warn('[VoiceVisualizer] FFT data seems stuck. Forcing texture and material refresh.')
        noChangeCounter.current = 0 // Reset counter
        if (sm.current && typeof sm.current.fill === 'function') {
          // Guard against potential issues
          sm.current.fill(0) // Explicitly reset smoothed FFT values
        }
      }
    } else {
      // Reset counter if not in the specific "stuck-prone" phase of state 2, or not in state 2 at all
      noChangeCounter.current = 0
    }
  })

  return <points ref={mesh} geometry={geometry} material={material} />
}

/* ───────────────────────────────── shaders ─────────────────────────────── */

const vertexShader = /* glsl */ `
uniform sampler2D uFFT;
uniform float     uState;
uniform float     uTime;
uniform vec3      uIdleCol;
uniform vec3      uLoadCol;
uniform vec3      uSpeakCol;

attribute vec3  aHome;
attribute float aId;
varying   vec3  vColor;

void main () {
  /* state blending weights */
  float wLoad  = smoothstep(0.0, 1.0, uState) - smoothstep(1.0, 2.0, uState);
  float wSpeak = smoothstep(1.0, 2.0, uState);

  /* displacement driven by FFT bin */
  float amp     = texture2D(uFFT, vec2( (aId + 0.5) / 256.0, 0.0 )).r;  // centre-sample
  vec3  pSpeak  = aHome + normalize(aHome) * amp * 3.0;                 // ×3 for punch
  vec3  p       = mix(aHome, pSpeak, wSpeak);

  /* subtle swirl while loading */
  float swirlW  = 1.0 - wSpeak;
  float ang     = atan(p.y, p.x) + uTime * 0.25 * swirlW;
  float r       = length(p.xy);
  p.xy          = vec2(cos(ang), sin(ang)) * r;

  /* render attributes */
  gl_PointSize = max(2.0, 2.0 + amp * 5.0 * wSpeak);
  vColor       = mix( mix(uIdleCol, uLoadCol, wLoad), uSpeakCol, wSpeak );
  gl_Position  = projectionMatrix * modelViewMatrix * vec4(p, 1.0);
}
`

const fragmentShader = /* glsl */ `
varying vec3 vColor;
void main () {
  float d = length(gl_PointCoord - 0.5);
  float a = smoothstep(0.5, 0.0, d);
  gl_FragColor = vec4(vColor, a);
}
`
