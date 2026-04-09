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
  
  const [rangeStart, setRangeStart] = useState<Date>(new Date())
  const [rangeEnd, setRangeEnd] = useState<Date>(new Date())

  // Fetch archives once on mount
  useEffect(() => {
    apiClient.get('/archives')
      .then((res: any) => {
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

  // Reset range when filtered archives change
  useEffect(() => {
    if (filteredArchives.length > 0) {
      setRangeStart(new Date(filteredArchives[0].created_at))
      setRangeEnd(new Date(filteredArchives[filteredArchives.length - 1].created_at))
    } else {
      const today = new Date()
      const lastWeek = new Date()
      lastWeek.setDate(today.getDate() - 7)
      setRangeStart(lastWeek)
      setRangeEnd(today)
    }
  }, [filteredArchives])

  const handleSearch = (e: React.FormEvent) => {
    e.preventDefault()
    setSubmittedUrl(urlInput)
    setSelectedArchive('') // reset selection on new search
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
        <form onSubmit={handleSearch} className="flex items-center gap-2">
          <Input
            placeholder="Search by URL (e.g. https://example.com)"
            value={urlInput}
            onChange={(e) => setUrlInput(e.target.value)}
            className="flex-1"
          />
          <Button type="submit">
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
          rangeStart={rangeStart}
          rangeEnd={rangeEnd}
          onSelect={setSelectedArchive}
          onRangeChange={(start, end) => {
            setRangeStart(start)
            setRangeEnd(end)
          }}
        />
      </div>
    </div>
  )
}
