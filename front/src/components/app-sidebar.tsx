import {
  Sidebar,
  SidebarContent,
  SidebarFooter,
  SidebarGroup,
  SidebarGroupContent,
  SidebarHeader,
  SidebarMenu,
  SidebarMenuButton,
  SidebarMenuItem,
} from "@/components/ui/sidebar"
import { apiClient } from "@/lib/api"
import type { Archive, GetArchivesResponse } from "@/models/archive"
import { File } from "lucide-react"
import { useEffect, useState, type Dispatch, type SetStateAction } from "react"

interface Props {
  onArchiveSelected: Dispatch<SetStateAction<string>>
  selectedArchive: string
}

export function AppSidebar({ onArchiveSelected, selectedArchive }: Props) {

  const [ archives, setArchives ] = useState<Archive[] | null >(null)

  const fetchArchives = async () => {
    try {
      const data = await apiClient.get('/archives') as GetArchivesResponse;
      setArchives(data.archives);  
    } catch (error) {
      console.error(error);
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