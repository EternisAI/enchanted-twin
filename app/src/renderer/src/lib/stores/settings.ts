import { create } from 'zustand'

interface SettingsStore {
  activeTab: string
  setActiveTab: (tab: string) => void
}

export const useSettingsStore = create<SettingsStore>((set) => ({
  activeTab: 'appearance',
  setActiveTab: (tab) => set({ activeTab: tab }),
})) 