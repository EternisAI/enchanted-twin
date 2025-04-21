import Spline from '@splinetool/react-spline'
import brainScene from '../../assets/illustrations/brain-2.splinecode?url'

export function Brain() {
  return <Spline scene={brainScene} />
  
  // Fallback to remote URL if needed
  // return <Spline scene="https://prod.spline.design/g9zmLQllMJGky8gO/scene.splinecode" />
}
