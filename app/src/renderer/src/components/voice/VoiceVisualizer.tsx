/**
 * GPU-driven audio visualiser.
 *
 * visualState: 0 = passive, 1 = loading, 2 = speaking
 *
 * Requires:
 *   pnpm add three @react-three/fiber @react-three/drei
 */

import { Canvas, useFrame } from '@react-three/fiber'
import { useMemo, useRef } from 'react'
import * as THREE from 'three'

/* ────────────────────────────────────────────────────────── */
/* Public wrapper – embed this anywhere with visualState prop */

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
    <Canvas className={className ?? ''}>
      <Particles
        visualState={visualState}
        particleCount={particleCount}
        getFreqData={getFreqData}
      />
    </Canvas>
  )
}

/* ────────────────────────────────────────────────────────── */
/* GPU particle implementation */

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

  /* ---------- audio texture (256-bin FFT) ---------- */
  const fftTex = useMemo(() => {
    const tex = new THREE.DataTexture(
      new Float32Array(256 * 4),
      256,
      1,
      THREE.RGBAFormat,
      THREE.FloatType
    )
    tex.needsUpdate = true
    return tex
  }, [])

  /* ---------- geometry (points in a sphere) ---------- */
  const geometry = useMemo(() => {
    const pos = new Float32Array(particleCount * 3)
    const id = new Float32Array(particleCount)

    for (let i = 0; i < particleCount; i++) {
      /* random inside sphere (radius≈1.3) */
      const r = Math.cbrt(Math.random()) * 1.3
      const theta = Math.random() * Math.PI * 2
      const phi = Math.acos(2 * Math.random() - 1)

      pos[i * 3] = r * Math.sin(phi) * Math.cos(theta)
      pos[i * 3 + 1] = r * Math.sin(phi) * Math.sin(theta)
      pos[i * 3 + 2] = r * Math.cos(phi)

      id[i] = i % 256 /* maps each vert to an FFT bin */
    }

    const g = new THREE.BufferGeometry()
    g.setAttribute('position', new THREE.BufferAttribute(pos, 3))
    g.setAttribute('aId', new THREE.BufferAttribute(id, 1))
    return g
  }, [particleCount])

  /* ---------- material with custom shaders ---------- */
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

  /* ---------- per-frame updates ---------- */
  /* ---------- frame loop ---------- */
  useFrame((state, delta) => {
    const { clock } = state
    /* 1 ▸ update FFT texture */
    const fft = getFreqData()
    for (let i = 0; i < 256; i++) {
      const v = (fft[i] ?? 0) / 255
      const idx = i * 4
      fftTex.image.data[idx] = fftTex.image.data[idx + 1] = fftTex.image.data[idx + 2] = v
      fftTex.image.data[idx + 3] = 1
    }
    fftTex.needsUpdate = true

    /* 2 ▸ **smooth spring** toward target visualState  (THREE.MathUtils.damp) */
    const cur = material.uniforms.uState.value as number
    material.uniforms.uState.value = THREE.MathUtils.damp(cur, visualState, 2.5, delta)
    /*  2.5   ⟶ “lambda”   →  smaller = slower ( try 2‒3 for ~½–¾ s transitions ) */

    /* 3 ▸ keep time uniform */
    material.uniforms.uTime.value = clock.elapsedTime
  })

  return <points ref={mesh} geometry={geometry} material={material} />
}

/* ───────────────────────── GLSL ─────────────────────────── */

/* prettier-ignore */
const vertexShader = /* glsl */`
uniform sampler2D uFFT;
uniform float uState;     // 0 → passive, 1 → loading, 2 → speaking (float, tweened)
uniform float uTime;
attribute float aId;

varying vec3 vColor;

/* helper for weights ------------------------------------------------------ */
float bell(float x, float c, float w){
  return clamp(1.0 - abs(x - c) / w, 0.0, 1.0);  // linear bell curve
}

void main() {
  vec3 p   = position;
  vec3 dir = normalize(p);

  /* -------- weights (all in [0,1]) -------------------------------------- */
  float wPassive = bell(uState, 0.0, 0.5);   // centred at 0, width 0.5
  float wLoad    = bell(uState, 1.0, 0.5);   // centred at 1
  float wSpeak   = bell(uState, 2.0, 0.5);   // centred at 2

  /* -------- PASSIVE  | breathing ---------------------------------------- */
  float breathe = sin(uTime * 0.9 + aId * 0.12) * 0.06;
  p += dir * breathe * wPassive;

  /* -------- LOADING  | swirl + gentle contraction ----------------------- */
  if (wLoad > 0.001) {
    float angle = uTime * 2.5 + aId * 0.03;
    mat2  rot   = mat2(cos(angle), -sin(angle), sin(angle), cos(angle));
    float squish = 0.65 + 0.25 * sin(uTime * 1.4);  // 0.4–0.9 radius
    vec3  swirl  = p;
    swirl.xy = rot * swirl.xy * squish;
    p = mix(p, swirl, wLoad);                       // blend by weight
  }

  /* -------- SPEAKING | audio-reactive radial pulse ---------------------- */
  float amp = texture2D(uFFT, vec2(aId / 256.0, 0.0)).r;
  p += dir * amp * amp * 0.7 * wSpeak;

  /* -------- point size -------------------------------------------------- */
  float size = 2.2
             + wLoad  * (2.5 + sin(uTime * 3.0) * 2.0)
             + wSpeak * amp * 16.0;
  gl_PointSize = size;

  /* -------- colour blend ------------------------------------------------ */
  vec3 cPassive = vec3(0.30, 0.42, 0.55);
  vec3 cLoad    = vec3(0.00, 0.82, 1.00);
  vec3 cSpeak   = vec3(1.00, 0.47, 0.10);
  vColor = cPassive * wPassive
         + cLoad    * wLoad
         + mix(cLoad, cSpeak, amp) * wSpeak;

  gl_Position = projectionMatrix * modelViewMatrix * vec4(p, 1.0);
}
`;

/* ─── VoiceVisualizer – UPDATED fragmentShader ─── */
const fragmentShader = /* glsl */ `
varying vec3 vColor;

void main() {
  float d = length(gl_PointCoord - 0.5);
  if (d > 0.5) discard;               // circular sprite

  float alpha = smoothstep(0.5, 0.0, d);   // radial fade
  gl_FragColor = vec4(vColor, alpha);
}
`
