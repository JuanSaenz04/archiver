import { createRootRoute, Link, Outlet } from '@tanstack/react-router'
import { ThemeProvider } from '@/components/theme-provider'
import { ModeToggle } from '@/components/mode-toggle'
import { Button } from '@/components/ui/button'
import { Plus, CalendarRange } from 'lucide-react'
import { JobsSheet } from '@/components/jobs-sheet'
import { Toaster } from '@/components/ui/sonner'
import { TooltipProvider } from '@/components/ui/tooltip'

const RootLayout = () => (
  <ThemeProvider>
    <TooltipProvider>
      <div className="relative h-screen w-full overflow-hidden bg-background">
        <div className="absolute top-2 right-4 z-50 flex items-center gap-2">
          <JobsSheet />
          <Link to="/timeline">
            <Button size="icon" variant="outline">
              <CalendarRange />
            </Button>
          </Link>
          <ModeToggle />
          <Link to="/create-archive">
            <Button size="icon" variant="outline">
              <Plus />
            </Button>
          </Link>
        </div>
        <Outlet />
        <Toaster position="bottom-right" richColors />
      </div>
    </TooltipProvider>
  </ThemeProvider>
)

export const Route = createRootRoute({ component: RootLayout })