export const truncatePath = (path: string): string => {
  return path.length > 40 ? '...' + path.slice(-37) : path
}
