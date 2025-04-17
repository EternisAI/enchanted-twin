import { create } from 'zustand'
import { persist } from 'zustand/middleware'

export type DataSourceType = 'WhatsApp' | 'Telegram' | 'Slack' | 'Gmail'

interface DataSource {
  type: DataSourceType
  path?: string
  status: 'pending' | 'processing' | 'completed' | 'error'
  progress?: number
}

interface OnboardingState {
  currentStep: number
  totalSteps: number
  userName: string
  dataSources: DataSource[]
  isCompleted: boolean
  lastCompletedStep: number
  setStep: (step: number) => void
  nextStep: () => void
  previousStep: () => void
  canGoNext: () => boolean
  canGoPrevious: () => boolean
  setUserName: (name: string) => void
  addDataSource: (source: DataSource) => void
  updateDataSource: (type: DataSourceType, updates: Partial<DataSource>) => void
  completeOnboarding: () => void
  resetOnboarding: () => void
}

export const useOnboardingStore = create<OnboardingState>()(
  persist(
    (set, get) => ({
      currentStep: 0,
      totalSteps: 3,
      userName: '',
      dataSources: [],
      isCompleted: false,
      lastCompletedStep: -1,
      setStep: (step) => set({ currentStep: step }),
      nextStep: () => {
        const { currentStep, totalSteps, canGoNext } = get()
        if (canGoNext()) {
          const nextStep = Math.min(currentStep + 1, totalSteps - 1)
          set({ 
            currentStep: nextStep,
            lastCompletedStep: Math.max(get().lastCompletedStep, currentStep)
          })
        }
      },
      previousStep: () => {
        const { currentStep } = get()
        if (currentStep > 0) {
          set({ currentStep: currentStep - 1 })
        }
      },
      canGoNext: () => {
        const { currentStep, userName, dataSources } = get()
        switch (currentStep) {
          case 0:
            return userName.trim().length > 0
          case 1:
            return dataSources.length > 0
          case 2:
            return dataSources.every((source) => source.status === 'completed')
          default:
            return false
        }
      },
      canGoPrevious: () => {
        const { currentStep } = get()
        return currentStep > 0
      },
      setUserName: (name) => set({ userName: name }),
      addDataSource: (source) =>
        set((state) => ({
          dataSources: [...state.dataSources, source]
        })),
      updateDataSource: (type, updates) =>
        set((state) => ({
          dataSources: state.dataSources.map((source) =>
            source.type === type ? { ...source, ...updates } : source
          )
        })),
      completeOnboarding: () => set({ 
        isCompleted: true,
        lastCompletedStep: get().currentStep 
      }),
      resetOnboarding: () => set((state) => ({ 
        isCompleted: false,
        currentStep: state.lastCompletedStep + 1
      }))
    }),
    {
      name: 'onboarding-storage'
    }
  )
) 