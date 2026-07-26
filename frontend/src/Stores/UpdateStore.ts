import { create } from 'zustand'

interface UpdateState {
  updateAvailable: boolean
  setUpdateAvailable: (available: boolean) => void
}

export const useUpdateStore = create<UpdateState>((set) => ({
  updateAvailable: false,
  setUpdateAvailable: (available) => set({ updateAvailable: available }),
}))
