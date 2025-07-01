import { initializeApp } from 'firebase/app'
import { getAuth } from 'firebase/auth'
import { FirebaseOptions } from 'firebase/app'

export const firebaseConfig: FirebaseOptions = {
  apiKey: process.env.NEXT_PUBLIC_FIREBASE_API_KEY!,
  authDomain: process.env.NEXT_PUBLIC_FIREBASE_AUTH_DOMAIN!,
  projectId: process.env.NEXT_PUBLIC_FIREBASE_PROJECT_ID!
}

const app = initializeApp(firebaseConfig)
export const auth = getAuth(app)
