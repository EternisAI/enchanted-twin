export function TypingIndicator() {
  return (
    <div className="text-sm text-muted-foreground italic px-3 py-1 bg-transparent rounded-md w-fit animate-typingFadeIn">
      <div className="flex items-center justify-center gap-1 h-4 typing-dots">
        {[...Array(3)].map((_, i) => (
          <div key={i} className="h-2 w-2 bg-accent-foreground rounded-full animate-dotPulse" />
        ))}
      </div>
    </div>
  )
}
