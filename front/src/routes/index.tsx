import { AppSidebar } from '@/components/app-sidebar'
import { ArchiveViewer } from '@/components/archive-viewer'
import { SidebarProvider, SidebarTrigger } from '@/components/ui/sidebar'
import { createFileRoute } from '@tanstack/react-router'
import { useState } from 'react'

export const Route = createFileRoute('/')({
  component: Index,
})

function Index() {

  const [ selectedArchive, setSelectedArchive ] = useState("")



  return (

    <SidebarProvider>

      <AppSidebar onArchiveSelected={setSelectedArchive} selectedArchive={selectedArchive} />

      <main className="w-full">

        <SidebarTrigger className='pl-2'/>

        <ArchiveViewer archiveName={selectedArchive} />

      </main>

    </SidebarProvider>

  )

}
