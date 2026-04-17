import { AppSidebar } from '@/components/app-sidebar'
import { ArchiveViewer } from '@/components/archive-viewer'
import { SidebarInset, SidebarProvider, SidebarTrigger } from '@/components/ui/sidebar'
import { apiClient } from '@/lib/api'
import type { Archive, GetArchivesResponse } from '@/models/archive'
import { createFileRoute } from '@tanstack/react-router'
import { useCallback, useState } from 'react'

export const Route = createFileRoute('/')({
  loader: async () => {
    const data = await apiClient.get<GetArchivesResponse>('/archives')
    return data.archives
  },
  component: Index,
})

function Index() {
  const initialArchives = Route.useLoaderData()
  const [ selectedArchive, setSelectedArchive ] = useState("")
  const [archives, setArchives] = useState<Archive[]>(initialArchives)
  const [loading, setLoading] = useState(false)

  const refreshArchives = useCallback(async () => {
    setLoading(true)
    try {
      const data = await apiClient.get<GetArchivesResponse>('/archives')
      setArchives(data.archives)
    } catch (error) {
      console.error(error)
    } finally {
      setLoading(false)
    }
  }, [])

  return (
    <SidebarProvider className="h-full min-h-0 relative">
      <AppSidebar
        onArchiveSelected={setSelectedArchive}
        selectedArchive={selectedArchive}
        archives={archives}
        loading={loading}
        onRefresh={refreshArchives}
      />
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
