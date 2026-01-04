import {
  Sidebar,
  SidebarContent,
  SidebarFooter,
  SidebarGroup,
  SidebarGroupAction,
  SidebarGroupContent,
  SidebarGroupLabel,
  SidebarHeader,
  SidebarMenu,
  SidebarMenuButton,
  SidebarMenuItem,
} from "@/components/ui/sidebar"
import { apiClient } from "@/lib/api"
import type { Archive, GetArchivesResponse } from "@/models/archive"
import { File, RefreshCw } from "lucide-react"
import { useEffect, useState, type Dispatch, type SetStateAction } from "react"

interface Props {
  onArchiveSelected: Dispatch<SetStateAction<string>>
  selectedArchive: string
}

export function AppSidebar({ onArchiveSelected, selectedArchive }: Props) {

  const [ archives, setArchives ] = useState<Archive[] | null >(null)
  const [ loading, setLoading ] = useState(false)

  const fetchArchives = async () => {
    setLoading(true)
    try {
      const data = await apiClient.get('/archives') as GetArchivesResponse;
      setArchives(data.archives);  
    } catch (error) {
      console.error(error);
    } finally {
      setLoading(false)
    }
  }

  useEffect(() => {
    fetchArchives();
  }, [])

  return (
    <Sidebar className="absolute h-full border-r">
      <SidebarHeader>
        <h2 className="font-bold text-center pt-2">Archives</h2>
      </SidebarHeader>
      <SidebarContent>
        <SidebarGroup>
          <SidebarGroupLabel>Archive List</SidebarGroupLabel>
          <SidebarGroupAction onClick={fetchArchives} title="Refresh Archives">
            <RefreshCw className={loading ? "animate-spin" : ""} />
          </SidebarGroupAction>
          <SidebarGroupContent>
            <SidebarMenu>
              { archives ?
                archives.map((archive, index) => (
                <SidebarMenuItem key={index}>
                  <SidebarMenuButton 
                    isActive={selectedArchive === archive.name}
                    onClick={() => {onArchiveSelected(archive.name)}}
                  >
                    <File />
                    <span>{archive.name.slice(0, -5)}</span>
                  </SidebarMenuButton>
                </SidebarMenuItem>
                )) : "Loading..."
              }
            </SidebarMenu>
          </SidebarGroupContent>
        </SidebarGroup>
      </SidebarContent>
      <SidebarFooter />
    </Sidebar>
  )
}