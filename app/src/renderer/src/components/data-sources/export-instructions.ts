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
    timeEstimate: '24-48 hours',
    steps: [
      'Go to Google Takeout (takeout.google.com)',
      'Sign in with your Google account',
      'Deselect all services',
      'Select only "Mail"',
      'Click "Next" and choose your export options',
      'Click "Create Export"'
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
  }
}
