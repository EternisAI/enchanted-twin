/**
 * VoiceVisualizer.tsx  – complete, ready-to-paste file
 *
 * Smooth state transitions, luxury-gold tool colour, and no white halo in light-mode.
 */

import { useEffect, useMemo, useRef } from 'react'
import { Canvas, useFrame } from '@react-three/fiber'
import { EffectComposer, Bloom } from '@react-three/postprocessing'
import { OrbitControls } from '@react-three/drei'
import * as THREE from 'three'

/* ─────────────────── public props ─────────────────── */

export interface VoiceVisualizerProps {
  /** 0 = idle   1 = tool morph / loading   2 = speaking */
  visualState: 0 | 1 | 2
  /** supplies 256-bin FFT data each frame */
  getFreqData: () => Uint8Array
  className?: string
  particleCount?: number
  assistantTextMessage?: string
  toolUrl?: string
}

/* ─────────────────── main component ─────────────────── */

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
        <div className="absolute bottom-28 left-1/2 -translate-x-1/2 text-center text-primary text-md">
          {assistantTextMessage}
        </div>
      )}
    </div>
  )
}

/* ─────────────────── helper types ─────────────────── */

type ShapeGenSync = (count: number) => Float32Array
type ShapeGenAsync = (count: number) => Promise<Float32Array>
type ShapeGen = ShapeGenSync | ShapeGenAsync

/* ─────────────────── shape generators ─────────────────── */

/* magnifying-glass outline (for `perplexity_ask`) */
const genMagnifyingGlass: ShapeGenSync = (n) => {
  const out = new Float32Array(n * 3)
  const ringN = Math.floor(n * 0.7)
  const rOuter = 0.8,
    rInner = 0.55
  for (let i = 0; i < ringN; i++) {
    const t = Math.random() * Math.PI * 2
    const r = THREE.MathUtils.lerp(rInner, rOuter, Math.random())
    out.set([Math.cos(t) * r, Math.sin(t) * r, 0], i * 3)
  }
  const start = new THREE.Vector2(rOuter * Math.SQRT1_2, -rOuter * Math.SQRT1_2)
  const end = start.clone().add(new THREE.Vector2(0, -0.8))
  for (let i = ringN; i < n; i++) {
    const p = start.clone().lerp(end, Math.random())
    out.set([p.x, p.y, 0], i * 3)
  }
  return out
}

/* simple picture-frame outline (for `generate_image`) */
const genPictureFrame: ShapeGenSync = (n) => {
  const out = new Float32Array(n * 3)
  for (let i = 0; i < n; i++) {
    const edge = i % 4
    const t = Math.random() * 2 - 1
    if (edge === 0) out.set([t, 1, 0], i * 3)
    else if (edge === 1) out.set([1, t, 0], i * 3)
    else if (edge === 2) out.set([t, -1, 0], i * 3)
    else out.set([-1, t, 0], i * 3)
  }
  return out
}

/* sample opaque pixels from a bitmap URL */
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

const TOOL_GENERATORS: Record<string, ShapeGen> = {
  perplexity_ask: genMagnifyingGlass,
  generate_image: genPictureFrame
}

/* ─────────────────── particle system ─────────────────── */

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
  /* refs ------------------------------------------------- */
  const mesh = useRef<THREE.Points>(null!)
  const sm = useRef(new Float32Array(256)) // smoothed FFT bins
  const stateSmoothRef = useRef<number>(visualState) // damped visual state
  const toolBlend = useRef(tool ? 1 : 0) // damped tool morph
  const currentToolId = useRef<string | undefined>(undefined)

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
    const g = new THREE.BufferGeometry()
    g.setAttribute('position', new THREE.BufferAttribute(home, 3))
    g.setAttribute('aHome', new THREE.BufferAttribute(home, 3))
    g.setAttribute('aTool', new THREE.BufferAttribute(home.slice(), 3))
    g.setAttribute('aId', new THREE.BufferAttribute(ids, 1))
    return g
  }, [particleCount])

  /* ---------- handle tool changes ---------- */
  useEffect(() => {
    if (tool === currentToolId.current) return
    currentToolId.current = tool

    let gen: ShapeGen
    if (!tool) {
      gen = () => geometry.getAttribute('aHome').array as Float32Array
    } else if (tool.startsWith('image:') || /^(https?:|data:image)/.test(tool)) {
      gen = genFromImage(tool.replace(/^image:/, ''))
    } else if (TOOL_GENERATORS[tool]) {
      gen = TOOL_GENERATORS[tool]
    } else {
      gen = () => geometry.getAttribute('aHome').array as Float32Array
    }

    const apply = (arr: Float32Array) => {
      const attr = geometry.getAttribute('aTool') as THREE.BufferAttribute
      attr.copyArray(arr)
      attr.needsUpdate = true
    }
    const maybe = gen(particleCount)
    maybe instanceof Promise ? maybe.then(apply).catch(console.error) : apply(maybe)
  }, [tool, particleCount, geometry])

  /* ---------- colours ---------- */
  const { isDarkTheme, idleCol, loadCol, speakCol } = useMemo(() => {
    const dark = window.matchMedia('(prefers-color-scheme: dark)').matches
    return {
      isDarkTheme: dark,
      idleCol: dark ? new THREE.Color(0.2, 0.4, 0.6) : new THREE.Color(0, 0, 0),
      /* 🟡 luxury gold for “tool / loading” */
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
        alphaTest: 0.05, // ✨ kill low-alpha halo
        blending: isDarkTheme ? THREE.AdditiveBlending : THREE.NormalBlending
      }),
    [fftTex, idleCol, loadCol, speakCol, isDarkTheme]
  )

  /* ---------- per-frame updates ---------- */
  useFrame(({ clock }, delta) => {
    /* smooth visual state (≈350 ms) */
    stateSmoothRef.current = THREE.MathUtils.damp(stateSmoothRef.current, visualState, 4, delta)
    material.uniforms.uState.value = stateSmoothRef.current

    /* smooth tool morph */
    const toolTarget = tool ? 1 : 0
    toolBlend.current = THREE.MathUtils.damp(toolBlend.current, toolTarget, 3, delta)
    material.uniforms.uToolBlend.value = toolBlend.current

    /* buoyant bob */
    mesh.current.position.y = 0.05 * Math.sin(clock.elapsedTime * 0.8)

    /* FFT → texture (with 25 % lerp) */
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

/* ─────────────────── shaders ─────────────────── */

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
  /* state weights */
  float wToolBase = smoothstep(0.,1.,uState) - smoothstep(1.,2.,uState);
  float wSpeak    = smoothstep(1.,2.,uState);
  float wTool     = max(wToolBase, uToolBlend);

  /* base morph */
  vec3 p = mix(aHome, aTool, wTool);

  /* tiny perpetual wiggle */
  float jitter = 0.02;
  p.xy += vec2(
    sin(uTime*0.8 + aId*12.0)*jitter,
    cos(uTime*0.6 + aId*17.0)*jitter
  );

  /* audio punch on speak */
  float amp = texture2D(uFFT, vec2((aId+0.5)/256., 0.)).r;
  p = mix(p, p + normalize(p)*amp*3., wSpeak);

  /* swirl while morphing */
  float swirlW = wTool * (1. - wSpeak);
  float ang    = atan(p.y, p.x) + uTime*0.25*swirlW;
  float r      = length(p.xy);
  p.xy         = vec2(cos(ang), sin(ang))*r;

  gl_PointSize = max(2., 2. + amp*5.*wSpeak);
  vColor = mix(mix(uIdleCol, uLoadCol, wTool), uSpeakCol, wSpeak);
  gl_Position = projectionMatrix * modelViewMatrix * vec4(p, 1.);
}
`

const fragmentShader = /* glsl */ `
varying vec3 vColor;
void main(){
  float d = length(gl_PointCoord - 0.5);
  float a = smoothstep(0.5, 0., d);
  gl_FragColor = vec4(vColor, a);
}
`
