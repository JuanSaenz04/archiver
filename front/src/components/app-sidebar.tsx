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
  SidebarMenuAction,
  SidebarMenuButton,
  SidebarMenuItem,
} from "@/components/ui/sidebar"
import {
  Tooltip,
  TooltipContent,
  TooltipTrigger,
} from "@/components/ui/tooltip"
import { Input } from "@/components/ui/input"
import { apiClient } from "@/lib/api"
import type { Archive, GetArchivesResponse } from "@/models/archive"
import { File, RefreshCw, Trash2, Pencil, Check, X } from "lucide-react"
import { useEffect, useState, type Dispatch, type SetStateAction } from "react"

interface Props {
  onArchiveSelected: Dispatch<SetStateAction<string>>
  selectedArchive: string
}

export function AppSidebar({ onArchiveSelected, selectedArchive }: Props) {

  const [ archives, setArchives ] = useState<Archive[] | null >(null)
  const [ loading, setLoading ] = useState(false)
  const [ editingArchive, setEditingArchive ] = useState<string | null>(null)
  const [ editValue, setEditValue ] = useState("")
  const [ searchQuery, setSearchQuery ] = useState("")

  const filteredArchives = archives?.filter(archive => 
    archive.name.toLowerCase().includes(searchQuery.toLowerCase())
  ) ?? null

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

  const deleteArchive = async (archiveName: string) => {
    if (!confirm(`Are you sure you want to delete ${archiveName}?`)) return

    try {
      await apiClient.delete(`/archives/${archiveName}`)
      if (selectedArchive === archiveName) {
        onArchiveSelected("")
      }
      await fetchArchives()
    } catch (error) {
      console.error("Failed to delete archive:", error)
    }
  }

  const startEditing = (archiveName: string) => {
    setEditingArchive(archiveName)
    setEditValue(archiveName.slice(0, -5)) // Remove .wacz for editing
  }

  const cancelEditing = () => {
    setEditingArchive(null)
    setEditValue("")
  }

  const saveArchiveName = async () => {
    if (!editingArchive || !editValue.trim()) return

    try {
      await apiClient.put(`/archives/${editingArchive}`, { name: editValue })
      
      // If the currently selected archive was renamed, update the selection
      if (selectedArchive === editingArchive) {
         // The backend likely appends .wacz, so we predict the new name
         // This might be slightly risky if backend logic changes, but standard behavior is needed
         onArchiveSelected(editValue + ".wacz")
      }
      
      await fetchArchives()
      cancelEditing()
    } catch (error) {
      console.error("Failed to rename archive:", error)
      alert("Failed to rename archive. Name might be taken or invalid.")
    }
  }

  useEffect(() => {
    fetchArchives();
  }, [])

  return (
    <Sidebar className="absolute h-full border-r">
      <SidebarHeader>
        <h2 className="font-bold text-center pt-2">Archives</h2>
        <div className="px-2 pb-2">
          <Input 
            placeholder="Search archives..." 
            value={searchQuery}
            onChange={(e) => setSearchQuery(e.target.value)}
            className="h-8 text-xs"
          />
        </div>
      </SidebarHeader>
      <SidebarContent>
        <SidebarGroup>
          <SidebarGroupLabel>Archive List</SidebarGroupLabel>
          <SidebarGroupAction onClick={fetchArchives} title="Refresh Archives">
            <RefreshCw className={loading ? "animate-spin" : ""} />
          </SidebarGroupAction>
          <SidebarGroupContent>
            <SidebarMenu>
              { filteredArchives ?
                filteredArchives.map((archive, index) => (
                <SidebarMenuItem key={index}>
                  {editingArchive === archive.name ? (
                     <div className="flex items-center gap-1 p-1">
                        <Input 
                          value={editValue}
                          onChange={(e) => setEditValue(e.target.value)}
                          className="h-7 text-xs"
                          autoFocus
                          onKeyDown={(e) => {
                            if (e.key === "Enter") saveArchiveName()
                            if (e.key === "Escape") cancelEditing()
                          }}
                        />
                        <button onClick={saveArchiveName} className="p-1 hover:bg-sidebar-accent rounded-md">
                          <Check className="size-4 text-green-500" />
                        </button>
                        <button onClick={cancelEditing} className="p-1 hover:bg-sidebar-accent rounded-md">
                          <X className="size-4 text-red-500" />
                        </button>
                     </div>
                  ) : (
                    <>
                      <Tooltip>
                        <TooltipTrigger asChild>
                          <SidebarMenuButton 
                            isActive={selectedArchive === archive.name}
                            onClick={() => {onArchiveSelected(archive.name)}}
                            className="hover:pr-14!"
                          >
                            <File />
                            <span className="truncate min-w-0">{archive.name.slice(0, -5)}</span>
                          </SidebarMenuButton>
                        </TooltipTrigger>
                        <TooltipContent side="right">
                          {archive.name.slice(0, -5)}
                        </TooltipContent>
                      </Tooltip>
                      <SidebarMenuAction 
                        showOnHover 
                        className="right-7"
                        onClick={() => startEditing(archive.name)}
                      >
                        <Pencil />
                      </SidebarMenuAction>
                      <SidebarMenuAction 
                        showOnHover 
                        onClick={() => deleteArchive(archive.name)}
                      >
                        <Trash2 />
                      </SidebarMenuAction>
                    </>
                  )}
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