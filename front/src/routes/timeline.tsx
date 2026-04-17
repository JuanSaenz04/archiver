import { createFileRoute, useNavigate } from '@tanstack/react-router'
import { useState, useEffect, useMemo } from 'react'
import { apiClient } from '@/lib/api'
import type { Archive, GetArchivesResponse } from '@/models/archive'
import { Input } from '@/components/ui/input'
import { Button } from '@/components/ui/button'
import { ArchiveViewer } from '@/components/archive-viewer'
import { ArchiveTimeline } from '@/components/archive-timeline'
import { Search, ArrowLeft } from 'lucide-react'

export const Route = createFileRoute('/timeline')({
  component: TimelinePage,
})

function TimelinePage() {
  const navigate = useNavigate()
  const [urlInput, setUrlInput] = useState('')
  const [submittedUrl, setSubmittedUrl] = useState('')
  const [archives, setArchives] = useState<Archive[]>([])
  const [selectedArchive, setSelectedArchive] = useState<string>('')
  
  const [rangeOverride, setRangeOverride] = useState<{ start: Date; end: Date } | null>(null)

  // Fetch archives once on mount
  useEffect(() => {
    apiClient.get('/archives')
      .then((res: unknown) => {
        const data = res as GetArchivesResponse;
        // Sort ascending by date
        const sorted = (data.archives || []).sort((a, b) => 
          new Date(a.created_at).getTime() - new Date(b.created_at).getTime()
        )
        setArchives(sorted)
      })
      .catch(console.error)
  }, [])

  const filteredArchives = useMemo(() => {
    if (!submittedUrl) return archives
    return archives.filter(a => a.source_url.includes(submittedUrl))
  }, [archives, submittedUrl])

  // Derive the default range bounds from filtered archives
  const defaultRange = useMemo(() => {
    if (filteredArchives.length > 0) {
      return {
        start: new Date(filteredArchives[0].created_at),
        end: new Date(filteredArchives[filteredArchives.length - 1].created_at)
      }
    }
    const today = new Date()
    const lastWeek = new Date()
    lastWeek.setDate(today.getDate() - 7)
    return { start: lastWeek, end: today }
  }, [filteredArchives])

  const effectiveRange = rangeOverride ?? defaultRange

  const handleSearch = (e: React.FormEvent) => {
    e.preventDefault()
    setSubmittedUrl(urlInput)
    setRangeOverride(null) // reset range to follow new dataset
  }

  return (
    <div className="flex flex-col h-full w-full bg-background relative z-0">
      {/* Top spacing to account for absolute positioned global top bar in RootLayout */}
      <div className="h-14 shrink-0 w-full" />
      
      <div className="px-4 pb-2 shrink-0 space-y-2 max-w-4xl mx-auto w-full">
        <div className="flex items-center gap-2 mb-2">
          <Button variant="ghost" size="icon" onClick={() => navigate({ to: '/' })}>
            <ArrowLeft className="size-5" />
          </Button>
          <h1 className="text-xl font-bold tracking-tight">Timeline</h1>
        </div>
        <form onSubmit={handleSearch} className="flex flex-col sm:flex-row items-stretch sm:items-center gap-2">
          <Input
            placeholder="Search by URL (e.g. https://example.com)"
            value={urlInput}
            onChange={(e) => setUrlInput(e.target.value)}
            className="flex-1"
          />
          <Button type="submit" className="w-full sm:w-auto">
            <Search className="size-4 mr-2" />
            Search
          </Button>
        </form>
        <p className="text-sm text-muted-foreground pl-1">
          {filteredArchives.length} archive{filteredArchives.length === 1 ? '' : 's'} found{submittedUrl ? ' for this URL' : ''}
        </p>
      </div>

      <div className="flex-1 min-h-0 px-4 pb-4">
        <ArchiveViewer archiveName={selectedArchive} />
      </div>

      <div className="shrink-0 z-10 bg-background">
        <ArchiveTimeline 
          archives={filteredArchives}
          selectedArchive={selectedArchive}
          rangeStart={effectiveRange.start}
          rangeEnd={effectiveRange.end}
          onSelect={setSelectedArchive}
          onRangeChange={(start, end) => {
            setRangeOverride({ start, end })
          }}
        />
      </div>
    </div>
  )
}
