import { createRootRoute, Link, Outlet } from '@tanstack/react-router'
import { ThemeProvider } from '@/components/theme-provider'
import { ModeToggle } from '@/components/mode-toggle'
import { Button } from '@/components/ui/button'
import { Plus } from 'lucide-react'

const RootLayout = () => (
  <ThemeProvider>
    <div className="relative h-screen w-full overflow-hidden bg-background">
      <div className="absolute top-2 right-4 z-50 flex items-center gap-2">
        <ModeToggle />
        <Link to="/create-archive">
          <Button size="icon" variant="outline">
            <Plus />
          </Button>
        </Link>
      </div>
      <Outlet />
    </div>
  </ThemeProvider>
)

export const Route = createRootRoute({ component: RootLayout })