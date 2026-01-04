import { AppSidebar } from '@/components/app-sidebar'
import { ArchiveViewer } from '@/components/archive-viewer'
import { SidebarInset, SidebarProvider, SidebarTrigger } from '@/components/ui/sidebar'
import { createFileRoute } from '@tanstack/react-router'
import { useState } from 'react'

export const Route = createFileRoute('/')({
  component: Index,
})

function Index() {

  const [ selectedArchive, setSelectedArchive ] = useState("")



  return (

    <SidebarProvider className="h-full min-h-0 relative">

      <AppSidebar onArchiveSelected={setSelectedArchive} selectedArchive={selectedArchive} />

      <SidebarInset className="flex flex-col flex-1 min-h-0 overflow-hidden">

        <header className="flex h-12 shrink-0 items-center gap-2 px-4">
          <SidebarTrigger />
        </header>

        <div className="flex-1 min-h-0 p-4 pt-0">
          <ArchiveViewer archiveName={selectedArchive} />
        </div>

      </SidebarInset>

    </SidebarProvider>

  )

}
