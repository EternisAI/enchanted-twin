import axios from 'axios'

interface AnonymizationResponse {
  choices: Array<{
    message: {
      content: string
    }
  }>
}

interface AnonymizationMap {
  [key: string]: string
}

const ANONYMIZATION_ENDPOINT = 'http://localhost:8000/v1/chat/completions'
const ANONYMIZATION_MODEL = 'qwen3-0.6b-q4_k_m'

function createAnonymizationPrompt(text: string): string {
  return `
            You are an anonymizer.
            Return ONLY <json>{"orig": "replacement", …}</json>.
            Example
            user: "John Doe is a software engineer at Google"
            assistant: "<json>{"John Doe":"Dave Smith","Google":"TechCorp"}</json>"
            anonymize this:
            ${text}
            /no_think
        `
}

function extractJsonFromResponse(content: string): string | null {
  // First try to extract from <json> tags
  const jsonMatch = content.match(/<json>(.*?)(?:>|<\/json>)/s)
  if (jsonMatch) {
    return jsonMatch[1]
  }

  // If no tags found, try to extract raw JSON
  const rawJsonMatch = content.match(/\{.*\}/s)
  if (rawJsonMatch) {
    return rawJsonMatch[0]
  }

  return null
}

function parseAnonymizationJson(jsonString: string): AnonymizationMap {
  // Replace single quotes with double quotes for valid JSON
  const cleanJsonString = jsonString.replace(/'/g, '"')

  try {
    const parsed = JSON.parse(cleanJsonString)

    // Validate that it's an object with string values
    if (typeof parsed !== 'object' || parsed === null || Array.isArray(parsed)) {
      throw new Error('Anonymization map must be an object')
    }

    // Ensure all values are strings
    for (const [key, value] of Object.entries(parsed)) {
      if (typeof value !== 'string') {
        throw new Error(`Anonymization value for "${key}" must be a string`)
      }
    }

    return parsed
  } catch (error) {
    throw new Error(
      `Failed to parse anonymization JSON: ${error instanceof Error ? error.message : 'Unknown error'}`
    )
  }
}

function applyAnonymization(text: string, anonymizationMap: AnonymizationMap): string {
  let anonymizedText = text

  Object.entries(anonymizationMap).forEach(([original, replacement]) => {
    // Use case-insensitive replacement with word boundaries to avoid partial matches
    const regex = new RegExp(`\\b${original.replace(/[.*+?^${}()|[\]\\]/g, '\\$&')}\\b`, 'gi')
    anonymizedText = anonymizedText.replace(regex, replacement)
  })

  return anonymizedText
}

export async function anonymizeText(text: string): Promise<string> {
  if (!text.trim()) {
    return text
  }

  try {
    const prompt = createAnonymizationPrompt(text)

    const response = await axios.post<AnonymizationResponse>(
      ANONYMIZATION_ENDPOINT,
      {
        messages: [
          {
            role: 'user',
            content: prompt
          }
        ],
        model: ANONYMIZATION_MODEL,
        temperature: 0.5,
        max_tokens: 1000
      },
      {
        headers: {
          'Content-Type': 'application/json'
        },
        timeout: 10000
      }
    )

    const anonymizedContent = response.data.choices[0]?.message?.content

    if (!anonymizedContent) {
      console.warn('No anonymized content received from server')
      return text
    }

    console.log('anonymizedContent', anonymizedContent)

    const jsonString = extractJsonFromResponse(anonymizedContent)

    console.log('jsonString', jsonString)

    if (!jsonString) {
      console.warn('No JSON found in anonymization response')
      return text
    }

    const anonymizationMap = parseAnonymizationJson(jsonString)
    const result = applyAnonymization(text, anonymizationMap)

    console.log('Anonymization applied:', {
      anonymizationMap: anonymizationMap,
      original: text,
      anonymized: result
    })

    return result
  } catch (error) {
    console.error('Anonymization failed:', error)

    // Return original text if anonymization fails
    return text
  }
}
