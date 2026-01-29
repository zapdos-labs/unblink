import { JSX } from 'solid-js'

interface ButtonProps {
  onClick?: () => void
  disabled?: boolean
  loading?: boolean
  children: JSX.Element
}

export function Button(props: ButtonProps) {
  return (
    <button
      onClick={props.onClick}
      disabled={props.disabled || props.loading}
      class="drop-shadow-xl px-3 py-1.5 rounded-xl border border-neu-750 bg-neu-800 hover:bg-neu-850 flex items-center space-x-2 text-sm text-white disabled:opacity-50 disabled:cursor-not-allowed transition-colors"
    >
      {props.children}
    </button>
  )
}
