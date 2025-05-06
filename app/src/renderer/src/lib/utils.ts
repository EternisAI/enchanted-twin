import { clsx, type ClassValue } from 'clsx'
import { twMerge } from 'tailwind-merge'
import { rrulestr } from 'rrule'

export function cn(...inputs: ClassValue[]) {
  return twMerge(clsx(inputs))
}

export function formatRRuleToText(rruleString: string): string {
  try {
    if (!rruleString) return ''
    const rule = rrulestr(rruleString)
    return rule.toText()
  } catch (err) {
    console.error('Invalid RRULE:', err)
    return 'Invalid schedule'
  }
}
