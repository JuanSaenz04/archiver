import { createRootRoute, Link, Outlet } from '@tanstack/react-router'
import {
  NavigationMenu,
  NavigationMenuItem,
  NavigationMenuLink,
  NavigationMenuList,
} from "@/components/ui/navigation-menu"
import { ThemeProvider } from '@/components/theme-provider'
import { ModeToggle } from '@/components/mode-toggle'

const RootLayout = () => (
  <>
    <ThemeProvider>
      <div className="p-2 flex gap-2 justify-center">
        <NavigationMenu>
          <NavigationMenuList>
            <NavigationMenuItem>
              <NavigationMenuLink>
                <Link to="/" className="[&.active]:font-bold">
                  Home
                </Link>
              </NavigationMenuLink>
            </NavigationMenuItem>
            <NavigationMenuItem>
              <NavigationMenuLink>
                <Link to="/create-archive" className="[&.active]:font-bold">
                  Create archive
                </Link>
              </NavigationMenuLink>
            </NavigationMenuItem>
            <NavigationMenuItem>
              <ModeToggle />
            </NavigationMenuItem>
          </NavigationMenuList>
        </NavigationMenu>
      </div>
      <Outlet />
    </ThemeProvider>
  </>
)

export const Route = createRootRoute({ component: RootLayout })