import WhatsAppIcon from '@renderer/assets/icons/whatsapp'
import TelegramIcon from '@renderer/assets/icons/telegram'
import SlackIcon from '@renderer/assets/icons/slack'
import GmailIcon from '@renderer/assets/icons/gmail'
import XformerlyTwitterIcon from '@renderer/assets/icons/x'
import OpenAI from '@renderer/assets/icons/openai'
import WhatsAppSync from './custom-view/WhatAppSync'
import { DataSource } from './types'

export const SUPPORTED_DATA_SOURCES: DataSource[] = [
  {
    name: 'X',
    label: 'Twitter',
    description: 'Import your X tweets and messages',
    selectType: 'files',
    fileRequirement: 'Select X ZIP',
    icon: <XformerlyTwitterIcon className="h-5 w-5" />,
    fileFilters: [{ name: 'X Archive', extensions: ['zip'] }]
  },
  {
    name: 'ChatGPT',
    label: 'ChatGPT',
    description: 'Import your ChatGPT history',
    selectType: 'files',
    fileRequirement: 'Select ChatGPT export file',
    icon: <OpenAI className="h-5 w-5" />,
    fileFilters: [{ name: 'ChatGPT', extensions: ['zip'] }]
  },
  {
    name: 'WhatsApp',
    label: 'WhatsApp',
    description: 'Import your WhatsApp chat history',
    selectType: 'files',
    fileRequirement: 'Select WhatsApp SQLITE file',
    icon: <WhatsAppIcon className="h-5 w-5" />,
    fileFilters: [{ name: 'WhatsApp Database', extensions: ['db', 'sqlite'] }],
    customView: {
      name: 'QR Code',
      component: <WhatsAppSync />
    }
  },
  {
    name: 'Telegram',
    label: 'Telegram',
    description: 'Import your Telegram messages and media',
    selectType: 'files',
    fileRequirement: 'Select Telegram JSON export file',
    icon: <TelegramIcon className="h-5 w-5" />,
    fileFilters: [{ name: 'Telegram Export', extensions: ['json'] }]
  },
  {
    name: 'Slack',
    label: 'Slack',
    description: 'Import your Slack workspace data',
    selectType: 'files',
    fileRequirement: 'Select Slack ZIP file',
    icon: <SlackIcon className="h-6 w-6" />,
    fileFilters: [{ name: 'Slack Export', extensions: ['zip'] }]
  },
  {
    name: 'Gmail',
    label: 'Gmail',
    description: 'Import your Gmail emails and attachments',
    selectType: 'files',
    fileRequirement: 'Select Google Takeout ZIP file',
    icon: <GmailIcon className="h-6 w-6" />,
    fileFilters: [{ name: 'Google Takeout', extensions: ['zip'] }]
  }
]
