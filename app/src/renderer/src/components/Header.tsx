import { Link } from '@tanstack/react-router'

export default function Header() {
  return (
    <header className="flex items-center justify-between px-6 py-4 shadow-md bg-white">
      <div className="text-2xl font-bold text-black tracking-wide">Enchanted Twin</div>

      <nav className="flex gap-4 text-gray-700 font-medium">
        <Link to="/" className="hover:text-purple-600 transition-colors">
          Home
        </Link>
        <Link to="/chat" className="hover:text-purple-600 transition-colors">
          Chat
        </Link>
      </nav>
    </header>
  )
}
