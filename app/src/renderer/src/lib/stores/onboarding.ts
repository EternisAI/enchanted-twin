import { create } from 'zustand'
import { createJSONStorage, persist } from 'zustand/middleware'

export enum OnboardingStep {
  Welcome = 0,
  MCPServers = 1,
  DataSources = 2,
  Indexing = 3,
  Finished = 4
}

interface OnboardingState {
  currentStep: OnboardingStep
  totalSteps: number
  isCompleted: boolean
  lastCompletedStep: number
  setStep: (step: OnboardingStep) => void
  nextStep: () => void
  previousStep: () => void
  completeOnboarding: () => void
  resetOnboarding: () => void
}

export const useOnboardingStore = create<OnboardingState>()(
  persist(
    (set, get) => ({
      currentStep: OnboardingStep.Welcome,
      totalSteps: Object.keys(OnboardingStep).length / 2,
      isCompleted: false,
      lastCompletedStep: -1,
      setStep: (step: OnboardingStep) => {
        const { totalSteps } = get()
        const newStep = Math.max(0, Math.min(step, totalSteps - 1)) as OnboardingStep
        set((state) => ({
          currentStep: newStep,
          lastCompletedStep: Math.max(state.lastCompletedStep, newStep)
        }))
      },
      nextStep: () => {
        const { currentStep, totalSteps } = get()
        if (currentStep < totalSteps - 1) {
          const nextStep = (currentStep + 1) as OnboardingStep
          set((state) => ({
            currentStep: nextStep,
            lastCompletedStep: Math.max(state.lastCompletedStep, currentStep)
          }))
        }
      },
      previousStep: () => {
        const { currentStep } = get()
        if (currentStep > 0) {
          const prevStep = (currentStep - 1) as OnboardingStep
          set(() => ({
            currentStep: prevStep
          }))
        }
      },
      completeOnboarding: () => {
        const { currentStep } = get()
        set({
          isCompleted: true,
          lastCompletedStep: currentStep
        })
      },
      resetOnboarding: () => {
        set({
          currentStep: OnboardingStep.Welcome,
          isCompleted: false,
          lastCompletedStep: -1
        })
      }
    }),
    {
      name: 'onboarding-storage',
      version: 2,
      storage: createJSONStorage(() => localStorage)
    }
  )
)
