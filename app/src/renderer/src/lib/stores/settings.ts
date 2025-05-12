import { create } from 'zustand'

interface SettingsStore {
  isOpen: boolean
  activeTab: string
  open: () => void
  close: () => void
  setActiveTab: (tab: string) => void
}

export const useSettingsStore = create<SettingsStore>((set) => ({
  isOpen: false,
  activeTab: 'appearance',
  open: () => set({ isOpen: true }),
  close: () => set({ isOpen: false }),
  setActiveTab: (tab) => set({ activeTab: tab })
})) 