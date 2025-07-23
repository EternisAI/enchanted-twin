// Helper functions for anonymization - extracted for testing
import React from 'react'

// We need to import the Markdown component for the AnonymizedContent component
// This will be passed as a prop to avoid circular dependencies
export interface MarkdownComponent {
  (props: { children: string }): React.ReactElement
}

// Helper function to sort privacy dictionary keys by length (descending)
export const sortKeysByLengthDesc = (privacyDict: Record<string, string>): string[] => {
  return Object.keys(privacyDict).sort((a, b) => b.length - a.length)
}

// Helper function to check if all letters are uppercase
export const isAllUppercase = (str: string): boolean => {
  // First check if string contains at least one letter
  if (!/[a-zA-Z]/.test(str)) {
    return false
  }
  return str.split('').every((char) => !char.match(/[a-z]/))
}

// Helper function to check if all letters are lowercase
export const isAllLowercase = (str: string): boolean => {
  // First check if string contains at least one letter
  if (!/[a-zA-Z]/.test(str)) {
    return false
  }
  return str.split('').every((char) => !char.match(/[A-Z]/))
}

// Helper function to apply case pattern to replacement
export const applyCasePattern = (source: string, target: string): string => {
  if (!source || !target) return target

  // Handle compound words (contain spaces)
  if (source.includes(' ') && target.includes(' ')) {
    const sourceWords = source.split(' ')
    const targetWords = target.split(' ')

    // If word counts don't match, fall back to simple rules
    if (sourceWords.length !== targetWords.length) {
      return applyCasePatternToSingleWord(source, target)
    }

    const result = sourceWords.map((sourceWord, i) => {
      const targetWord = targetWords[i]
      return targetWord ? applyCasePatternToSingleWord(sourceWord, targetWord) : targetWord
    })

    return result.join(' ')
  }

  // Handle single words
  return applyCasePatternToSingleWord(source, target)
}

// Helper function to apply case pattern to a single word
export const applyCasePatternToSingleWord = (source: string, target: string): string => {
  if (!source || !target) return target

  // Check if source is all uppercase
  if (isAllUppercase(source)) {
    return target.toUpperCase()
  }

  // Check if source is all lowercase
  if (isAllLowercase(source)) {
    return target.toLowerCase()
  }

  // In other cases, capitalize only first letter
  return target.charAt(0).toUpperCase() + target.slice(1).toLowerCase()
}

// Helper function to check if position is at word boundary
export const isWordBoundaryAt = (text: string, idx: number, length: number): boolean => {
  // Check character before - should not be a letter or digit
  if (idx > 0) {
    const prevChar = text[idx - 1]
    if (prevChar.match(/[a-zA-Z0-9]/)) {
      return false
    }
  }

  // Check character after - should not be a letter or digit
  if (idx + length < text.length) {
    const nextChar = text[idx + length]
    if (nextChar.match(/[a-zA-Z0-9]/)) {
      return false
    }
  }

  return true
}

// Helper function to perform case-preserving replacement with word boundaries
export const replaceWithCasePreservation = (
  text: string,
  original: string,
  replacement: string
): string => {
  // Safety check for string inputs
  if (typeof text !== 'string' || typeof original !== 'string' || typeof replacement !== 'string') {
    return text
  }

  let result = text
  const originalLower = original.toLowerCase()

  let searchStart = 0

  while (true) {
    // Find next occurrence (case-insensitive)
    const lowerText = result.toLowerCase()
    const idx = lowerText.indexOf(originalLower, searchStart)

    if (idx === -1) break

    // Check if this is a word boundary
    if (!isWordBoundaryAt(result, idx, originalLower.length)) {
      // Not a word boundary, skip this occurrence and continue searching
      searchStart = idx + 1
      continue
    }

    // Extract the actual case pattern from the text
    const foundText = result.substring(idx, idx + originalLower.length)

    // Apply the case pattern to the replacement
    const casePreservedReplacement = applyCasePattern(foundText, replacement)

    // Replace in the result
    result =
      result.substring(0, idx) +
      casePreservedReplacement +
      result.substring(idx + originalLower.length)

    // Update search start position to continue after this replacement
    searchStart = idx + casePreservedReplacement.length
  }

  return result
}

// Main anonymization function for strings
export const anonymizeTextString = (text: string, privacyDict: Record<string, string>): string => {
  if (!privacyDict || Object.keys(privacyDict).length === 0) return text

  let result = text

  // Sort rules by length (longest first) to avoid partial matches
  const sortedOriginals = sortKeysByLengthDesc(privacyDict)

  sortedOriginals.forEach((original) => {
    const replacement = privacyDict[original]

    // Skip if replacement is not a string
    if (typeof replacement !== 'string') {
      return
    }

    result = replaceWithCasePreservation(result, original, replacement)
  })

  return result
}

// Anonymization function for HTML/Markdown strings with HTML tag avoidance
export const anonymizeTextForMarkdownString = (
  text: string,
  privacyDict: Record<string, string>,
  styleConfig: AnonymizationStyleConfig = DEFAULT_ANONYMIZATION_STYLE
): string => {
  if (!privacyDict || Object.keys(privacyDict).length === 0) return text

  let result = text

  // Sort rules by length (longest first) to avoid partial matches
  const sortedOriginals = sortKeysByLengthDesc(privacyDict)

  sortedOriginals.forEach((original) => {
    const replacement = privacyDict[original]

    // Skip if replacement is not a string
    if (typeof replacement !== 'string') {
      return
    }

    const originalLower = original.toLowerCase()

    let searchStart = 0

    while (true) {
      // Find next occurrence (case-insensitive)
      const lowerText = result.toLowerCase()
      const idx = lowerText.indexOf(originalLower, searchStart)

      if (idx === -1) break

      // Check if we're inside an HTML tag or already processed span
      const beforeIdx = result.substring(0, idx)

      // Check if we're inside an HTML tag
      const lastOpenTag = beforeIdx.lastIndexOf('<')
      const lastCloseTag = beforeIdx.lastIndexOf('>')
      const isInsideTag = lastOpenTag !== -1 && (lastCloseTag === -1 || lastOpenTag > lastCloseTag)

      // Check if we're inside an already processed span by counting open/close spans
      const spanOpenCount = (beforeIdx.match(/<span class="bg-muted-foreground/g) || []).length
      const spanCloseCount = (beforeIdx.match(/<\/span>/g) || []).length
      const isInsideSpan = spanOpenCount > spanCloseCount

      if (isInsideTag || isInsideSpan) {
        // Skip this occurrence and continue searching
        searchStart = idx + 1
        continue
      }

      // Check if this is a word boundary
      if (!isWordBoundaryAt(result, idx, originalLower.length)) {
        // Not a word boundary, skip this occurrence and continue searching
        searchStart = idx + 1
        continue
      }

      // Extract the actual case pattern from the text
      const foundText = result.substring(idx, idx + originalLower.length)

      // Apply the case pattern to the replacement
      const casePreservedReplacement = applyCasePattern(foundText, replacement)

      // Replace with HTML span
      const htmlReplacement = `<span class="${styleConfig.className}">${casePreservedReplacement}</span>`

      // Replace in the result
      result =
        result.substring(0, idx) + htmlReplacement + result.substring(idx + originalLower.length)

      // Update search start position to continue after this replacement
      searchStart = idx + htmlReplacement.length
    }
  })

  return result
}

// Utility function to parse privacy dictionary JSON
export const parsePrivacyDict = (privacyDictJson: string | null): Record<string, string> | null => {
  if (!privacyDictJson) return null

  try {
    return JSON.parse(privacyDictJson) as Record<string, string>
  } catch {
    // If JSON is malformed, return null
    return null
  }
}

// Configuration for anonymized text styling
export interface AnonymizationStyleConfig {
  className: string
}

export const DEFAULT_ANONYMIZATION_STYLE: AnonymizationStyleConfig = {
  className: 'bg-muted-foreground px-1.25 py-0.25 rounded text-primary-foreground font-medium'
}

// React-specific anonymization function that returns JSX elements
export const anonymizeTextForReact = (
  text: string,
  privacyDict: Record<string, string>,
  styleConfig: AnonymizationStyleConfig = DEFAULT_ANONYMIZATION_STYLE
): React.ReactElement => {
  if (!privacyDict || Object.keys(privacyDict).length === 0) {
    return React.createElement('span', {}, text)
  }

  let parts: (string | React.ReactElement)[] = [text]

  // Sort rules by length (longest first) to avoid partial matches
  const sortedOriginals = sortKeysByLengthDesc(privacyDict)

  sortedOriginals.forEach((original) => {
    const replacement = privacyDict[original]

    // Skip if replacement is not a string
    if (typeof replacement !== 'string') {
      return
    }

    parts = parts.flatMap((part) => {
      if (typeof part === 'string') {
        // Use the case-preserving replacement logic
        const processedText = replaceWithCasePreservation(part, original, replacement)

        // If no replacement occurred, return the original part
        if (processedText === part) {
          return [part]
        }

        // Now split by the replacement to create React elements
        const segments: (string | React.ReactElement)[] = []
        let searchStart = 0

        while (true) {
          const lowerText = processedText.toLowerCase()
          const idx = lowerText.indexOf(replacement.toLowerCase(), searchStart)

          if (idx === -1) {
            // No more replacements, add the rest of the text
            if (searchStart < processedText.length) {
              segments.push(processedText.substring(searchStart))
            }
            break
          }

          // Add text before the replacement
          if (idx > searchStart) {
            segments.push(processedText.substring(searchStart, idx))
          }

          // Add the replacement as a React element
          segments.push(
            React.createElement(
              'span',
              {
                key: `${original}-${idx}`,
                className: styleConfig.className
              },
              processedText.substring(idx, idx + replacement.length)
            )
          )

          searchStart = idx + replacement.length
        }

        return segments.filter((segment) => segment !== '')
      }
      return part
    })
  })

  return React.createElement('span', {}, ...parts)
}

// Convenience function that combines JSON parsing with React anonymization
export const anonymizeTextWithJson = (
  text: string,
  privacyDictJson: string | null,
  isAnonymized: boolean,
  styleConfig?: AnonymizationStyleConfig
): React.ReactElement | string => {
  if (!privacyDictJson || !isAnonymized) return text

  const privacyDict = parsePrivacyDict(privacyDictJson)
  if (!privacyDict) return text

  return anonymizeTextForReact(text, privacyDict, styleConfig)
}

// Convenience function for markdown anonymization with JSON parsing
export const anonymizeTextForMarkdownWithJson = (
  text: string,
  privacyDictJson: string | null,
  isAnonymized: boolean,
  styleConfig?: AnonymizationStyleConfig
): string => {
  if (!privacyDictJson || !isAnonymized) return text

  const privacyDict = parsePrivacyDict(privacyDictJson)
  if (!privacyDict) return text

  return anonymizeTextForMarkdownString(text, privacyDict, styleConfig)
}

// Generic AnonymizedContent component that can work with or without markdown
export interface AnonymizedContentProps {
  text: string
  chatPrivacyDict: string | null
  isAnonymized: boolean
  asMarkdown?: boolean
  styleConfig?: AnonymizationStyleConfig
  MarkdownComponent?: MarkdownComponent
}

export const AnonymizedContent: React.FC<AnonymizedContentProps> = ({
  text,
  chatPrivacyDict,
  isAnonymized,
  asMarkdown = false,
  styleConfig,
  MarkdownComponent
}) => {
  if (asMarkdown) {
    if (!MarkdownComponent) {
      throw new Error('MarkdownComponent must be provided when asMarkdown is true')
    }
    const mdText = anonymizeTextForMarkdownWithJson(
      text,
      chatPrivacyDict,
      isAnonymized,
      styleConfig
    )
    return React.createElement(MarkdownComponent, {}, mdText)
  } else {
    const result = anonymizeTextWithJson(text, chatPrivacyDict, isAnonymized, styleConfig)
    return typeof result === 'string' ? React.createElement('span', {}, result) : result
  }
}
