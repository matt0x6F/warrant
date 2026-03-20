import { createContext } from 'react'

import { createWarrantClient } from '@/lib/api/client'

export type WarrantClient = ReturnType<typeof createWarrantClient>

export type AuthContextValue = {
  token: string | null
  client: WarrantClient
  signOut: () => void
  refreshTokenFromStorage: () => void
}

export const AuthContext = createContext<AuthContextValue | null>(null)
