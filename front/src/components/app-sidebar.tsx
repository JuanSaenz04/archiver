import {
  Sidebar,
  SidebarContent,
  SidebarFooter,
  SidebarGroup,
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
import { Button } from "@/components/ui/button"
import { Badge } from "@/components/ui/badge"
import { apiClient } from "@/lib/api"
import type { Archive, GetArchivesResponse } from "@/models/archive"
import { File, RefreshCw, Info, Search, Filter, X, Tag } from "lucide-react"
import { useEffect, useState, useMemo, type Dispatch, type SetStateAction } from "react"
import { ArchiveDetailsDialog } from "./archive-details-dialog"
import { cn } from "@/lib/utils"
import {
  Popover,
  PopoverContent,
  PopoverTrigger,
} from "@/components/ui/popover"

interface Props {
  onArchiveSelected: Dispatch<SetStateAction<string>>
  selectedArchive: string
}

export function AppSidebar({ onArchiveSelected, selectedArchive }: Props) {

  const [ archives, setArchives ] = useState<Archive[] | null >(null)
  const [ loading, setLoading ] = useState(false)
  const [ searchQuery, setSearchQuery ] = useState("")
  const [ selectedTags, setSelectedTags ] = useState<string[]>([])

  // Dialog state
  const [ detailArchive, setDetailArchive ] = useState<Archive | null>(null)
  const [ isDialogOpen, setIsDialogOpen ] = useState(false)

  const allTags = useMemo(() => {
    if (!archives) return []
    const tags = new Set<string>()
    archives.forEach(a => a.tags?.forEach(t => tags.add(t)))
    return Array.from(tags).sort()
  }, [archives])

  const filteredArchives = useMemo(() => {
    if (!archives) return null

    return archives.filter(archive => {
      // Search text match (name, description, tags)
      const searchLower = searchQuery.toLowerCase()
      const matchesSearch = !searchQuery || 
        archive.name.toLowerCase().includes(searchLower) ||
        archive.description?.toLowerCase().includes(searchLower) ||
        archive.tags?.some(tag => tag.toLowerCase().includes(searchLower))

      // Tag filter match (AND semantics)
      const matchesTags = selectedTags.length === 0 || 
        selectedTags.every(tag => archive.tags?.includes(tag))

      return matchesSearch && matchesTags
    })
  }, [archives, searchQuery, selectedTags])

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

  const handleArchiveDeleted = (archiveId: string) => {
    if (archives) {
      const deleted = archives.find(a => a.id === archiveId)
      if (deleted && selectedArchive === deleted.name) {
        onArchiveSelected("")
      }
      setArchives(archives.filter(a => a.id !== archiveId))
    }
  }

  const handleArchiveUpdated = (updatedArchive: Archive) => {
    if (archives) {
      // Find the old archive by ID
      setArchives(archives.map(a => 
        a.id === updatedArchive.id ? updatedArchive : a
      ))

      // If the currently selected archive was the one updated (potentially renamed)
      // We must check by old name before update, or just check by ID if we had ID
      const oldArchive = archives.find(a => a.id === updatedArchive.id)
      if (oldArchive && selectedArchive === oldArchive.name) {
        onArchiveSelected(updatedArchive.name)
      }

      // Update the detailArchive to reflect changes in the dialog immediately
      if (detailArchive?.id === updatedArchive.id) {
        setDetailArchive(updatedArchive)
      }
    }
  }

  const toggleTag = (tag: string) => {
    setSelectedTags(prev => 
      prev.includes(tag) ? prev.filter(t => t !== tag) : [...prev, tag]
    )
  }

  const clearFilters = () => {
    setSearchQuery("")
    setSelectedTags([])
  }

  useEffect(() => {
    fetchArchives();
  }, [])

  return (
    <Sidebar className="absolute h-full border-r">
      <SidebarHeader>
        <div className="flex items-center justify-between px-4 pt-4 pb-2">
            <h2 className="font-bold text-xl tracking-tight">Archives</h2>
            <div className="flex items-center gap-1">
                { (searchQuery || selectedTags.length > 0) && (
                    <Button variant="ghost" size="icon" className="size-8" onClick={clearFilters}>
                        <X className="size-4" />
                    </Button>
                )}
                <Button variant="ghost" size="icon" className="size-8" onClick={fetchArchives} disabled={loading}>
                    <RefreshCw className={cn("size-4", loading && "animate-spin")} />
                </Button>
            </div>
        </div>

        <div className="px-4 pb-4 space-y-2">
          <div className="flex items-center gap-2">
            <div className="relative group flex-1">
              <Search className="absolute left-2.5 top-2.5 size-4 text-muted-foreground transition-colors group-focus-within:text-primary" />
              <Input 
                  placeholder="Search archives..." 
                  value={searchQuery}
                  onChange={(e) => setSearchQuery(e.target.value)}
                  className="pl-9 h-9 text-sm bg-muted/50 border-none focus-visible:ring-1 focus-visible:ring-ring"
              />
            </div>
            
            <Popover>
                <PopoverTrigger asChild>
                    <Button variant="outline" size="sm" className={cn("h-9 px-3 text-xs gap-1", selectedTags.length > 0 && "bg-primary/10 border-primary/20")}>
                        <Filter className="size-3" />
                        Tags
                        {selectedTags.length > 0 && (
                            <Badge variant="secondary" className="h-4 px-1 text-[10px] ml-1 bg-primary text-primary-foreground hover:bg-primary/90">
                                {selectedTags.length}
                            </Badge>
                        )}
                    </Button>
                </PopoverTrigger>
                <PopoverContent className="w-56 p-2" align="end">
                    <div className="space-y-2">
                        <div className="text-xs font-semibold px-2 py-1 text-muted-foreground uppercase tracking-wider">Filter by Tags</div>
                        <div className="max-h-60 overflow-y-auto space-y-1 pr-1">
                            {allTags.length > 0 ? (
                                allTags.map(tag => (
                                    <button
                                        key={tag}
                                        onClick={() => toggleTag(tag)}
                                        className={cn(
                                            "flex items-center justify-between w-full px-2 py-1.5 text-sm rounded-md transition-colors",
                                            selectedTags.includes(tag) 
                                                ? "bg-primary text-primary-foreground" 
                                                : "hover:bg-muted"
                                        )}
                                    >
                                        <div className="flex items-center gap-2">
                                            <Tag className="size-3" />
                                            <span>{tag}</span>
                                        </div>
                                        {selectedTags.includes(tag) && <X className="size-3" />}
                                    </button>
                                ))
                            ) : (
                                <div className="text-xs text-muted-foreground px-2 py-4 text-center">No tags found</div>
                            )}
                        </div>
                        {selectedTags.length > 0 && (
                            <Button 
                                variant="ghost" 
                                size="sm" 
                                className="w-full h-8 text-xs text-destructive hover:text-destructive hover:bg-destructive/10"
                                onClick={() => setSelectedTags([])}
                            >
                                Clear selected
                            </Button>
                        )}
                    </div>
                </PopoverContent>
            </Popover>
          </div>

          {selectedTags.length > 0 && (
            <div className="flex items-center gap-2 overflow-x-auto no-scrollbar py-1">
                {selectedTags.map(tag => (
                    <Badge 
                        key={tag} 
                        variant="secondary" 
                        className="h-6 gap-1 pr-1 pl-2 text-[11px] whitespace-nowrap bg-primary/10 text-primary border-primary/20"
                    >
                        {tag}
                        <button onClick={() => toggleTag(tag)} className="hover:bg-primary/20 rounded-full p-0.5">
                            <X className="size-3" />
                        </button>
                    </Badge>
                ))}
            </div>
          )}
        </div>
      </SidebarHeader>
      <SidebarContent>
        <SidebarGroup>
          <SidebarGroupLabel className="px-4">Archive List</SidebarGroupLabel>
          <SidebarGroupContent>
            <SidebarMenu className="px-2">
              { filteredArchives ? (
                filteredArchives.length > 0 ? (
                    filteredArchives.map((archive) => (
                    <SidebarMenuItem key={archive.id}>
                        <Tooltip>
                            <TooltipTrigger asChild>
                                <SidebarMenuButton 
                                    isActive={selectedArchive === archive.name}
                                    onClick={() => {onArchiveSelected(archive.name)}}
                                    className="group/btn data-[active=true]:bg-primary/10 data-[active=true]:text-primary"
                                >
                                    <File className={cn("size-4", selectedArchive === archive.name && "text-primary")} />
                                    <span className="truncate font-medium">{archive.name.replace(".wacz", "")}</span>
                                </SidebarMenuButton>
                            </TooltipTrigger>
                            <TooltipContent side="right">
                                {archive.name.replace(".wacz", "")}
                            </TooltipContent>
                        </Tooltip>
                        <SidebarMenuAction 
                            showOnHover 
                            onClick={() => {
                                setDetailArchive(archive)
                                setIsDialogOpen(true)
                            }}
                        >
                            <Info className="size-4" />
                        </SidebarMenuAction>
                    </SidebarMenuItem>
                    ))
                ) : (
                    <div className="px-4 py-8 text-center">
                        <div className="text-sm text-muted-foreground font-medium">No archives found</div>
                        <p className="text-xs text-muted-foreground/70 mt-1">Try adjusting your filters</p>
                    </div>
                )
              ) : (
                <div className="px-4 py-8 space-y-4">
                    {[1,2,3,4,5].map(i => (
                        <div key={i} className="flex items-center gap-2 animate-pulse">
                            <div className="size-4 bg-muted rounded" />
                            <div className="h-4 bg-muted rounded w-full" />
                        </div>
                    ))}
                </div>
              )}
            </SidebarMenu>
          </SidebarGroupContent>
        </SidebarGroup>
      </SidebarContent>
      <SidebarFooter />

      <ArchiveDetailsDialog 
        archive={detailArchive}
        open={isDialogOpen}
        onOpenChange={setIsDialogOpen}
        onDeleted={handleArchiveDeleted}
        onUpdated={handleArchiveUpdated}
      />
    </Sidebar>
  )
}