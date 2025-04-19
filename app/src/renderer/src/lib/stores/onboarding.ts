import { create } from 'zustand'
import { createJSONStorage, persist } from 'zustand/middleware'

export enum IndexingState {
  NotStarted = 'NOT_STARTED',
  DownloadingModel = 'DOWNLOADING_MODEL',
  ProcessingData = 'PROCESSING_DATA',
  IndexingData = 'INDEXING_DATA',
  CleanUp = 'CLEAN_UP',
  Completed = 'COMPLETED',
  Failed = 'FAILED'
}

export enum OnboardingStep {
  Welcome = 0,
  DataSources = 1,
  Indexing = 2
}

interface DataSource {
  id: string
  name: string
  path: string
  updatedAt: Date
  isProcessed: boolean
  isIndexed: boolean
  hasError: boolean
}

interface StepValidation {
  canProceed: () => boolean
  canGoBack: () => boolean
}

interface OnboardingState {
  currentStep: OnboardingStep
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
  stepValidation: StepValidation
  setStep: (step: OnboardingStep) => void
  nextStep: () => void
  previousStep: () => void
  setUserName: (name: string) => void
  addDataSource: (source: DataSource) => void
  removeDataSource: (id: string) => void
  updateDataSource: (id: string, updates: Partial<DataSource>) => void
  updateIndexingStatus: (status: Partial<OnboardingState['indexingStatus']>) => void
  completeOnboarding: () => void
  resetOnboarding: () => void
}

const validateStep = (state: OnboardingState): StepValidation => ({
  canProceed: () => {
    switch (state.currentStep) {
      case OnboardingStep.Welcome:
        return state.userName.trim().length > 0
      case OnboardingStep.DataSources:
        return state.dataSources.length > 0
      case OnboardingStep.Indexing:
        return state.indexingStatus.status === IndexingState.Completed
      default:
        return false
    }
  },
  canGoBack: () => state.currentStep > OnboardingStep.Welcome
})

export const useOnboardingStore = create<OnboardingState>()(
  persist(
    (set, get) => {
      const createState = (state: Partial<OnboardingState> = {}) => ({
        currentStep: state.currentStep ?? OnboardingStep.Welcome,
        totalSteps: state.totalSteps ?? Object.keys(OnboardingStep).length / 2,
        userName: state.userName ?? '',
        dataSources: state.dataSources ?? [],
        indexingStatus: state.indexingStatus ?? {
          status: IndexingState.NotStarted,
          processingDataProgress: 0,
          indexingDataProgress: 0
        },
        isCompleted: state.isCompleted ?? false,
        lastCompletedStep: state.lastCompletedStep ?? -1,
        stepValidation: validateStep({
          currentStep: state.currentStep ?? OnboardingStep.Welcome,
          totalSteps: state.totalSteps ?? Object.keys(OnboardingStep).length / 2,
          userName: state.userName ?? '',
          dataSources: state.dataSources ?? [],
          indexingStatus: state.indexingStatus ?? {
            status: IndexingState.NotStarted,
            processingDataProgress: 0,
            indexingDataProgress: 0
          },
          isCompleted: state.isCompleted ?? false,
          lastCompletedStep: state.lastCompletedStep ?? -1,
          stepValidation: {} as StepValidation,
          setStep: () => {},
          nextStep: () => {},
          previousStep: () => {},
          setUserName: () => {},
          addDataSource: () => {},
          removeDataSource: () => {},
          updateDataSource: () => {},
          updateIndexingStatus: () => {},
          completeOnboarding: () => {},
          resetOnboarding: () => {}
        }),
        setStep: (step: OnboardingStep) => {
          const { totalSteps } = get()
          const newStep = Math.max(0, Math.min(step, totalSteps - 1)) as OnboardingStep
          set((state) => ({
            currentStep: newStep,
            lastCompletedStep: Math.max(state.lastCompletedStep, newStep),
            stepValidation: validateStep({ ...state, currentStep: newStep })
          }))
        },
        nextStep: () => {
          const { currentStep, stepValidation } = get()
          if (stepValidation.canProceed()) {
            const nextStep = (currentStep + 1) as OnboardingStep
            set((state) => ({
              currentStep: nextStep,
              lastCompletedStep: Math.max(state.lastCompletedStep, currentStep),
              stepValidation: validateStep({ ...state, currentStep: nextStep })
            }))
          }
        },
        previousStep: () => {
          const { currentStep, stepValidation } = get()
          if (stepValidation.canGoBack()) {
            const prevStep = (currentStep - 1) as OnboardingStep
            set((state) => ({
              currentStep: prevStep,
              stepValidation: validateStep({ ...state, currentStep: prevStep })
            }))
          }
        },
        setUserName: (name: string) => {
          set((state) => ({
            userName: name,
            stepValidation: validateStep({ ...state, userName: name })
          }))
        },
        addDataSource: (source: DataSource) => {
          set((state) => ({
            dataSources: [...state.dataSources, source],
            stepValidation: validateStep({ ...state, dataSources: [...state.dataSources, source] })
          }))
        },
        removeDataSource: (id: string) => {
          set((state) => ({
            dataSources: state.dataSources.filter((source) => source.id !== id),
            stepValidation: validateStep({
              ...state,
              dataSources: state.dataSources.filter((source) => source.id !== id)
            })
          }))
        },
        updateDataSource: (id: string, updates: Partial<DataSource>) =>
          set((state) => ({
            dataSources: state.dataSources.map((source) =>
              source.id === id ? { ...source, ...updates } : source
            )
          })),
        updateIndexingStatus: (status: Partial<OnboardingState['indexingStatus']>) => {
          set((state) => ({
            indexingStatus: { ...state.indexingStatus, ...status },
            stepValidation: validateStep({
              ...state,
              indexingStatus: { ...state.indexingStatus, ...status }
            })
          }))
        },
        completeOnboarding: () => {
          const { currentStep } = get()
          set({
            isCompleted: true,
            lastCompletedStep: currentStep
          })
        },
        resetOnboarding: () => {
          set(createState())
        }
      })

      return createState()
    },
    {
      name: 'onboarding-storage',
      version: 1,
      storage: createJSONStorage(() => localStorage),
      onRehydrateStorage: () => (state) => {
        if (state) {
          state.stepValidation = validateStep(state)
        }
      }
    }
  )
)
