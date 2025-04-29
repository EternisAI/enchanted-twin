import { useNavigate } from '@tanstack/react-router'
import { useEffect, useRef } from 'react'

export default function KeyboardShortcuts() {
  const navigate = useNavigate()
  const enterPressed = useRef(false)
  const timer = useRef<NodeJS.Timeout | null>(null)

  useEffect(() => {
    const handleKeyDown = (e: KeyboardEvent) => {
      if (e.key === 'Enter') {
        enterPressed.current = true

        if (timer.current) {
          clearTimeout(timer.current)
        }

        timer.current = setTimeout(() => {
          enterPressed.current = false
        }, 500)
      } else if (enterPressed.current && e.key === 'd') {
        navigate({ to: '/admin' })
        enterPressed.current = false
        if (timer.current) {
          clearTimeout(timer.current)
          timer.current = null
        }
      }
    }

    window.addEventListener('keydown', handleKeyDown)

    return () => {
      window.removeEventListener('keydown', handleKeyDown)
      if (timer.current) {
        clearTimeout(timer.current)
      }
    }
  }, [navigate])

  return null
}
