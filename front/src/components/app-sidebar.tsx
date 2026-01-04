import {
  Sidebar,
  SidebarContent,
  SidebarFooter,
  SidebarGroup,
  SidebarGroupContent,
  SidebarHeader,
  SidebarMenu,
  SidebarMenuItem,
} from "@/components/ui/sidebar"
import { apiClient } from "@/lib/api"
import type { Archive, GetArchivesResponse } from "@/models/archive"
import { useEffect, useState } from "react"

export function AppSidebar() {

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
    <Sidebar>
      <SidebarHeader />
      <SidebarContent>
        <SidebarGroup>
          <SidebarGroupContent>
            <SidebarMenu>
              { archives ?
                archives.map(archive => <SidebarMenuItem>{archive.name}</SidebarMenuItem>) :
                "Loading..."
              }
            </SidebarMenu>
          </SidebarGroupContent>
        </SidebarGroup>
      </SidebarContent>
      <SidebarFooter />
    </Sidebar>
  )
}