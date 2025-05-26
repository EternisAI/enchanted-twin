/**
 * VoiceVisualizer.tsx
 *
 * • Luxury-gold tool icons
 * • Smooth state transitions
 * • No halo on light theme
 * • Tool shape always shown ≥ 0.5 s, then fades smoothly
 */

import { useEffect, useMemo, useRef } from 'react'
import { Canvas, useFrame } from '@react-three/fiber'
import { EffectComposer, Bloom } from '@react-three/postprocessing'
import { OrbitControls } from '@react-three/drei'
import * as THREE from 'three'

/* ──────────── public props ──────────── */

export interface VoiceVisualizerProps {
  visualState: 0 | 1 | 2
  getFreqData: () => Uint8Array
  className?: string
  particleCount?: number
  assistantTextMessage?: string
  toolUrl?: string
}

/* ──────────── main component ──────────── */

export default function VoiceVisualizer({
  visualState,
  getFreqData,
  className,
  particleCount = 12_000,
  assistantTextMessage,
  toolUrl
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
          tool={toolUrl}
        />
        <EffectComposer>
          <Bloom luminanceThreshold={0.3} intensity={1.2} />
        </EffectComposer>
        <OrbitControls enableZoom={false} enablePan={false} />
      </Canvas>

      {assistantTextMessage && (
        <div className="absolute bottom-28 left-1/2 -translate-x-1/2 max-w-xl text-center text-primary text-md overflow-hidden">
          {assistantTextMessage}
        </div>
      )}
    </div>
  )
}

/* ──────────── helper types ──────────── */
type ShapeGenSync = (count: number) => Float32Array
type ShapeGenAsync = (count: number) => Promise<Float32Array>
type ShapeGen = ShapeGenSync | ShapeGenAsync

const genFromImage =
  (url: string, alpha = 128): ShapeGenAsync =>
  async (count) => {
    const img = await new Promise<HTMLImageElement>((ok, err) => {
      const im = new Image()
      im.crossOrigin = 'anonymous'
      im.onload = () => ok(im)
      im.onerror = err
      im.src = url
    })
    const { width: w, height: h } = img
    const c = document.createElement('canvas')
    c.width = w
    c.height = h
    const ctx = c.getContext('2d')!
    ctx.drawImage(img, 0, 0)
    const { data } = ctx.getImageData(0, 0, w, h)

    const pts: [number, number][] = []
    for (let y = 0; y < h; y++)
      for (let x = 0; x < w; x++) if (data[(y * w + x) * 4 + 3] > alpha) pts.push([x, y])

    const out = new Float32Array(count * 3)
    const maxDim = Math.max(w, h)
    for (let i = 0; i < count; i++) {
      const [px, py] = pts[(Math.random() * pts.length) | 0]
      const nx = (px - w / 2) / (maxDim / 2)
      const ny = -(py - h / 2) / (maxDim / 2)
      out.set([nx, ny, 0], i * 3)
    }
    return out
  }

/* ──────────── particle system ──────────── */

function CubeParticles({
  visualState,
  particleCount,
  getFreqData,
  tool
}: {
  visualState: 0 | 1 | 2
  particleCount: number
  getFreqData: () => Uint8Array
  tool?: string
}) {
  /* refs ------------------------------------------------ */
  const mesh = useRef<THREE.Points>(null!)
  const sm = useRef(new Float32Array(256))
  const homePosRef = useRef<Float32Array>(undefined) // original cube positions
  const stateSmooth = useRef<number>(visualState)
  const toolBlend = useRef(0)
  const currentTool = useRef<string | undefined>(undefined) // what's currently shown
  const displayTimer = useRef(0) // how long current shape has been shown
  const pendingTool = useRef<string | undefined>(undefined) // what should be shown next
  const isTransitioning = useRef(false)
  const MIN_DISPLAY_TIME = 0.5 // minimum seconds to display tool

  /* ---------- FFT texture ---------- */
  const fftTex = useMemo(() => {
    const tex = new THREE.DataTexture(new Uint8Array(256 * 4), 256, 1, THREE.RGBAFormat)
    tex.minFilter = tex.magFilter = THREE.NearestFilter
    tex.needsUpdate = true
    return tex
  }, [])

  /* ---------- geometry ---------- */
  const geometry = useMemo(() => {
    const home = new Float32Array(particleCount * 3)
    const ids = new Float32Array(particleCount)
    for (let i = 0; i < particleCount; i++) {
      home.set(
        [
          (Math.random() * 2 - 1) * 0.5,
          (Math.random() * 2 - 1) * 0.5,
          (Math.random() * 2 - 1) * 0.5
        ],
        i * 3
      )
      ids[i] = i % 256
    }
    homePosRef.current = home
    const g = new THREE.BufferGeometry()
    g.setAttribute('position', new THREE.BufferAttribute(home, 3))
    g.setAttribute('aHome', new THREE.BufferAttribute(home, 3))
    g.setAttribute('aTool', new THREE.BufferAttribute(home.slice(), 3))
    g.setAttribute('aId', new THREE.BufferAttribute(ids, 1))
    return g
  }, [particleCount])

  /* ---------- load tool shape into buffer ---------- */
  const loadToolShape = (toolId: string | undefined) => {
    if (!toolId) {
      // Keep the current aTool buffer - don't modify it
      // The shader will blend to home positions automatically
      return
    }

    // choose generator
    let gen: ShapeGen
    if (toolId.startsWith('image:') || /^(https?:|data:image)/.test(toolId)) {
      gen = genFromImage(toolId.replace(/^image:/, ''))
    } else {
      gen = () => geometry.getAttribute('aHome').array as Float32Array
    }

    const apply = (arr: Float32Array) => {
      ;(geometry.getAttribute('aTool') as THREE.BufferAttribute).copyArray(arr).needsUpdate = true
    }
    const maybe = gen(particleCount)
    maybe instanceof Promise ? maybe.then(apply).catch(console.error) : apply(maybe)
  }

  /* ---------- handle tool changes ---------- */
  useEffect(() => {
    // Update pending tool whenever prop changes
    pendingTool.current = tool
  }, [tool])

  /* ---------- colours ---------- */
  const { isDarkTheme, idleCol, loadCol, speakCol } = useMemo(() => {
    const dark = window.matchMedia('(prefers-color-scheme: dark)').matches
    return {
      isDarkTheme: dark,
      idleCol: dark ? new THREE.Color(0.2, 0.4, 0.6) : new THREE.Color(0, 0, 0),
      loadCol: new THREE.Color(0xd4af37),
      speakCol: dark ? new THREE.Color(1, 0.5, 0.1) : new THREE.Color(0.8, 0, 0)
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
          uSpeakCol: { value: speakCol },
          uToolBlend: { value: toolBlend.current }
        },
        vertexShader,
        fragmentShader,
        transparent: true,
        depthWrite: false,
        alphaTest: 0.05,
        blending: isDarkTheme ? THREE.AdditiveBlending : THREE.NormalBlending
      }),
    [fftTex, idleCol, loadCol, speakCol, isDarkTheme]
  )

  /* ---------- per-frame updates ---------- */
  useFrame(({ clock }, delta) => {
    /* smooth state transitions */
    stateSmooth.current = THREE.MathUtils.damp(stateSmooth.current, visualState, 4, delta)
    material.uniforms.uState.value = stateSmooth.current

    /* update display timer */
    if (currentTool.current !== undefined) {
      displayTimer.current += delta
    }

    /* check if we need to transition */
    const needsTransition = pendingTool.current !== currentTool.current
    const canTransition =
      displayTimer.current >= MIN_DISPLAY_TIME || currentTool.current === undefined

    if (needsTransition && canTransition && !isTransitioning.current) {
      isTransitioning.current = true
    }

    /* handle transitions */
    if (isTransitioning.current) {
      const targetBlend = 0
      toolBlend.current = THREE.MathUtils.damp(toolBlend.current, targetBlend, 5, delta)

      // Once faded out, switch to new tool
      if (toolBlend.current < 0.01) {
        currentTool.current = pendingTool.current
        displayTimer.current = 0

        if (currentTool.current !== undefined) {
          // Load and show new tool
          loadToolShape(currentTool.current)
          isTransitioning.current = false
        } else {
          // We're going back to cube, keep transitioning false
          isTransitioning.current = false
        }
      }
    } else {
      // Not transitioning - maintain proper blend
      const targetBlend = currentTool.current !== undefined ? 1 : 0
      toolBlend.current = THREE.MathUtils.damp(toolBlend.current, targetBlend, 5, delta)
    }

    material.uniforms.uToolBlend.value = toolBlend.current

    /* gentle bob */
    if (mesh.current) {
      mesh.current.position.y = 0.05 * Math.sin(clock.elapsedTime * 0.8)
    }

    /* FFT → texture */
    const fft = getFreqData()
    const img = fftTex.image.data as Uint8Array
    for (let i = 0; i < 256; i++) {
      sm.current[i] = THREE.MathUtils.lerp(sm.current[i], fft[i] ?? 0, 0.25)
      const j = i * 4
      img[j] = img[j + 1] = img[j + 2] = sm.current[i]
      img[j + 3] = 255
    }
    fftTex.needsUpdate = true
    material.uniforms.uTime.value = clock.elapsedTime
  })

  return <points ref={mesh} geometry={geometry} material={material} />
}

/* ──────────── shaders (unchanged) ──────────── */

const vertexShader = /* glsl */ `
uniform sampler2D uFFT;
uniform float     uState;
uniform float     uTime;
uniform vec3      uIdleCol;
uniform vec3      uLoadCol;
uniform vec3      uSpeakCol;
uniform float     uToolBlend;

attribute vec3  aHome;
attribute vec3  aTool;
attribute float aId;
varying   vec3  vColor;

void main(){
  // 1. Determine base particle position by blending between home and tool shapes
  vec3 p_base = mix(aHome, aTool, uToolBlend);
  vec3 p = p_base;

  // 2. Apply FFT displacement based on uState
  float amp = texture2D(uFFT, vec2((aId + 0.5) / 256.0, 0.5)).r / 255.0;

  // Ensure displacement_dir is valid even if p_base is zero vector
  vec3 displacement_dir = length(p_base) > 0.0001 ? normalize(p_base) : vec3(0.0, 0.0, 1.0);
  if (length(p_base) < 0.0001 && length(aHome) > 0.0001) {
      displacement_dir = normalize(aHome);
  }

  float idle_strength = 0.3;
  float load_strength = 0.8;
  float speak_strength = 2.5;
  float displacement_strength;

  if (uState < 1.0) {
      displacement_strength = mix(idle_strength, load_strength, uState);
  } else {
      displacement_strength = mix(load_strength, speak_strength, uState - 1.0);
  }
  displacement_strength = max(0.0, displacement_strength);

  p += displacement_dir * amp * displacement_strength;

  // 3. Add jitter based on overall activity
  float activityLevel = smoothstep(0.0, 0.1, uState);
  float jitterAmount = activityLevel * 0.02;
  p.xy += vec2(
    sin(uTime * 0.8 + aId * 12.0) * jitterAmount,
    cos(uTime * 0.6 + aId * 17.0) * jitterAmount
  );

  // 4. Swirl effect
  float speakReductionForSwirl = smoothstep(1.0, 1.8, uState);
  float swirlActivation = uToolBlend * (1.0 - speakReductionForSwirl);
  if (length(p.xy) > 0.01 && swirlActivation > 0.01) {
    float ang = atan(p.y, p.x) + uTime * 0.25 * swirlActivation;
    float r   = length(p.xy);
    p.xy      = vec2(cos(ang), sin(ang)) * r;
  }

  // 5. Point size
  float speakFactorForPointSize = smoothstep(1.0, 2.0, uState);
  gl_PointSize = max(2.0, 2.0 + amp * 255.0 * 5.0 * speakFactorForPointSize);
  gl_PointSize = mix(gl_PointSize, max(1.0, gl_PointSize * 0.5), 1.0 - uToolBlend);

  // 6. Color determination
  vec3 color = uIdleCol;
  color = mix(color, uLoadCol, uToolBlend);
  float speakColorFactor = smoothstep(1.0, 2.0, uState);
  color = mix(color, uSpeakCol, speakColorFactor);

  vColor = color;
  gl_Position = projectionMatrix * modelViewMatrix * vec4(p, 1.0);
}
`

const fragmentShader = /* glsl */ `
varying vec3 vColor;
void main(){
  float d = length(gl_PointCoord - 0.5);
  float a = smoothstep(0.5, 0.35, d);
  if (a < 0.01) discard;
  gl_FragColor = vec4(vColor, a);
}
`
