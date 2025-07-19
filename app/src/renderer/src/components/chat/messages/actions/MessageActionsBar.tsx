export function MessageActionsBar({ children }: { children: React.ReactNode }) {
  return (
    <div className="opacity-25 focus-within:opacity-100 hover:opacity-100 transition-opacity relative -left-2 flex flex-row items-start gap-0 justify-start w-full">
      {children}
    </div>
  )
}
