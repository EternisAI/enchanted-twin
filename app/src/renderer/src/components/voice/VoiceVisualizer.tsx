import { Canvas, useFrame, extend } from '@react-three/fiber'
import { EffectComposer, Bloom } from '@react-three/postprocessing'
import { OrbitControls } from '@react-three/drei'
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
      <color attach="background" args={['#000000']} />
      <Particles
        visualState={visualState}
        particleCount={particleCount}
        getFreqData={getFreqData}
      />
      <EffectComposer disableNormalPass>
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
  const fftTex = useMemo(() => {
    const data = new Uint8Array(256 * 4)
    const tex = new THREE.DataTexture(data, 256, 1, THREE.RGBAFormat)
    tex.minFilter = THREE.NearestFilter
    tex.magFilter = THREE.NearestFilter
    tex.needsUpdate = true
    return tex
  }, [])

  const smoothedFFT = useRef(new Float32Array(256))

  const geometry = useMemo(() => {
    const pos = new Float32Array(particleCount * 3)
    const id = new Float32Array(particleCount)
    for (let i = 0; i < particleCount; i++) {
      const r = Math.cbrt(Math.random()) * 1.3
      const theta = Math.random() * Math.PI * 2
      const phi = Math.acos(2 * Math.random() - 1)
      pos.set(
        [
          r * Math.sin(phi) * Math.cos(theta),
          r * Math.sin(phi) * Math.sin(theta),
          r * Math.cos(phi)
        ],
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
          uState: { value: 0 },
          uTime: { value: 0 }
        },
        vertexShader: vertexShaderSource,
        fragmentShader: fragmentShaderSource,
        transparent: true,
        depthWrite: false,
        blending: THREE.AdditiveBlending
      }),
    [fftTex]
  )

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

    const silent = sm.every((v) => v < 2)
    const target = visualState === 2 && silent ? 0 : visualState
    material.uniforms.uState.value = THREE.MathUtils.lerp(
      material.uniforms.uState.value,
      target,
      delta * 2.0
    )

    material.uniforms.uTime.value = clock.elapsedTime
  })

  return <points ref={mesh} geometry={geometry} material={material} />
}

const vertexShaderSource = /* glsl */ `
uniform sampler2D uFFT;
uniform float uState;
uniform float uTime;
attribute float aId;
varying vec3 vColor;

float bellSmooth(float x, float c, float w) {
  return 1.0 - smoothstep(0.0, w, abs(x - c));
}

void main() {
  vec3 p0 = position;
  vec3 dir = normalize(p0);

  // Swirl idle/loading
  float swirl = mix(0.2, 1.0, uState);
  float angle = uTime * swirl;
  mat2 rot = mat2(cos(angle), -sin(angle), sin(angle), cos(angle));
  vec3 p = p0;
  p.xz = rot * p.xz;

  // Passive breathing in idle
  float wIdle = bellSmooth(uState, 0.0, 1.5);
  p += dir * sin(uTime * 0.8 + aId * 0.1) * 0.05 * wIdle;

  // Speaking expansion based on frequency
  float amp = texture2D(uFFT, vec2(aId / 256.0, 0.0)).r;
  float wSpeak = bellSmooth(uState, 2.0, 1.0);
  float expansion = pow(amp, 1.5) * 3.0;
  p = mix(p, dir * (length(p0) + expansion), wSpeak);

  // Point size modulation
  float size = 2.0 + wIdle * 1.5 + amp * 10.0 * wSpeak;
  gl_PointSize = size;

  // Color mix between idle and speaking
  vec3 colorIdle = vec3(0.2, 0.4, 0.6);
  vec3 colorSpeak = vec3(1.0, 0.5, 0.1);
  vColor = mix(colorIdle, colorSpeak, wSpeak);

  gl_Position = projectionMatrix * modelViewMatrix * vec4(p, 1.0);
}`

const fragmentShaderSource = /* glsl */ `
varying vec3 vColor;
void main() {
  float d = length(gl_PointCoord - 0.5);
  float alpha = smoothstep(0.5, 0.0, d);
  gl_FragColor = vec4(vColor, alpha);
}`
