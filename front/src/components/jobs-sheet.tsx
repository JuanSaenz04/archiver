import {
  Sheet,
  SheetContent,
  SheetDescription,
  SheetHeader,
  SheetTitle,
  SheetTrigger,
} from "@/components/ui/sheet"
import { Button } from "@/components/ui/button"
import { List, RefreshCw } from "lucide-react"
import { useEffect, useState } from "react"
import { apiClient } from "@/lib/api"
import type { Job, GetJobsResponse } from "@/models/job"
import { cn } from "@/lib/utils"

export function JobsSheet() {
  const [jobs, setJobs] = useState<Job[]>([])
  const [isOpen, setIsOpen] = useState(false)
  const [isLoading, setIsLoading] = useState(false)

  const fetchJobs = async () => {
    setIsLoading(true)
    try {
      const data = await apiClient.get('/jobs') as GetJobsResponse
      // Check if data is array (handle API potentially wrapping it or not, though code says direct)
      if (Array.isArray(data)) {
        setJobs(data)
      } else {
        // Fallback if backend changes to { jobs: [...] } without us knowing
        // @ts-ignore
        setJobs(data.jobs || [])
      }
    } catch (error) {
      console.error("Failed to fetch jobs:", error)
    } finally {
      setIsLoading(false)
    }
  }

  useEffect(() => {
    if (isOpen) {
      fetchJobs()
    }
  }, [isOpen])

  return (
    <Sheet open={isOpen} onOpenChange={setIsOpen}>
      <SheetTrigger asChild>
        <Button size="icon" variant="outline">
          <List />
        </Button>
      </SheetTrigger>
      <SheetContent>
        <SheetHeader>
          <div className="flex items-center gap-2">
            <SheetTitle>Jobs</SheetTitle>
            <Button variant="ghost" size="icon-sm" onClick={fetchJobs} disabled={isLoading} className="size-8">
                <RefreshCw className={cn("size-4", isLoading ? "animate-spin" : "")} />
            </Button>
          </div>
          <SheetDescription>
            Current status of crawling jobs.
          </SheetDescription>
        </SheetHeader>
        <div className="mt-4 flex flex-col gap-2 h-full overflow-y-auto pb-8">
            {jobs.length === 0 ? (
                <div className="text-center text-sm text-muted-foreground mt-8">No jobs found.</div>
            ) : (
                jobs.map((job) => (
                    <div key={job.id} className="mr-2 ml-2 p-3 border rounded-md text-sm">
                        <div className="font-medium truncate" title={job.url}>{job.url}</div>
                        <div className="flex items-center justify-between mt-2">
                             <span className="text-xs text-muted-foreground font-mono">{job.id.slice(0,8)}</span>
                             <span className={`text-[10px] uppercase font-bold px-2 py-0.5 rounded-full ${
                                 job.status === 'completed' ? 'bg-green-100 text-green-800 dark:bg-green-900/30 dark:text-green-400' :
                                 job.status === 'failed' ? 'bg-red-100 text-red-800 dark:bg-red-900/30 dark:text-red-400' :
                                 'bg-blue-100 text-blue-800 dark:bg-blue-900/30 dark:text-blue-400'
                             }`}>
                                 {job.status}
                             </span>
                        </div>
                    </div>
                ))
            )}
        </div>
      </SheetContent>
    </Sheet>
  )
}
