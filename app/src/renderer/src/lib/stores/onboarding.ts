import { create } from 'zustand'
import { persist } from 'zustand/middleware'

export enum IndexingState {
  NotStarted = 'NotStarted',
  ProcessingData = 'ProcessingData',
  IndexingData = 'IndexingData',
  Completed = 'Completed',
  DownloadingModel = 'DownloadingModel',
  CleanUp = 'CleanUp'
}

interface DataSource {
  id: string
  name: string
  path: string
  updatedAt: Date
  isProcessed: boolean
  isIndexed: boolean
}

interface OnboardingState {
  currentStep: number
  totalSteps: number
  userName: string
  dataSources: DataSource[]
  indexingStatus: {
    status: IndexingState
    processingDataProgress: number
    indexingDataProgress: number
  }
  isCompleted: boolean
  lastCompletedStep: number
  setStep: (step: number) => void
  nextStep: () => void
  previousStep: () => void
  canGoNext: () => boolean
  canGoPrevious: () => boolean
  setUserName: (name: string) => void
  addDataSource: (source: DataSource) => void
  updateDataSource: (id: string, updates: Partial<DataSource>) => void
  updateIndexingStatus: (status: Partial<OnboardingState['indexingStatus']>) => void
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
      indexingStatus: {
        status: IndexingState.NotStarted,
        processingDataProgress: 0,
        indexingDataProgress: 0
      },
      isCompleted: false,
      lastCompletedStep: -1,
      setStep: (step) => {
        const { totalSteps } = get()
        // Ensure step is within bounds
        const newStep = Math.max(0, Math.min(step, totalSteps - 1))
        set({ 
          currentStep: newStep,
          lastCompletedStep: Math.max(get().lastCompletedStep, newStep)
        })
      },
      nextStep: () => {
        const { currentStep, totalSteps, canGoNext } = get()
        console.log('Next step:', { currentStep, canGoNext: canGoNext() })
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
        console.log('Previous step:', { currentStep, canGoPrevious: currentStep > 0 })
        if (currentStep > 0) {
          set({ currentStep: currentStep - 1 })
        }
      },
      canGoNext: () => {
        const { currentStep, userName, dataSources, indexingStatus } = get()
        console.log('Checking canGoNext:', { 
          currentStep, 
          userName, 
          dataSourcesLength: dataSources.length,
          indexingStatus: indexingStatus.status 
        })
        switch (currentStep) {
          case 0:
            return userName.trim().length > 0
          case 1:
            return dataSources.length > 0
          case 2:
            return indexingStatus.status === IndexingState.Completed
          default:
            return false
        }
      },
      canGoPrevious: () => {
        const { currentStep } = get()
        console.log('Checking canGoPrevious:', { currentStep })
        return currentStep > 0
      },
      setUserName: (name) => {
        console.log('Setting username:', name)
        set({ userName: name })
      },
      addDataSource: (source) => {
        console.log('Adding data source:', source)
        set((state) => ({
          dataSources: [...state.dataSources, source]
        }))
      },
      updateDataSource: (id, updates) =>
        set((state) => ({
          dataSources: state.dataSources.map((source) =>
            source.id === id ? { ...source, ...updates } : source
          )
        })),
      updateIndexingStatus: (status) => {
        console.log('Updating indexing status:', status)
        set((state) => ({
          indexingStatus: { ...state.indexingStatus, ...status }
        }))
      },
      completeOnboarding: () => {
        const { currentStep } = get()
        console.log('Completing onboarding at step:', currentStep)
        set({ 
          isCompleted: true,
          lastCompletedStep: currentStep
        })
      },
      resetOnboarding: () => {
        console.log('Resetting onboarding')
        set(() => ({ 
          isCompleted: false,
          currentStep: 0,
          lastCompletedStep: -1,
          userName: '',
          dataSources: [],
          indexingStatus: {
            status: IndexingState.NotStarted,
            processingDataProgress: 0,
            indexingDataProgress: 0
          }
        }))
      }
    }),
    {
      name: 'onboarding-storage'
    }
  )
) 