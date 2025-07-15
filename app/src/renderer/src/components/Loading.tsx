import { Loader } from 'lucide-react'

import FreysaLoading from '@renderer/assets/icons/freysaLoading.png'
import { useTheme } from '@renderer/lib/theme'

export default function Loading({ description }: { description?: string }) {
  const { theme } = useTheme()
  return (
    <div className="flex flex-col h-screen w-screen">
      <div className="titlebar text-center fixed top-0 left-0 right-0 text-muted-foreground text-xs h-8 z-20 flex items-center justify-center backdrop-blur-sm" />
      <div
        className="flex-1 flex items-center justify-center"
        style={{
          background:
            theme === 'light'
              ? 'linear-gradient(180deg, #6068E9 0%, #A5AAF9 100%)'
              : 'linear-gradient(180deg, #18181B 0%, #000 100%)'
        }}
      >
        <div className="flex flex-col gap-12 text-primary-foreground p-10 border border-white/50 rounded-lg bg-white/5 min-w-2xl">
          <div className="flex flex-col gap-1 text-center items-center">
            <img src={FreysaLoading} alt="Enchanted" className="w-16 h-16" />
            <h1 className="text-lg font-normal text-white">Starting Enchanted</h1>
            {description && <p className="text-md text-white/80">{description}</p>}
          </div>

          <div className="flex flex-col gap-4">
            <div className="flex flex-col  items-center justify-center gap-3">
              <div className="flex flex-col gap-2 items-center max-w-sm">
                <Loader className="animate-spin w-8 h-8 text-white" />
              </div>
            </div>
          </div>
        </div>
      </div>
    </div>
  )
}
