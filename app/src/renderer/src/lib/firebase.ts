import { initializeApp } from 'firebase/app'
import { getAuth } from 'firebase/auth'
import { FirebaseOptions } from 'firebase/app'

export const firebaseConfig: FirebaseOptions = {
  apiKey: import.meta.env.VITE_FIREBASE_API_KEY!,
  authDomain: import.meta.env.VITE_FIREBASE_AUTH_DOMAIN!,
  projectId: import.meta.env.VITE_FIREBASE_PROJECT_ID!
}

const app = initializeApp(firebaseConfig)
export const auth = getAuth(app)
