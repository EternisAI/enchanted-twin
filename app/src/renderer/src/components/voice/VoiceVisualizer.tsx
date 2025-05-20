// VoiceVisualizer.tsx
import { Canvas, useFrame } from '@react-three/fiber'
import { useMemo, useRef } from 'react'
import * as THREE from 'three'

export default function VoiceVisualizer({
  visualState,
  getFreqData,
  className,
  particleCount = 12000
}: {
  visualState: 0 | 1 | 2
  getFreqData: () => Uint8Array
  className?: string
  particleCount?: number
}) {
  return (
    <Canvas
      className={className}
      camera={{ position: [0, 0, 5], fov: 60 }}
      gl={{ antialias: true }}
    >
      {/* dark background so additive points show up */}
      <Particles
        visualState={visualState}
        particleCount={particleCount}
        getFreqData={getFreqData}
      />
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

  // 1) use Uint8Array + UnsignedByteType for max compatibility
  const fftTex = useMemo(() => {
    const size = 256 * 4
    const data = new Uint8Array(size) // RGBA8
    const tex = new THREE.DataTexture(data, 256, 1, THREE.RGBAFormat, THREE.UnsignedByteType)
    tex.minFilter = THREE.NearestFilter
    tex.magFilter = THREE.NearestFilter
    tex.needsUpdate = true
    return tex
  }, [])

  const geometry = useMemo(() => {
    const pos = new Float32Array(particleCount * 3)
    const id = new Float32Array(particleCount)
    for (let i = 0; i < particleCount; i++) {
      const r = Math.cbrt(Math.random()) * 1.3
      const θ = Math.random() * Math.PI * 2
      const φ = Math.acos(2 * Math.random() - 1)
      pos.set(
        [r * Math.sin(φ) * Math.cos(θ), r * Math.sin(φ) * Math.sin(θ), r * Math.cos(φ)],
        i * 3
      )
      id[i] = i % 256
    }
    const g = new THREE.BufferGeometry()
    g.setAttribute('position', new THREE.BufferAttribute(pos, 3))
    g.setAttribute('aId', new THREE.BufferAttribute(id, 1))
    return g
  }, [particleCount])

  const material = useMemo(
    () =>
      new THREE.ShaderMaterial({
        uniforms: {
          uFFT: { value: fftTex },
          uState: { value: visualState },
          uTime: { value: 0 }
        },
        vertexShader,
        fragmentShader,
        transparent: true,
        depthWrite: false,
        blending: THREE.AdditiveBlending
      }),
    [fftTex, visualState]
  )

  useFrame(({ clock }, delta) => {
    // update FFT texture bytes
    const fft = getFreqData()
    const data = fftTex.image.data as Uint8Array
    for (let i = 0; i < 256; i++) {
      const b = fft[i] // 0–255
      const off = i * 4
      data[off] = b
      data[off + 1] = b
      data[off + 2] = b
      data[off + 3] = 255
    }
    fftTex.needsUpdate = true

    // smooth state tween (≈0.6 s)
    const cur = material.uniforms.uState.value as number
    material.uniforms.uState.value = THREE.MathUtils.damp(cur, visualState, 4.0, delta)

    material.uniforms.uTime.value = clock.elapsedTime
  })

  return <points ref={mesh} geometry={geometry} material={material} />
}

/* ─── GLSL ───────────────────────────────────────────────── */

/* smooth bell with overlap */
const vertexShader = /* glsl */ `
uniform sampler2D uFFT;
uniform float      uState;
uniform float      uTime;
attribute float    aId;
varying   vec3     vColor;

float bellSmooth(float x, float c, float w){
  float d = abs(x - c);
  return 1.0 - smoothstep(0.0, w, d);
}

const float WIDTH = 1.0;

void main() {
  vec3 p   = position;
  vec3 dir = normalize(p);

  float wP = bellSmooth(uState, 0.0, WIDTH);
  float wL = bellSmooth(uState, 1.0, WIDTH);
  float wS = bellSmooth(uState, 2.0, WIDTH);

  float sumW = wP + wL + wS + 1e-4;
  wP /= sumW; wL /= sumW; wS /= sumW;

  // passive breathing
  float breathe = sin(uTime * 0.9 + aId * 0.12) * 0.06;
  p += dir * breathe * wP;

  // loading swirl
  if (wL > 0.001) {
    float a = uTime * 2.5 + aId * 0.03;
    mat2 R = mat2(cos(a), -sin(a), sin(a), cos(a));
    vec3 q = p;
    q.xy = R * q.xy * (0.60 + 0.25 * sin(uTime * 1.4));
    p = mix(p, q, wL);
  }

  // speaking pulse
  float amp = texture2D(uFFT, vec2(aId/256.0, 0.)).r;
  p += dir * amp * amp * 0.7 * wS;

  // size & color
  float size = 2.2
             + wL * (2.5 + sin(uTime * 3.0) * 2.0)
             + wS * amp * 16.0;
  gl_PointSize = size;

  vec3 cP = vec3(0.30,0.42,0.55);
  vec3 cL = vec3(0.00,0.82,1.00);
  vec3 cS = vec3(1.00,0.47,0.10);
  vColor = cP*wP + cL*wL + mix(cL,cS,amp)*wS;

  gl_Position = projectionMatrix * modelViewMatrix * vec4(p,1.0);
}`

const fragmentShader = /* glsl */ `
varying vec3 vColor;
void main() {
  float d = length(gl_PointCoord - 0.5);
  if (d > 0.5) discard;
  float a = smoothstep(0.5, 0.0, d);
  gl_FragColor = vec4(vColor, a);
}`
