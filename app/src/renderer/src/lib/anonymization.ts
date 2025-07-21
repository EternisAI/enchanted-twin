// Helper functions for anonymization - extracted for testing

// Helper function to sort privacy dictionary keys by length (descending)
export const sortKeysByLengthDesc = (privacyDict: Record<string, string>): string[] => {
  return Object.keys(privacyDict).sort((a, b) => b.length - a.length)
}

// Helper function to check if all letters are uppercase
export const isAllUppercase = (str: string): boolean => {
  return str.split('').every((char) => !char.match(/[a-z]/))
}

// Helper function to check if all letters are lowercase
export const isAllLowercase = (str: string): boolean => {
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
  privacyDict: Record<string, string>
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
      const htmlReplacement = `<span class="bg-muted-foreground px-1.25 py-0.25 rounded text-primary-foreground font-medium">${casePreservedReplacement}</span>`

      // Replace in the result
      result =
        result.substring(0, idx) + htmlReplacement + result.substring(idx + originalLower.length)

      // Update search start position to continue after this replacement
      searchStart = idx + htmlReplacement.length
    }
  })

  return result
}
