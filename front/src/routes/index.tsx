import { AppSidebar } from '@/components/app-sidebar'
import { ArchiveViewer } from '@/components/archive-viewer'
import { SidebarProvider, SidebarTrigger } from '@/components/ui/sidebar'
import { createFileRoute } from '@tanstack/react-router'

export const Route = createFileRoute('/')({
  component: Index,
})

function Index() {
  return (
    <SidebarProvider>
      <AppSidebar />
      <main>
        <SidebarTrigger className='pl-2'/>
        <ArchiveViewer />
      </main>
    </SidebarProvider>
  )
}