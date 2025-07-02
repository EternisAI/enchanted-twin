import { ExportInstructions } from './types'

export const EXPORT_INSTRUCTIONS: Record<string, ExportInstructions> = {
  WhatsApp: {
    timeEstimate: '5-10 minutes',
    steps: ['Open Finder', 'Find your ChatStorage.sqlite file']
  },
  Telegram: {
    timeEstimate: '10-30 minutes',
    steps: ['Open Finder', 'Find your Telegram Export file']
  },
  Slack: {
    timeEstimate: '1-2 hours',
    steps: [
      'Open Slack in your browser',
      'Click your workspace name in the top left',
      'Go to Settings & Administration > Workspace Settings',
      'Click "Import/Export Data" in the left sidebar',
      'Click "Export" and follow the prompts'
    ]
  },
  Gmail: {
    timeEstimate: '5 minutes - 48 hours',
    steps: [
      'Option 1: Upload .mbox file directly (if you have one)',
      'Option 2: Export from Google Takeout:',
      '  • Go to Google Takeout (takeout.google.com)',
      '  • Sign in with your Google account',
      '  • Deselect all services, select only "Mail"',
      '  • Click "Next" and choose export options',
      '  • Click "Create Export" and wait for completion'
    ],
    link: 'https://takeout.google.com'
  },
  X: {
    timeEstimate: '24-48 hours',
    steps: [
      'Go to X (Twitter) in your browser',
      'Click your profile picture',
      'Go to Settings and Privacy',
      'Click "Download an archive of your data"',
      'Enter your password and click "Confirm"',
      'Wait for the email with your data archive'
    ],
    link: 'https://x.com/settings/download_your_data'
  },
  ChatGPT: {
    timeEstimate: '5-10 minutes',
    steps: [
      'Option 1: Upload JSON file directly (if you have conversations.json)',
      'Option 2: Export from ChatGPT:',
      '  • Open ChatGPT → Settings → Data Controls',
      '  • Click "Export Data" and follow the prompts',
      '  • Wait 5-10 minutes for the export to complete',
      '  • Check your email for ZIP file or extract conversations.json'
    ],
    link: 'https://chatgpt.com/#settings/DataControls'
  },
  Files: {
    timeEstimate: '1-2 minutes',
    steps: [
      'Upload your files',
      'Select the files you want to import',
      'Click "Import"',
      'Wait for the import to complete'
    ]
  }
}
