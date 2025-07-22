import WhatsAppIcon from '@renderer/assets/icons/whatsapp'
import TelegramIcon from '@renderer/assets/icons/telegram'
import SlackIcon from '@renderer/assets/icons/slack'
import GmailIcon from '@renderer/assets/icons/gmail'
import XformerlyTwitterIcon from '@renderer/assets/icons/x'
import OpenAI from '@renderer/assets/icons/openai'
import WhatsAppSync from './custom-view/WhatAppSync'
import { DataSource } from './types'
import { FileIcon } from 'lucide-react'
import IconContainer from '@renderer/assets/icons/IconContainer'

export const SUPPORTED_DATA_SOURCES: DataSource[] = [
  {
    name: 'X',
    label: 'Twitter',
    description: 'Import your X tweets and messages',
    selectType: 'files',
    fileRequirement: 'Select X ZIP',
    icon: (
      <IconContainer className="bg-foreground size-8">
        <XformerlyTwitterIcon className="size-5 text-primary-foreground" />
      </IconContainer>
    ),
    fileFilters: [{ name: 'X Archive', extensions: ['zip'] }]
  },
  {
    name: 'ChatGPT',
    label: 'ChatGPT',
    description: 'Import your ChatGPT history',
    selectType: 'files',
    fileRequirement: 'Select ChatGPT export file',
    icon: <OpenAI className="size-8" />,
    fileFilters: [{ name: 'ChatGPT', extensions: ['zip'] }]
  },
  {
    name: 'WhatsApp',
    label: 'WhatsApp',
    description: 'Import your WhatsApp chat history',
    selectType: 'files',
    fileRequirement: 'Select WhatsApp SQLITE file',
    icon: (
      <IconContainer className="bg-[#01E676] size-8">
        <WhatsAppIcon className="size-5 text-[#01E676]" />
      </IconContainer>
    ),
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
    icon: <TelegramIcon className="size-8" />,
    fileFilters: [{ name: 'Telegram Export', extensions: ['json'] }]
  },
  {
    name: 'Slack',
    label: 'Slack',
    description: 'Import your Slack workspace data',
    selectType: 'files',
    fileRequirement: 'Select Slack ZIP file',
    icon: <SlackIcon className="size-8" />,
    fileFilters: [{ name: 'Slack Export', extensions: ['zip'] }]
  },
  {
    name: 'Gmail',
    label: 'Gmail',
    description: 'Import your Gmail emails and attachments',
    selectType: 'files',
    fileRequirement: 'Select Google Takeout ZIP file',
    icon: <GmailIcon className="size-8" />,
    fileFilters: [{ name: 'Google Takeout', extensions: ['zip'] }]
  },
  {
    name: 'synced-document',
    label: 'Files',
    description: 'Import your files',
    selectType: 'files',
    fileRequirement: 'Select files in .txt, .pdf, .doc, .docx, .xls, .xlsx, .csv format',
    icon: (
      <IconContainer className="size-8">
        <FileIcon strokeWidth={1.5} className="size-5" />
      </IconContainer>
    ),
    fileFilters: [
      { name: 'Files', extensions: ['txt', 'pdf', 'doc', 'docx', 'xls', 'xlsx', 'csv'] }
    ]
  }
]
