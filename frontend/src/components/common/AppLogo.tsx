interface AppLogoProps {
  className?: string
  size?: number
}

export function AppLogo({ className, size = 24 }: AppLogoProps) {
  return (
    <div
      className={`overflow-hidden rounded-full bg-primary/10 ${className || ''}`}
      style={{ width: size, height: size }}
    >
      <img
        src="/icon.png"
        alt="Marcus"
        className="h-full w-full object-cover"
        style={{ width: size, height: size }}
      />
    </div>
  )
}
