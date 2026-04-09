import type { Archive } from '@/models/archive'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Separator } from '@/components/ui/separator'
import { Tooltip, TooltipContent, TooltipTrigger } from '@/components/ui/tooltip'
import { cn } from '@/lib/utils'

interface ArchiveTimelineProps {
  archives: Archive[]
  selectedArchive: string
  rangeStart: Date
  rangeEnd: Date
  onSelect: (archiveName: string) => void
  onRangeChange: (start: Date, end: Date) => void
}

export function ArchiveTimeline({
  archives,
  selectedArchive,
  rangeStart,
  rangeEnd,
  onSelect,
  onRangeChange
}: ArchiveTimelineProps) {
  
  const handlePreset = (days: number | 'all') => {
    const end = new Date()
    end.setHours(23, 59, 59, 999)
    if (days === 'all') {
      if (archives.length > 0) {
        onRangeChange(new Date(archives[0].created_at), new Date(archives[archives.length - 1].created_at))
      }
    } else {
      const start = new Date(end)
      start.setDate(end.getDate() - days)
      start.setHours(0, 0, 0, 0)
      onRangeChange(start, end)
    }
  }

  const rangeStartMs = rangeStart.getTime()
  const rangeEndMs = rangeEnd.getTime()
  const rangeDuration = Math.max(1, rangeEndMs - rangeStartMs) // Prevent division by zero

  const visibleArchives = archives.filter(a => {
    const time = new Date(a.created_at).getTime()
    return time >= rangeStartMs && time <= rangeEndMs
  })

  // Ensure dates input values are in YYYY-MM-DD
  const formatYMD = (d: Date) => {
    return `${d.getFullYear()}-${String(d.getMonth() + 1).padStart(2, '0')}-${String(d.getDate()).padStart(2, '0')}`
  }

  const formatDisplayDate = (d: Date) => {
    return new Intl.DateTimeFormat('en-US', { year: 'numeric', month: 'short', day: 'numeric' }).format(d)
  }

  const formatDisplayDateTime = (d: Date) => {
     return new Intl.DateTimeFormat('en-US', { year: 'numeric', month: 'short', day: 'numeric', hour: '2-digit', minute: '2-digit', second: '2-digit' }).format(d)
  }

  return (
    <div className="flex flex-col gap-4 p-4 border-t bg-card text-card-foreground">
      <div className="flex items-center justify-between gap-4 flex-wrap">
        <div className="flex items-center gap-2">
          <Button variant="outline" size="sm" onClick={() => handlePreset(7)}>7d</Button>
          <Button variant="outline" size="sm" onClick={() => handlePreset(30)}>30d</Button>
          <Button variant="outline" size="sm" onClick={() => handlePreset(365)}>1y</Button>
          <Button variant="outline" size="sm" onClick={() => handlePreset('all')}>All</Button>
        </div>
        <div className="flex items-center gap-2">
          <span className="text-sm font-medium">From:</span>
          <Input 
            type="date" 
            className="w-auto h-8 text-sm"
            value={formatYMD(rangeStart)}
            onChange={(e) => {
               if (e.target.value) {
                 const newStart = new Date(e.target.value + "T00:00:00") // avoid timezone issues
                 if (!isNaN(newStart.getTime()) && newStart <= rangeEnd) {
                   onRangeChange(newStart, rangeEnd)
                 }
               }
            }}
          />
          <span className="text-sm font-medium">To:</span>
          <Input 
            type="date" 
            className="w-auto h-8 text-sm"
            value={formatYMD(rangeEnd)}
            onChange={(e) => {
               if (e.target.value) {
                 const newEnd = new Date(e.target.value + "T23:59:59")
                 if (!isNaN(newEnd.getTime()) && newEnd >= rangeStart) {
                   onRangeChange(rangeStart, newEnd)
                 }
               }
            }}
          />
        </div>
      </div>
      
      <Separator />

      <div className="relative h-12 w-full flex items-center">
        {/* Timeline Track line */}
        <div className="absolute w-full h-0.5 bg-muted"></div>
        
        {visibleArchives.map((archive) => {
          const time = new Date(archive.created_at).getTime()
          let ratio = (time - rangeStartMs) / rangeDuration
          ratio = Math.max(0, Math.min(1, ratio)) // Clamp between 0 and 1
          const leftPercent = ratio * 100
          const isSelected = archive.name === selectedArchive

          return (
            <Tooltip key={archive.id}>
              <TooltipTrigger asChild>
                <Button
                  variant="ghost"
                  size="icon"
                  className="absolute w-6 h-6 -ml-3 rounded-full hover:bg-accent hover:scale-125 transition-transform"
                  style={{ left: `${leftPercent}%` }}
                  onClick={() => onSelect(archive.name)}
                >
                  <div className={cn(
                    "w-3 h-3 rounded-full shadow-sm",
                    isSelected ? "bg-primary" : "bg-muted-foreground"
                  )} />
                </Button>
              </TooltipTrigger>
              <TooltipContent className="flex flex-col gap-1 z-50">
                <span className="font-semibold">{archive.name || "Unnamed"}</span>
                <span className="text-xs text-muted-foreground">{formatDisplayDateTime(new Date(archive.created_at))}</span>
              </TooltipContent>
            </Tooltip>
          )
        })}
      </div>

      <div className="flex justify-between text-xs text-muted-foreground">
        <span>{formatDisplayDate(rangeStart)}</span>
        <span>{visibleArchives.length} archives visible</span>
        <span>{formatDisplayDate(rangeEnd)}</span>
      </div>
    </div>
  )
}
