type MediaStatusType =
  | 'granted'
  | 'not-determined'
  | 'denied'
  | 'restricted'
  | 'unavailable'
  | 'loading'

type ScreenRecordingPermission = 'granted' | 'denied' | 'not-determined' | 'restricted' | 'unavailable'

/**
 * Type guard to safely convert MediaStatusType to ScreenRecordingPermission
 * by handling the 'loading' state that isn't supported by the modal component
 */
export function getSafeScreenRecordingPermission(permission: MediaStatusType): ScreenRecordingPermission {
  if (permission === 'loading') {
    return 'not-determined' // Default to not-determined when loading
  }
  // Type assertion is now safe because we've handled the loading case
  return permission as ScreenRecordingPermission
}