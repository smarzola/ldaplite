import { type FormEvent, type MouseEvent, useEffect, useMemo, useState } from "react"
import {
  AlertCircle,
  Copy,
  Database,
  Eye,
  FolderTree,
  KeyRound,
  Pencil,
  Plus,
  RefreshCw,
  Search,
  Settings2,
  ShieldCheck,
  Trash2,
  UserRound,
  Users,
  type LucideIcon,
} from "lucide-react"

import { Alert, AlertDescription, AlertTitle } from "@/components/ui/alert"
import {
  AlertDialog,
  AlertDialogAction,
  AlertDialogCancel,
  AlertDialogContent,
  AlertDialogDescription,
  AlertDialogFooter,
  AlertDialogHeader,
  AlertDialogMedia,
  AlertDialogTitle,
} from "@/components/ui/alert-dialog"
import { Badge } from "@/components/ui/badge"
import { Button } from "@/components/ui/button"
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card"
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog"
import {
  Empty,
  EmptyDescription,
  EmptyHeader,
  EmptyMedia,
  EmptyTitle,
} from "@/components/ui/empty"
import {
  Field,
  FieldDescription,
  FieldGroup,
  FieldLabel,
} from "@/components/ui/field"
import { Input } from "@/components/ui/input"
import {
  Pagination,
  PaginationContent,
  PaginationItem,
  PaginationLink,
  PaginationNext,
  PaginationPrevious,
} from "@/components/ui/pagination"
import {
  Select,
  SelectContent,
  SelectGroup,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select"
import { Separator } from "@/components/ui/separator"
import {
  Sheet,
  SheetContent,
  SheetDescription,
  SheetHeader,
  SheetTitle,
} from "@/components/ui/sheet"
import { Skeleton } from "@/components/ui/skeleton"
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from "@/components/ui/table"
import { Textarea } from "@/components/ui/textarea"

type Session = {
  baseDN: string
  userDN: string
  userID: string
  capabilities: string[]
  roles: {
    admin: boolean
    directoryRead: boolean
    directoryWrite: boolean
    passwordSelf: boolean
    passwordReset: boolean
  }
}

type EntrySummary = {
  dn: string
  type: DirectoryEntryType
  objectClass: string
  name: string
  description?: string
  mail?: string
  members?: string[]
  memberOf?: string[]
}

type LoadState = {
  session?: Session
  loading: boolean
  error?: string
}

type Notice = {
  kind: "success" | "error"
  text: string
}

type ViewId = "directory" | "users" | "groups" | "ous" | "admin" | "account"
type DirectorySearchType = "all" | "users" | "groups" | "ous"
type DirectoryEntryType = "entry" | "user" | "group" | "ou"

type DirectorySearchResponse = {
  baseDN: string
  query: string
  type: DirectorySearchType
  page: number
  pageSize: number
  total: number
  totalPages: number
  entries: EntrySummary[]
}

type EntryDetail = EntrySummary & {
  attributes: Record<string, string[]>
  createdAt?: string
  updatedAt?: string
}

type DirectoryDetailResponse = {
  baseDN: string
  entry: EntryDetail
}

type WorkflowType = "user" | "group" | "ou"

type AdminWorkflow =
  | { kind: "create"; entryType: WorkflowType }
  | { kind: "edit"; entry: EntryDetail }
  | { kind: "reset"; entry: EntrySummary }
  | { kind: "members"; entry: EntryDetail }

const protectedExtraAttributes = [
  "createtimestamp",
  "entryuuid",
  "memberof",
  "modifytimestamp",
  "objectclass",
  "userpassword",
]

type NavItem = {
  id: ViewId
  label: string
  description: string
  icon: LucideIcon
}

const capabilityLabels: Array<[keyof Session["roles"], string]> = [
  ["directoryRead", "Directory read"],
  ["directoryWrite", "Directory write"],
  ["admin", "Administration"],
  ["passwordSelf", "Own password"],
  ["passwordReset", "Password reset"],
]

export default function App() {
  const [state, setState] = useState<LoadState>({ loading: true })
  const [notice, setNotice] = useState<Notice>()
  const [mutating, setMutating] = useState(false)
  const [activeView, setActiveView] = useState<ViewId>(() => viewFromLocation())

  useEffect(() => {
    let cancelled = false
    void loadConsole().then((loaded) => {
      if (!cancelled) {
        setState(loaded)
      }
    })
    return () => {
      cancelled = true
    }
  }, [])

  useEffect(() => {
    function syncViewFromURL() {
      setActiveView(viewFromLocation())
    }

    window.addEventListener("popstate", syncViewFromURL)
    return () => window.removeEventListener("popstate", syncViewFromURL)
  }, [])

  const session = state.session
  const navItems = useMemo(() => buildNavItems(session), [session])
  const accessLabel = accessBadgeLabel(session)
  const currentView = useMemo(
    () => normalizeActiveView(activeView, navItems, session),
    [activeView, navItems, session]
  )
  const pageCopy = viewCopy(currentView)

  useEffect(() => {
    if (!state.loading && session && currentView !== activeView) {
      navigateToView(currentView, setActiveView, true)
    }
  }, [activeView, currentView, session, state.loading])

  async function runMutation(path: string, method: string, payload: unknown, success: string, reload = true) {
    setMutating(true)
    setNotice(undefined)
    try {
      await mutateJSON(path, method, payload)
      if (reload) {
        setState(await loadConsole())
      }
      setNotice({ kind: "success", text: success })
    } catch (error) {
      const message = error instanceof Error ? error.message : "The request failed."
      setNotice({
        kind: "error",
        text: message,
      })
      throw new Error(message)
    } finally {
      setMutating(false)
    }
  }

  return (
    <main className="min-h-svh overflow-x-hidden bg-background text-foreground">
      <div className="mx-auto grid min-h-svh w-full max-w-[100vw] min-w-0 lg:grid-cols-[17rem_minmax(0,1fr)]">
        <aside className="flex min-w-0 flex-col gap-4 border-b px-4 py-4 sm:px-6 lg:border-b-0 lg:border-r lg:px-5 lg:py-6">
          <div className="flex min-w-0 flex-col gap-1">
            <div className="flex items-center gap-2">
              <Database />
              <p className="text-sm font-semibold">LDAPLite</p>
            </div>
            <p className="break-all font-mono text-xs text-muted-foreground">
              {session?.baseDN ?? "Loading directory"}
            </p>
          </div>

          <nav aria-label="Primary" className="flex min-w-0 gap-2 overflow-x-auto pb-1 lg:flex-col lg:overflow-visible lg:pb-0">
            {navItems.map((item) => {
              const Icon = item.icon
              const active = item.id === currentView
              return (
                <Button
                  aria-current={active ? "page" : undefined}
                  className="shrink-0 justify-start"
                  key={item.id}
                  onClick={() => navigateToView(item.id, setActiveView)}
                  size="sm"
                  variant={active ? "default" : "ghost"}
                >
                  <Icon data-icon="inline-start" />
                  {item.label}
                </Button>
              )
            })}
          </nav>

          {session ? (
            <div className="hidden flex-col gap-2 lg:flex">
              <Separator />
              <Badge className="w-fit" variant={session.roles.admin ? "default" : "secondary"}>
                {accessLabel}
              </Badge>
            </div>
          ) : null}
        </aside>

        <div className="flex min-w-0 flex-col gap-6 px-4 py-6 sm:px-6 lg:px-8">
          <header className="flex min-w-0 flex-col gap-2 border-b pb-5">
            <p className="font-mono text-xs uppercase tracking-normal text-muted-foreground">
              {session?.userID ?? "Loading account"}
            </p>
            <div className="flex min-w-0 flex-col gap-1">
              <h1 className="text-2xl font-semibold tracking-normal">{pageCopy.title}</h1>
              <p className="max-w-2xl break-words text-sm leading-relaxed text-muted-foreground">
                {pageCopy.description}
              </p>
            </div>
          </header>

          {state.error ? (
            <Alert variant="destructive">
              <AlertCircle />
              <AlertTitle>Console unavailable</AlertTitle>
              <AlertDescription>{state.error}</AlertDescription>
            </Alert>
          ) : null}

          {notice ? (
            <Alert variant={notice.kind === "error" ? "destructive" : "default"}>
              {notice.kind === "error" ? <AlertCircle /> : <ShieldCheck />}
              <AlertTitle>{notice.kind === "error" ? "Request failed" : "Saved"}</AlertTitle>
              <AlertDescription>{notice.text}</AlertDescription>
            </Alert>
          ) : null}

          {state.loading ? (
            <ShellSkeleton />
          ) : session ? (
            <AppView
              activeView={currentView}
              mutating={mutating}
              onMutate={runMutation}
              onNotice={setNotice}
              session={session}
            />
          ) : null}
        </div>
      </div>
    </main>
  )
}

function AppView({
  activeView,
  mutating,
  onMutate,
  onNotice,
  session,
}: {
  activeView: ViewId
  mutating: boolean
  onMutate: (path: string, method: string, payload: unknown, success: string, reload?: boolean) => Promise<void>
  onNotice: (notice: Notice) => void
  session: Session
}) {
  if (activeView === "account") {
    return (
      <section className="grid gap-4 xl:grid-cols-[minmax(0,1fr)_20rem]">
        <AccountPanel disabled={mutating} onMutate={onMutate} session={session} />
        <SessionCard session={session} />
      </section>
    )
  }

  if (activeView === "admin" && session.roles.admin) {
    return (
      <AdminPanel
        baseDN={session.baseDN}
        onMutate={onMutate}
      />
    )
  }

  if (activeView === "users") {
    return (
      <DirectorySearchView
        fixedType="users"
        onMutate={onMutate}
        onNotice={onNotice}
        session={session}
      />
    )
  }

  if (activeView === "groups") {
    return (
      <DirectorySearchView
        fixedType="groups"
        onMutate={onMutate}
        onNotice={onNotice}
        session={session}
      />
    )
  }

  if (activeView === "ous") {
    return (
      <DirectorySearchView
        fixedType="ous"
        onMutate={onMutate}
        onNotice={onNotice}
        session={session}
      />
    )
  }

  return (
    <DirectorySearchView onMutate={onMutate} onNotice={onNotice} session={session} />
  )
}

function DirectorySearchView({
  fixedType,
  onMutate,
  onNotice,
  session,
}: {
  fixedType?: Exclude<DirectorySearchType, "all">
  onMutate: (path: string, method: string, payload: unknown, success: string, reload?: boolean) => Promise<void>
  onNotice: (notice: Notice) => void
  session: Session
}) {
  const [query, setQuery] = useState("")
  const [submittedQuery, setSubmittedQuery] = useState("")
  const [entryType, setEntryType] = useState<DirectorySearchType>(fixedType ?? "all")
  const [page, setPage] = useState(1)
  const [pageSize, setPageSize] = useState(10)
  const [retryKey, setRetryKey] = useState(0)
  const [search, setSearch] = useState<{
    data?: DirectorySearchResponse
    error?: string
    loading: boolean
  }>({ loading: true })
  const [selectedEntry, setSelectedEntry] = useState<EntrySummary>()
  const [detailRetryKey, setDetailRetryKey] = useState(0)
  const [detail, setDetail] = useState<{
    data?: DirectoryDetailResponse
    error?: string
    loading: boolean
  }>({ loading: false })
  const [workflow, setWorkflow] = useState<AdminWorkflow>()
  const [deleteTarget, setDeleteTarget] = useState<EntrySummary>()

  const effectiveType = fixedType ?? entryType

  function changeSearchQuery(value: string) {
    setQuery(value)
    setPage(1)
    setSubmittedQuery(value.trim())
  }

  useEffect(() => {
    setEntryType(fixedType ?? "all")
    setPage(1)
    setSelectedEntry(undefined)
  }, [fixedType])

  useEffect(() => {
    let cancelled = false
    const params = new URLSearchParams()
    params.set("q", submittedQuery)
    params.set("type", effectiveType)
    params.set("page", String(page))
    params.set("pageSize", String(pageSize))

    setSearch((current) => ({ data: current.data, loading: true }))
    void fetchJSON<DirectorySearchResponse>(`/api/directory/search?${params.toString()}`)
      .then((data) => {
        if (!cancelled) {
          setSearch({ data, loading: false })
        }
      })
      .catch((error) => {
        if (!cancelled) {
          setSearch({
            error: error instanceof Error ? error.message : "Unable to search directory.",
            loading: false,
          })
        }
      })

    return () => {
      cancelled = true
    }
  }, [effectiveType, page, pageSize, retryKey, submittedQuery])

  useEffect(() => {
    if (!selectedEntry) {
      setDetail({ loading: false })
      return
    }

    let cancelled = false
    setDetail((current) => ({ data: current.data, loading: true }))
    void fetchJSON<DirectoryDetailResponse>(`/api/directory/entry?dn=${encodeURIComponent(selectedEntry.dn)}`)
      .then((data) => {
        if (!cancelled) {
          setDetail({ data, loading: false })
        }
      })
      .catch((error) => {
        if (!cancelled) {
          setDetail({
            error: error instanceof Error ? error.message : "Unable to load entry details.",
            loading: false,
          })
        }
      })

    return () => {
      cancelled = true
    }
  }, [detailRetryKey, selectedEntry])

  function runSearch() {
    const input = document.getElementById("directory-search")
    const nextQuery = input instanceof HTMLInputElement ? input.value : query
    setPage(1)
    setSubmittedQuery(nextQuery.trim())
  }

  function submit(event: FormEvent) {
    event.preventDefault()
    runSearch()
  }

  async function copyDN(entry: EntrySummary) {
    try {
      await copyText(entry.dn)
      onNotice({ kind: "success", text: "DN copied." })
    } catch {
      onNotice({ kind: "error", text: "Could not copy the DN." })
    }
  }

  async function copyDetailValue(value: string, label: string) {
    try {
      await copyText(value)
      onNotice({ kind: "success", text: `${label} copied.` })
    } catch {
      onNotice({ kind: "error", text: `Could not copy ${label.toLowerCase()}.` })
    }
  }

  function resetSearch() {
    setQuery("")
    setSubmittedQuery("")
    if (!fixedType) {
      setEntryType("all")
    }
    setPage(1)
    setSelectedEntry(undefined)
  }

  async function runAdminMutation(path: string, method: string, payload: unknown, success: string) {
    try {
      await onMutate(path, method, payload, success)
      setWorkflow(undefined)
      setDeleteTarget(undefined)
      if (deleteTarget && selectedEntry?.dn === deleteTarget.dn) {
        setSelectedEntry(undefined)
      }
      setRetryKey((current) => current + 1)
      setDetailRetryKey((current) => current + 1)
    } catch {
      setDetailRetryKey((current) => current + 1)
    }
  }

  const data = search.data
  const range = data ? resultRange(data) : ""
  const title = fixedType ? `${directoryTypeLabel(fixedType)} search` : "Directory search"

  return (
    <div className="flex flex-col gap-4">
      <Card>
        <CardHeader>
          <div className="flex flex-col gap-2 sm:flex-row sm:items-start sm:justify-between">
            <div className="flex flex-col gap-1">
              <CardTitle>{title}</CardTitle>
              <CardDescription>
                Find entries by DN, uid, cn, mail, OU, group, description, or membership.
              </CardDescription>
            </div>
            {data ? <Badge variant="secondary">{data.total} result{data.total === 1 ? "" : "s"}</Badge> : null}
          </div>
        </CardHeader>
        <CardContent className="flex flex-col gap-4">
          {session.roles.admin ? (
            <AdminCreateActions
              fixedType={fixedType}
              onCreate={(entryType) => setWorkflow({ kind: "create", entryType })}
            />
          ) : null}

          <form className="flex flex-col gap-3 lg:flex-row lg:items-end" onSubmit={submit}>
            <Field className="min-w-0 flex-1">
              <FieldLabel htmlFor="directory-search">Search directory</FieldLabel>
              <div className="flex gap-2">
                <Input
                  id="directory-search"
                  onChange={(event) => changeSearchQuery(event.target.value)}
                  placeholder="uid, cn, mail, DN, group, OU"
                  value={query}
                />
                <Button onClick={runSearch} type="button">
                  <Search data-icon="inline-start" />
                  Search
                </Button>
              </div>
            </Field>

            {!fixedType ? (
              <Field>
                <FieldLabel>Type</FieldLabel>
                <Select
                  onValueChange={(value) => {
                    setEntryType(value as DirectorySearchType)
                    setPage(1)
                    setSelectedEntry(undefined)
                  }}
                  value={entryType}
                >
                  <SelectTrigger aria-label="Entry type" className="w-40">
                    <SelectValue />
                  </SelectTrigger>
                  <SelectContent>
                    <SelectGroup>
                      <SelectItem value="all">All entries</SelectItem>
                      <SelectItem value="users">Users</SelectItem>
                      <SelectItem value="groups">Groups</SelectItem>
                      <SelectItem value="ous">OUs</SelectItem>
                    </SelectGroup>
                  </SelectContent>
                </Select>
              </Field>
            ) : null}

            <Field>
              <FieldLabel>Page size</FieldLabel>
              <Select
                onValueChange={(value) => {
                  setPageSize(Number(value))
                  setPage(1)
                }}
                value={String(pageSize)}
              >
                <SelectTrigger aria-label="Page size" className="w-28">
                  <SelectValue />
                </SelectTrigger>
                <SelectContent>
                  <SelectGroup>
                    <SelectItem value="5">5</SelectItem>
                    <SelectItem value="10">10</SelectItem>
                    <SelectItem value="25">25</SelectItem>
                    <SelectItem value="50">50</SelectItem>
                  </SelectGroup>
                </SelectContent>
              </Select>
            </Field>
          </form>

          {submittedQuery || effectiveType !== "all" ? (
            <div className="flex flex-wrap items-center gap-2">
              {submittedQuery ? <Badge variant="secondary">Query: {submittedQuery}</Badge> : null}
              <Badge variant="secondary">Type: {directoryTypeLabel(effectiveType)}</Badge>
              <Button onClick={resetSearch} size="sm" type="button" variant="ghost">
                <RefreshCw data-icon="inline-start" />
                Reset
              </Button>
            </div>
          ) : null}
        </CardContent>
      </Card>

      {search.error ? (
        <Alert variant="destructive">
          <AlertCircle />
          <AlertTitle>Search failed</AlertTitle>
          <AlertDescription className="flex flex-col gap-3 sm:flex-row sm:items-center sm:justify-between">
            <span>{search.error}</span>
            <Button onClick={() => setRetryKey((current) => current + 1)} size="sm" type="button" variant="outline">
              Retry
            </Button>
          </AlertDescription>
        </Alert>
      ) : null}

      <Card>
        <CardHeader>
          <div className="flex flex-col gap-2 sm:flex-row sm:items-start sm:justify-between">
            <div className="flex flex-col gap-1">
              <CardTitle>Results</CardTitle>
              <CardDescription>{range || "Search results will appear here."}</CardDescription>
            </div>
            {data ? <Badge variant="secondary">Page {data.page} of {Math.max(data.totalPages, 1)}</Badge> : null}
          </div>
        </CardHeader>
        <CardContent className="flex flex-col gap-4">
          {search.loading ? (
            <ResultSkeleton />
          ) : data && data.entries.length > 0 ? (
            <>
              <SearchResults
                entries={data.entries}
                onCopyDN={copyDN}
                onSelect={setSelectedEntry}
                onAdmin={setSelectedEntry}
                selectedDN={selectedEntry?.dn}
                showAdminAction={session.roles.admin}
              />
              <ResultPagination
                page={data.page}
                pageSize={data.pageSize}
                total={data.total}
                totalPages={data.totalPages}
                onPageChange={setPage}
              />
            </>
          ) : (
            <Empty>
              <EmptyHeader>
                <EmptyMedia variant="icon">
                  <Search />
                </EmptyMedia>
                <EmptyTitle>No entries found</EmptyTitle>
                <EmptyDescription>Change the search text, type filter, or page size.</EmptyDescription>
              </EmptyHeader>
            </Empty>
          )}
        </CardContent>
      </Card>

      <DirectoryDetailSheet
        detail={detail}
        entry={selectedEntry}
        onCopyDN={copyDN}
        onCopyValue={copyDetailValue}
        onDelete={(entry) => setDeleteTarget(entry)}
        onEdit={(entry) => setWorkflow({ kind: "edit", entry })}
        onManageMembers={(entry) => setWorkflow({ kind: "members", entry })}
        onOpenChange={(open) => {
          if (!open) {
            setSelectedEntry(undefined)
          }
        }}
        onRetry={() => setDetailRetryKey((current) => current + 1)}
        onResetPassword={(entry) => setWorkflow({ kind: "reset", entry })}
        showAdminActions={session.roles.admin}
      />

      <AdminWorkflowDialog
        baseDN={session.baseDN}
        onOpenChange={(open) => {
          if (!open) {
            setWorkflow(undefined)
          }
        }}
        onSubmit={runAdminMutation}
        workflow={workflow}
      />

      <DeleteEntryDialog
        entry={deleteTarget}
        onOpenChange={(open) => {
          if (!open) {
            setDeleteTarget(undefined)
          }
        }}
        onSubmit={runAdminMutation}
      />
    </div>
  )
}

function SessionCard({ session }: { session: Session }) {
  return (
    <Card>
      <CardHeader>
        <CardTitle>Session details</CardTitle>
        <CardDescription>Current bind DN and access summary.</CardDescription>
      </CardHeader>
      <CardContent className="flex flex-col gap-3">
        <div className="flex items-center gap-2">
          <ShieldCheck />
          <span className="text-sm font-medium">{session.userID}</span>
        </div>
        <p className="break-all font-mono text-xs text-muted-foreground">{session.userDN}</p>
        <Separator />
        <div className="flex flex-wrap gap-2">
          {capabilityLabels.map(([key, label]) => (
            <Badge key={key} variant={session.roles[key] ? "default" : "secondary"}>
              {label}
            </Badge>
          ))}
        </div>
      </CardContent>
    </Card>
  )
}

function SearchResults({
  entries,
  onAdmin,
  onCopyDN,
  onSelect,
  selectedDN,
  showAdminAction,
}: {
  entries: EntrySummary[]
  onAdmin: (entry: EntrySummary) => void
  onCopyDN: (entry: EntrySummary) => void
  onSelect: (entry: EntrySummary) => void
  selectedDN?: string
  showAdminAction: boolean
}) {
  return (
    <>
      <div className="flex flex-col gap-3 md:hidden">
        {entries.map((entry) => (
          <div className="flex flex-col gap-3 border-b border-border pb-3 last:border-b-0 last:pb-0" key={entry.dn}>
            <div className="flex items-start justify-between gap-3">
              <div className="min-w-0">
                <div className="flex flex-wrap items-center gap-2">
                  <p className="font-medium">{entry.name || entry.dn}</p>
                  <Badge variant="secondary">{entryTypeLabel(entry.type)}</Badge>
                </div>
                <p className="mt-1 break-all font-mono text-xs leading-relaxed text-muted-foreground">
                  {entry.dn}
                </p>
              </div>
            </div>
            <p className="text-sm text-muted-foreground">{entrySummaryText(entry)}</p>
            <RowActions
              entry={entry}
              isSelected={selectedDN === entry.dn}
              onAdmin={onAdmin}
              onCopyDN={onCopyDN}
              onSelect={onSelect}
              showAdminAction={showAdminAction}
            />
          </div>
        ))}
      </div>

      <div className="hidden md:block">
        <Table>
          <TableHeader>
            <TableRow>
              <TableHead>Name</TableHead>
              <TableHead>Type</TableHead>
              <TableHead>DN</TableHead>
              <TableHead>Summary</TableHead>
              <TableHead className="w-52">Actions</TableHead>
            </TableRow>
          </TableHeader>
          <TableBody>
            {entries.map((entry) => (
              <TableRow data-state={selectedDN === entry.dn ? "selected" : undefined} key={entry.dn}>
                <TableCell className="font-medium">{entry.name || entry.dn}</TableCell>
                <TableCell>
                  <Badge variant="secondary">{entryTypeLabel(entry.type)}</Badge>
                </TableCell>
                <TableCell className="max-w-80 truncate font-mono text-xs text-muted-foreground">
                  {entry.dn}
                </TableCell>
                <TableCell>{entrySummaryText(entry)}</TableCell>
                <TableCell>
                  <RowActions
                    entry={entry}
                    isSelected={selectedDN === entry.dn}
                    onAdmin={onAdmin}
                    onCopyDN={onCopyDN}
                    onSelect={onSelect}
                    showAdminAction={showAdminAction}
                  />
                </TableCell>
              </TableRow>
            ))}
          </TableBody>
        </Table>
      </div>
    </>
  )
}

function RowActions({
  entry,
  isSelected,
  onAdmin,
  onCopyDN,
  onSelect,
  showAdminAction,
}: {
  entry: EntrySummary
  isSelected: boolean
  onAdmin: (entry: EntrySummary) => void
  onCopyDN: (entry: EntrySummary) => void
  onSelect: (entry: EntrySummary) => void
  showAdminAction: boolean
}) {
  return (
    <div className="flex flex-wrap gap-2">
      <Button onClick={() => onSelect(entry)} size="sm" type="button" variant={isSelected ? "default" : "outline"}>
        <Eye data-icon="inline-start" />
        View
      </Button>
      <Button onClick={() => onCopyDN(entry)} size="sm" type="button" variant="outline">
        <Copy data-icon="inline-start" />
        Copy DN
      </Button>
      {showAdminAction && isAdminWorkflowEntry(entry.type) ? (
        <Button onClick={() => onAdmin(entry)} size="sm" type="button" variant="ghost">
          <Settings2 data-icon="inline-start" />
          Actions
        </Button>
      ) : null}
    </div>
  )
}

function ResultPagination({
  onPageChange,
  page,
  pageSize,
  total,
  totalPages,
}: {
  onPageChange: (page: number) => void
  page: number
  pageSize: number
  total: number
  totalPages: number
}) {
  if (total <= pageSize && totalPages <= 1) {
    return null
  }

  const pages = visiblePages(page, Math.max(totalPages, 1))
  const canPrevious = page > 1
  const canNext = totalPages > 0 && page < totalPages

  function pageLink(targetPage: number) {
    return {
      href: `#page-${targetPage}`,
      onClick: (event: MouseEvent<HTMLAnchorElement>) => {
        event.preventDefault()
        onPageChange(targetPage)
      },
    }
  }

  return (
    <Pagination>
      <PaginationContent>
        <PaginationItem>
          <PaginationPrevious
            aria-disabled={!canPrevious}
            className={!canPrevious ? "pointer-events-none opacity-50" : undefined}
            {...pageLink(Math.max(1, page - 1))}
          />
        </PaginationItem>
        {pages.map((targetPage) => (
          <PaginationItem key={targetPage}>
            <PaginationLink isActive={targetPage === page} {...pageLink(targetPage)}>
              {targetPage}
            </PaginationLink>
          </PaginationItem>
        ))}
        <PaginationItem>
          <PaginationNext
            aria-disabled={!canNext}
            className={!canNext ? "pointer-events-none opacity-50" : undefined}
            {...pageLink(canNext ? page + 1 : page)}
          />
        </PaginationItem>
      </PaginationContent>
    </Pagination>
  )
}

function DirectoryDetailSheet({
  detail,
  entry,
  onCopyDN,
  onCopyValue,
  onDelete,
  onEdit,
  onManageMembers,
  onOpenChange,
  onRetry,
  onResetPassword,
  showAdminActions,
}: {
  detail: {
    data?: DirectoryDetailResponse
    error?: string
    loading: boolean
  }
  entry?: EntrySummary
  onCopyDN: (entry: EntrySummary) => void
  onCopyValue: (value: string, label: string) => void
  onDelete: (entry: EntrySummary) => void
  onEdit: (entry: EntryDetail) => void
  onManageMembers: (entry: EntryDetail) => void
  onOpenChange: (open: boolean) => void
  onRetry: () => void
  onResetPassword: (entry: EntrySummary) => void
  showAdminActions: boolean
}) {
  const loadedEntry = detail.data?.entry
  const displayEntry = loadedEntry ?? entry
  const parent = displayEntry ? parentDN(displayEntry.dn) : ""
  const rows = loadedEntry ? attributeRows(loadedEntry.attributes) : []
  const objectClasses = loadedEntry?.attributes.objectclass ?? (displayEntry ? [displayEntry.objectClass] : [])
  const members = loadedEntry?.members ?? []
  const memberOf = loadedEntry?.memberOf ?? []
  const canEdit = Boolean(loadedEntry && isAdminWorkflowEntry(loadedEntry.type))
  const canDelete = Boolean(displayEntry && isAdminWorkflowEntry(displayEntry.type))

  return (
    <Sheet open={Boolean(entry)} onOpenChange={onOpenChange}>
      <SheetContent className="overflow-y-auto data-[side=right]:w-full sm:max-w-2xl" side="right">
        <SheetHeader className="border-b">
          <div className="flex flex-col gap-3 pr-8">
            <div className="flex flex-wrap items-center gap-2">
              {displayEntry ? <Badge variant="secondary">{entryTypeLabel(displayEntry.type)}</Badge> : null}
              {displayEntry ? <Badge variant="secondary">{displayEntry.objectClass}</Badge> : null}
            </div>
            <div className="flex flex-col gap-1">
              <SheetTitle>{displayEntry?.name || "Entry details"}</SheetTitle>
              <SheetDescription>
                {displayEntry ? entrySummaryText(displayEntry) : "Loading directory entry."}
              </SheetDescription>
            </div>
          </div>
        </SheetHeader>

        {detail.loading ? (
          <div className="flex flex-col gap-3 p-4">
            <Skeleton className="h-16" />
            <Skeleton className="h-32" />
            <Skeleton className="h-48" />
          </div>
        ) : detail.error ? (
          <div className="p-4">
            <Alert variant="destructive">
              <AlertCircle />
              <AlertTitle>Could not load details</AlertTitle>
              <AlertDescription className="flex flex-col gap-3">
                <span>{detail.error}</span>
                <Button onClick={onRetry} size="sm" type="button" variant="outline">
                  Retry
                </Button>
              </AlertDescription>
            </Alert>
          </div>
        ) : displayEntry ? (
          <div className="flex flex-col gap-5 p-4">
            <section className="flex flex-col gap-3">
              <div className="flex flex-col gap-1">
                <p className="text-sm font-medium">DN</p>
                <p className="break-all font-mono text-xs leading-relaxed text-muted-foreground">
                  {displayEntry.dn}
                </p>
              </div>
              <div className="flex flex-wrap gap-2">
                <Button onClick={() => onCopyDN(displayEntry)} size="sm" type="button" variant="outline">
                  <Copy data-icon="inline-start" />
                  Copy DN
                </Button>
                {showAdminActions ? (
                  <>
                    {loadedEntry && canEdit ? (
                      <Button onClick={() => onEdit(loadedEntry)} size="sm" type="button" variant="outline">
                        <Pencil data-icon="inline-start" />
                        Edit entry
                      </Button>
                    ) : null}
                    {displayEntry.type === "user" ? (
                      <Button onClick={() => onResetPassword(displayEntry)} size="sm" type="button" variant="outline">
                        <KeyRound data-icon="inline-start" />
                        Reset password
                      </Button>
                    ) : null}
                    {displayEntry.type === "group" && loadedEntry ? (
                      <Button onClick={() => onManageMembers(loadedEntry)} size="sm" type="button" variant="outline">
                        <Users data-icon="inline-start" />
                        Manage members
                      </Button>
                    ) : null}
                    {canDelete ? (
                      <Button onClick={() => onDelete(displayEntry)} size="sm" type="button" variant="destructive">
                        <Trash2 data-icon="inline-start" />
                        Delete entry
                      </Button>
                    ) : null}
                  </>
                ) : null}
              </div>
            </section>

            <Separator />

            <section className="flex flex-col gap-3">
              <h2 className="text-sm font-medium">Directory path</h2>
              <div className="flex flex-col gap-2 rounded-md border p-3">
                {dnLineage(displayEntry.dn).map((part, index) => (
                  <div className="flex items-start gap-2" key={`${part}-${index}`}>
                    <Badge className="mt-0.5" variant={index === 0 ? "default" : "secondary"}>
                      {index === 0 ? "Entry" : "Parent"}
                    </Badge>
                    <p className="break-all font-mono text-xs leading-relaxed text-muted-foreground">{part}</p>
                  </div>
                ))}
              </div>
              {parent ? (
                <DetailValue label="Parent DN" value={parent} onCopy={(value) => onCopyValue(value, "Parent DN")} />
              ) : null}
            </section>

            <section className="grid gap-3 sm:grid-cols-2">
              <DetailList label="Object classes" values={objectClasses} onCopy={onCopyValue} />
              <DetailList label="Member of" empty="No group memberships" values={memberOf} onCopy={onCopyValue} />
              <DetailList label="Members" empty="No members" values={members} onCopy={onCopyValue} />
              <div className="flex flex-col gap-3 rounded-md border p-3">
                <h2 className="text-sm font-medium">Operational</h2>
                <DetailDatum label="Created" value={formatTimestamp(loadedEntry?.createdAt)} />
                <DetailDatum label="Modified" value={formatTimestamp(loadedEntry?.updatedAt)} />
              </div>
            </section>

            <section className="flex flex-col gap-3">
              <h2 className="text-sm font-medium">Attributes</h2>
              {rows.length > 0 ? (
                <div className="overflow-hidden rounded-md border">
                  <Table>
                    <TableHeader>
                      <TableRow>
                        <TableHead>Attribute</TableHead>
                        <TableHead>Value</TableHead>
                        <TableHead className="w-24">Action</TableHead>
                      </TableRow>
                    </TableHeader>
                    <TableBody>
                      {rows.map((row) => (
                        <TableRow key={`${row.name}-${row.value}`}>
                          <TableCell className="font-mono text-xs">{row.name}</TableCell>
                          <TableCell className="break-all font-mono text-xs text-muted-foreground">
                            {row.value}
                          </TableCell>
                          <TableCell>
                            <Button
                              aria-label={`Copy ${row.name}`}
                              onClick={() => onCopyValue(row.value, row.name)}
                              size="icon-sm"
                              type="button"
                              variant="ghost"
                            >
                              <Copy />
                            </Button>
                          </TableCell>
                        </TableRow>
                      ))}
                    </TableBody>
                  </Table>
                </div>
              ) : (
                <Empty>
                  <EmptyHeader>
                    <EmptyTitle>No safe attributes</EmptyTitle>
                    <EmptyDescription>This entry has no displayable attributes.</EmptyDescription>
                  </EmptyHeader>
                </Empty>
              )}
            </section>
          </div>
        ) : null}
      </SheetContent>
    </Sheet>
  )
}

function DetailValue({
  label,
  onCopy,
  value,
}: {
  label: string
  onCopy: (value: string) => void
  value: string
}) {
  return (
    <div className="flex flex-col gap-2 rounded-md border p-3">
      <div className="flex items-center justify-between gap-3">
        <p className="text-sm font-medium">{label}</p>
        <Button aria-label={`Copy ${label}`} onClick={() => onCopy(value)} size="icon-sm" type="button" variant="ghost">
          <Copy />
        </Button>
      </div>
      <p className="break-all font-mono text-xs leading-relaxed text-muted-foreground">{value}</p>
    </div>
  )
}

function TargetDN({ label, value }: { label: string; value: string }) {
  return (
    <div className="flex flex-col gap-1 rounded-md border p-3">
      <p className="text-sm font-medium">{label}</p>
      <p className="break-all font-mono text-xs leading-relaxed text-muted-foreground">{value}</p>
    </div>
  )
}

function DetailList({
  empty = "None",
  label,
  onCopy,
  values,
}: {
  empty?: string
  label: string
  onCopy: (value: string, label: string) => void
  values: string[]
}) {
  return (
    <div className="flex flex-col gap-3 rounded-md border p-3">
      <h2 className="text-sm font-medium">{label}</h2>
      {values.length > 0 ? (
        <div className="flex flex-col gap-2">
          {values.map((value) => (
            <div className="flex items-start justify-between gap-2" key={value}>
              <p className="break-all font-mono text-xs leading-relaxed text-muted-foreground">{value}</p>
              <Button aria-label={`Copy ${label}`} onClick={() => onCopy(value, label)} size="icon-sm" type="button" variant="ghost">
                <Copy />
              </Button>
            </div>
          ))}
        </div>
      ) : (
        <p className="text-sm text-muted-foreground">{empty}</p>
      )}
    </div>
  )
}

function DetailDatum({ label, value }: { label: string; value: string }) {
  return (
    <div className="flex flex-col gap-1">
      <p className="text-xs text-muted-foreground">{label}</p>
      <p className="font-mono text-xs">{value}</p>
    </div>
  )
}

function AdminCreateActions({
  fixedType,
  onCreate,
}: {
  fixedType?: Exclude<DirectorySearchType, "all">
  onCreate: (entryType: WorkflowType) => void
}) {
  const types: WorkflowType[] = fixedType === "users"
    ? ["user"]
    : fixedType === "groups"
      ? ["group"]
      : fixedType === "ous"
        ? ["ou"]
        : ["user", "group", "ou"]

  return (
    <div className="flex flex-wrap gap-2">
      {types.map((entryType) => (
        <Button key={entryType} onClick={() => onCreate(entryType)} size="sm" type="button" variant="outline">
          <Plus data-icon="inline-start" />
          Create {workflowTypeLabel(entryType)}
        </Button>
      ))}
    </div>
  )
}

function AdminWorkflowDialog({
  baseDN,
  onOpenChange,
  onSubmit,
  workflow,
}: {
  baseDN: string
  onOpenChange: (open: boolean) => void
  onSubmit: (path: string, method: string, payload: unknown, success: string) => Promise<void>
  workflow?: AdminWorkflow
}) {
  const title = workflow ? workflowTitle(workflow) : "Admin workflow"
  const description = workflow ? workflowDescription(workflow) : ""

  return (
    <Dialog open={Boolean(workflow)} onOpenChange={onOpenChange}>
      <DialogContent className="max-h-[calc(100svh-2rem)] overflow-y-auto sm:max-w-2xl">
        <DialogHeader>
          <DialogTitle>{title}</DialogTitle>
          <DialogDescription>{description}</DialogDescription>
        </DialogHeader>
        {workflow ? (
          <AdminWorkflowForm
            baseDN={baseDN}
            key={workflowKey(workflow)}
            onCancel={() => onOpenChange(false)}
            onSubmit={onSubmit}
            workflow={workflow}
          />
        ) : null}
      </DialogContent>
    </Dialog>
  )
}

function AdminWorkflowForm({
  baseDN,
  onCancel,
  onSubmit,
  workflow,
}: {
  baseDN: string
  onCancel: () => void
  onSubmit: (path: string, method: string, payload: unknown, success: string) => Promise<void>
  workflow: AdminWorkflow
}) {
  if (workflow.kind === "create") {
    if (workflow.entryType === "user") {
      return <CreateUserWorkflow baseDN={baseDN} onCancel={onCancel} onSubmit={onSubmit} />
    }
    if (workflow.entryType === "group") {
      return <CreateGroupWorkflow baseDN={baseDN} onCancel={onCancel} onSubmit={onSubmit} />
    }
    return <CreateOUWorkflow baseDN={baseDN} onCancel={onCancel} onSubmit={onSubmit} />
  }
  if (workflow.kind === "edit") {
    if (workflow.entry.type === "user") {
      return <EditUserWorkflow entry={workflow.entry} onCancel={onCancel} onSubmit={onSubmit} />
    }
    if (workflow.entry.type === "group") {
      return <EditGroupWorkflow entry={workflow.entry} onCancel={onCancel} onSubmit={onSubmit} />
    }
    return <EditOUWorkflow entry={workflow.entry} onCancel={onCancel} onSubmit={onSubmit} />
  }
  if (workflow.kind === "reset") {
    return <ResetPasswordWorkflow entry={workflow.entry} onCancel={onCancel} onSubmit={onSubmit} />
  }
  return <ManageMembersWorkflow entry={workflow.entry} onCancel={onCancel} onSubmit={onSubmit} />
}

function CreateUserWorkflow({
  baseDN,
  onCancel,
  onSubmit,
}: {
  baseDN: string
  onCancel: () => void
  onSubmit: (path: string, method: string, payload: unknown, success: string) => Promise<void>
}) {
  const [form, setForm] = useState({
    parentDN: `ou=users,${baseDN}`,
    uid: "",
    cn: "",
    sn: "",
    givenName: "",
    mail: "",
    password: "",
    attributes: "",
  })
  const [error, setError] = useState("")

  return (
    <form
      className="flex flex-col gap-4"
      onSubmit={(event) => {
        event.preventDefault()
        const missing = requiredMessage([
          ["Parent DN", form.parentDN],
          ["UID", form.uid],
          ["Common name", form.cn],
          ["Surname", form.sn],
          ["Initial password", form.password],
        ])
        if (missing) {
          setError(missing)
          return
        }
        void onSubmit("/api/users", "POST", { ...form, attributes: parseAttributes(form.attributes) }, "User created.")
      }}
    >
      <WorkflowError message={error} />
      <FieldGroup className="grid gap-4 md:grid-cols-2">
        <TextField id="create-user-parent" label="Parent DN" value={form.parentDN} onChange={(parentDN) => setForm({ ...form, parentDN })} />
        <TextField id="create-user-uid" label="UID" value={form.uid} onChange={(uid) => setForm({ ...form, uid })} />
        <TextField id="create-user-cn" label="Common name" value={form.cn} onChange={(cn) => setForm({ ...form, cn })} />
        <TextField id="create-user-sn" label="Surname" value={form.sn} onChange={(sn) => setForm({ ...form, sn })} />
        <TextField id="create-user-given" label="Given name" value={form.givenName} onChange={(givenName) => setForm({ ...form, givenName })} />
        <TextField id="create-user-mail" label="Email" value={form.mail} onChange={(mail) => setForm({ ...form, mail })} type="email" />
        <TextField id="create-user-password" label="Initial password" value={form.password} onChange={(password) => setForm({ ...form, password })} type="password" />
      </FieldGroup>
      <AttributesField id="create-user-attributes" value={form.attributes} onChange={(attributes) => setForm({ ...form, attributes })} />
      <WorkflowFooter onCancel={onCancel} submitLabel="Create user" />
    </form>
  )
}

function CreateGroupWorkflow({
  baseDN,
  onCancel,
  onSubmit,
}: {
  baseDN: string
  onCancel: () => void
  onSubmit: (path: string, method: string, payload: unknown, success: string) => Promise<void>
}) {
  const [form, setForm] = useState({
    parentDN: `ou=groups,${baseDN}`,
    cn: "",
    description: "",
    members: "",
    attributes: "",
  })
  const [error, setError] = useState("")

  return (
    <form
      className="flex flex-col gap-4"
      onSubmit={(event) => {
        event.preventDefault()
        const missing = requiredMessage([
          ["Parent DN", form.parentDN],
          ["CN", form.cn],
          ["Members", form.members],
        ])
        const memberError = validateMemberDNs(form.members)
        if (missing || memberError) {
          setError(missing || memberError)
          return
        }
        void onSubmit(
          "/api/groups",
          "POST",
          { ...form, members: lines(form.members), attributes: parseAttributes(form.attributes) },
          "Group created."
        )
      }}
    >
      <WorkflowError message={error} />
      <FieldGroup className="grid gap-4 md:grid-cols-2">
        <TextField id="create-group-parent" label="Parent DN" value={form.parentDN} onChange={(parentDN) => setForm({ ...form, parentDN })} />
        <TextField id="create-group-cn" label="CN" value={form.cn} onChange={(cn) => setForm({ ...form, cn })} />
        <TextField id="create-group-description" label="Description" value={form.description} onChange={(description) => setForm({ ...form, description })} />
      </FieldGroup>
      <LinesField id="create-group-members" label="Members" value={form.members} onChange={(members) => setForm({ ...form, members })} />
      <AttributesField id="create-group-attributes" value={form.attributes} onChange={(attributes) => setForm({ ...form, attributes })} />
      <WorkflowFooter onCancel={onCancel} submitLabel="Create group" />
    </form>
  )
}

function CreateOUWorkflow({
  baseDN,
  onCancel,
  onSubmit,
}: {
  baseDN: string
  onCancel: () => void
  onSubmit: (path: string, method: string, payload: unknown, success: string) => Promise<void>
}) {
  const [form, setForm] = useState({ parentDN: baseDN, ou: "", description: "", attributes: "" })
  const [error, setError] = useState("")

  return (
    <form
      className="flex flex-col gap-4"
      onSubmit={(event) => {
        event.preventDefault()
        const missing = requiredMessage([
          ["Parent DN", form.parentDN],
          ["OU", form.ou],
        ])
        if (missing) {
          setError(missing)
          return
        }
        void onSubmit("/api/ous", "POST", { ...form, attributes: parseAttributes(form.attributes) }, "OU created.")
      }}
    >
      <WorkflowError message={error} />
      <FieldGroup className="grid gap-4 md:grid-cols-2">
        <TextField id="create-ou-parent" label="Parent DN" value={form.parentDN} onChange={(parentDN) => setForm({ ...form, parentDN })} />
        <TextField id="create-ou-name" label="OU" value={form.ou} onChange={(ou) => setForm({ ...form, ou })} />
        <TextField id="create-ou-description" label="Description" value={form.description} onChange={(description) => setForm({ ...form, description })} />
      </FieldGroup>
      <AttributesField id="create-ou-attributes" value={form.attributes} onChange={(attributes) => setForm({ ...form, attributes })} />
      <WorkflowFooter onCancel={onCancel} submitLabel="Create OU" />
    </form>
  )
}

function EditUserWorkflow({
  entry,
  onCancel,
  onSubmit,
}: {
  entry: EntryDetail
  onCancel: () => void
  onSubmit: (path: string, method: string, payload: unknown, success: string) => Promise<void>
}) {
  const [form, setForm] = useState({
    cn: firstAttribute(entry, "cn") || entry.name,
    sn: firstAttribute(entry, "sn"),
    givenName: firstAttribute(entry, "givenname"),
    mail: firstAttribute(entry, "mail") || entry.mail || "",
    attributes: attributesToText(entry.attributes, ["uid", "cn", "sn", "givenname", "mail", ...protectedExtraAttributes]),
  })
  const [error, setError] = useState("")

  return (
    <form
      className="flex flex-col gap-4"
      onSubmit={(event) => {
        event.preventDefault()
        const missing = requiredMessage([
          ["Common name", form.cn],
          ["Surname", form.sn],
        ])
        if (missing) {
          setError(missing)
          return
        }
        void onSubmit(
          `/api/users?dn=${encodeURIComponent(entry.dn)}`,
          "PUT",
          { dn: entry.dn, ...form, attributes: parseAttributes(form.attributes) },
          "User updated."
        )
      }}
    >
      <WorkflowError message={error} />
      <TargetDN label="Target DN" value={entry.dn} />
      <FieldGroup className="grid gap-4 md:grid-cols-2">
        <TextField id="edit-user-cn" label="Common name" value={form.cn} onChange={(cn) => setForm({ ...form, cn })} />
        <TextField id="edit-user-sn" label="Surname" value={form.sn} onChange={(sn) => setForm({ ...form, sn })} />
        <TextField id="edit-user-given" label="Given name" value={form.givenName} onChange={(givenName) => setForm({ ...form, givenName })} />
        <TextField id="edit-user-mail" label="Email" value={form.mail} onChange={(mail) => setForm({ ...form, mail })} type="email" />
      </FieldGroup>
      <AttributesField id="edit-user-attributes" value={form.attributes} onChange={(attributes) => setForm({ ...form, attributes })} />
      <WorkflowFooter onCancel={onCancel} submitLabel="Save user" />
    </form>
  )
}

function EditGroupWorkflow({
  entry,
  onCancel,
  onSubmit,
}: {
  entry: EntryDetail
  onCancel: () => void
  onSubmit: (path: string, method: string, payload: unknown, success: string) => Promise<void>
}) {
  const [form, setForm] = useState({
    description: firstAttribute(entry, "description") || entry.description || "",
    members: entry.members?.join("\n") ?? "",
    attributes: attributesToText(entry.attributes, ["cn", "description", "member", ...protectedExtraAttributes]),
  })
  const [error, setError] = useState("")

  return (
    <form
      className="flex flex-col gap-4"
      onSubmit={(event) => {
        event.preventDefault()
        const memberError = validateMemberDNs(form.members)
        if (memberError) {
          setError(memberError)
          return
        }
        void onSubmit(
          `/api/groups?dn=${encodeURIComponent(entry.dn)}`,
          "PUT",
          { dn: entry.dn, description: form.description, members: lines(form.members), attributes: parseAttributes(form.attributes) },
          "Group updated."
        )
      }}
    >
      <WorkflowError message={error} />
      <TargetDN label="Target DN" value={entry.dn} />
      <TextField id="edit-group-description" label="Description" value={form.description} onChange={(description) => setForm({ ...form, description })} />
      <LinesField id="edit-group-members" label="Members" value={form.members} onChange={(members) => setForm({ ...form, members })} />
      <AttributesField id="edit-group-attributes" value={form.attributes} onChange={(attributes) => setForm({ ...form, attributes })} />
      <WorkflowFooter onCancel={onCancel} submitLabel="Save group" />
    </form>
  )
}

function EditOUWorkflow({
  entry,
  onCancel,
  onSubmit,
}: {
  entry: EntryDetail
  onCancel: () => void
  onSubmit: (path: string, method: string, payload: unknown, success: string) => Promise<void>
}) {
  const [form, setForm] = useState({
    description: firstAttribute(entry, "description") || entry.description || "",
    attributes: attributesToText(entry.attributes, ["ou", "description", ...protectedExtraAttributes]),
  })

  return (
    <form
      className="flex flex-col gap-4"
      onSubmit={(event) => {
        event.preventDefault()
        void onSubmit(
          `/api/ous?dn=${encodeURIComponent(entry.dn)}`,
          "PUT",
          { dn: entry.dn, description: form.description, attributes: parseAttributes(form.attributes) },
          "OU updated."
        )
      }}
    >
      <TargetDN label="Target DN" value={entry.dn} />
      <TextField id="edit-ou-description" label="Description" value={form.description} onChange={(description) => setForm({ ...form, description })} />
      <AttributesField id="edit-ou-attributes" value={form.attributes} onChange={(attributes) => setForm({ ...form, attributes })} />
      <WorkflowFooter onCancel={onCancel} submitLabel="Save OU" />
    </form>
  )
}

function ResetPasswordWorkflow({
  entry,
  onCancel,
  onSubmit,
}: {
  entry: EntrySummary
  onCancel: () => void
  onSubmit: (path: string, method: string, payload: unknown, success: string) => Promise<void>
}) {
  const [password, setPassword] = useState("")
  const [error, setError] = useState("")

  return (
    <form
      className="flex flex-col gap-4"
      onSubmit={(event) => {
        event.preventDefault()
        if (!password.trim()) {
          setError("New password is required.")
          return
        }
        void onSubmit("/api/users/password", "POST", { dn: entry.dn, password }, "Password reset.")
      }}
    >
      <WorkflowError message={error} />
      <TargetDN label="User DN" value={entry.dn} />
      <TextField id="reset-password-value" label="New password" value={password} onChange={setPassword} type="password" />
      <WorkflowFooter onCancel={onCancel} submitLabel="Reset password" />
    </form>
  )
}

function ManageMembersWorkflow({
  entry,
  onCancel,
  onSubmit,
}: {
  entry: EntryDetail
  onCancel: () => void
  onSubmit: (path: string, method: string, payload: unknown, success: string) => Promise<void>
}) {
  const [members, setMembers] = useState(entry.members?.join("\n") ?? "")
  const [error, setError] = useState("")

  return (
    <form
      className="flex flex-col gap-4"
      onSubmit={(event) => {
        event.preventDefault()
        const memberError = validateMemberDNs(members)
        if (memberError) {
          setError(memberError)
          return
        }
        void onSubmit(
          `/api/groups?dn=${encodeURIComponent(entry.dn)}`,
          "PUT",
          {
            dn: entry.dn,
            description: firstAttribute(entry, "description") || entry.description || "",
            members: lines(members),
            attributes: parseAttributes(attributesToText(entry.attributes, ["cn", "description", "member", ...protectedExtraAttributes])),
          },
          "Members updated."
        )
      }}
    >
      <WorkflowError message={error} />
      <TargetDN label="Group DN" value={entry.dn} />
      <LinesField id="manage-members-values" label="Members" value={members} onChange={setMembers} />
      <FieldDescription>Each member DN must point to an existing directory entry. The server validates references before saving.</FieldDescription>
      <WorkflowFooter onCancel={onCancel} submitLabel="Save members" />
    </form>
  )
}

function DeleteEntryDialog({
  entry,
  onOpenChange,
  onSubmit,
}: {
  entry?: EntrySummary
  onOpenChange: (open: boolean) => void
  onSubmit: (path: string, method: string, payload: unknown, success: string) => Promise<void>
}) {
  return (
    <AlertDialog open={Boolean(entry)} onOpenChange={onOpenChange}>
      <AlertDialogContent>
        <AlertDialogHeader>
          <AlertDialogMedia>
            <Trash2 />
          </AlertDialogMedia>
          <AlertDialogTitle>Delete entry</AlertDialogTitle>
          <AlertDialogDescription>
            {entry ? `Delete ${entry.dn}? This cannot be undone and may fail if child entries still exist.` : ""}
          </AlertDialogDescription>
        </AlertDialogHeader>
        <AlertDialogFooter>
          <AlertDialogCancel>Cancel</AlertDialogCancel>
          <AlertDialogAction
            onClick={() => {
              if (entry) {
                void onSubmit(deletePath(entry), "DELETE", undefined, "Entry deleted.")
              }
            }}
            variant="destructive"
          >
            Delete entry
          </AlertDialogAction>
        </AlertDialogFooter>
      </AlertDialogContent>
    </AlertDialog>
  )
}

function WorkflowError({ message }: { message: string }) {
  if (!message) {
    return null
  }
  return (
    <Alert variant="destructive">
      <AlertCircle />
      <AlertTitle>Check the form</AlertTitle>
      <AlertDescription>{message}</AlertDescription>
    </Alert>
  )
}

function WorkflowFooter({ onCancel, submitLabel }: { onCancel: () => void; submitLabel: string }) {
  return (
    <DialogFooter>
      <Button onClick={onCancel} type="button" variant="outline">
        Cancel
      </Button>
      <Button type="submit">{submitLabel}</Button>
    </DialogFooter>
  )
}

function AccountPanel({
  disabled,
  onMutate,
  session,
}: {
  disabled: boolean
  onMutate: (path: string, method: string, payload: unknown, success: string, reload?: boolean) => Promise<void>
  session: Session
}) {
  const [password, setPassword] = useState("")

  function submit(event: FormEvent) {
    event.preventDefault()
    void onMutate(
      "/api/account/password",
      "POST",
      { password },
      "Your password was changed.",
      false
    )
      .then(() => setPassword(""))
      .catch(() => undefined)
  }

  return (
    <Card>
      <CardHeader>
        <CardTitle>Account</CardTitle>
        <CardDescription>Self-service actions for the current bind DN.</CardDescription>
      </CardHeader>
      <CardContent>
        <form className="flex flex-col gap-4" onSubmit={submit}>
          <FieldGroup>
            <Field>
              <FieldLabel htmlFor="account-dn">DN</FieldLabel>
              <Input id="account-dn" value={session.userDN} disabled />
            </Field>
            <Field>
              <FieldLabel htmlFor="account-password">New password</FieldLabel>
              <Input
                id="account-password"
                autoComplete="new-password"
                disabled={disabled || !session.roles.passwordSelf}
                onChange={(event) => setPassword(event.target.value)}
                required
                type="password"
                value={password}
              />
              <FieldDescription>Only your own password can be changed here.</FieldDescription>
            </Field>
          </FieldGroup>
          <Button disabled={disabled || !session.roles.passwordSelf || password === ""} type="submit">
            Change password
          </Button>
        </form>
      </CardContent>
    </Card>
  )
}

function AdminPanel({
  baseDN,
  onMutate,
}: {
  baseDN: string
  onMutate: (path: string, method: string, payload: unknown, success: string, reload?: boolean) => Promise<void>
}) {
  const [workflow, setWorkflow] = useState<AdminWorkflow>()

  async function runAdminMutation(path: string, method: string, payload: unknown, success: string) {
    try {
      await onMutate(path, method, payload, success)
      setWorkflow(undefined)
    } catch {
      // Keep the dialog open so the global error notice remains actionable.
    }
  }

  return (
    <>
      <Card>
        <CardHeader>
          <CardTitle>Start an admin workflow</CardTitle>
          <CardDescription>Create entries here. Use search results and entry details to edit, reset, manage members, or delete.</CardDescription>
        </CardHeader>
        <CardContent>
          <AdminCreateActions onCreate={(entryType) => setWorkflow({ kind: "create", entryType })} />
        </CardContent>
      </Card>

      <AdminWorkflowDialog
        baseDN={baseDN}
        onOpenChange={(open) => {
          if (!open) {
            setWorkflow(undefined)
          }
        }}
        onSubmit={runAdminMutation}
        workflow={workflow}
      />
    </>
  )
}

function TextField({
  id,
  label,
  onChange,
  type = "text",
  value,
}: {
  id: string
  label: string
  onChange: (value: string) => void
  type?: string
  value: string
}) {
  return (
    <Field>
      <FieldLabel htmlFor={id}>{label}</FieldLabel>
      <Input id={id} onChange={(event) => onChange(event.target.value)} type={type} value={value} />
    </Field>
  )
}

function LinesField({
  id,
  label,
  onChange,
  value,
}: {
  id: string
  label: string
  onChange: (value: string) => void
  value: string
}) {
  return (
    <Field>
      <FieldLabel htmlFor={id}>{label}</FieldLabel>
      <Textarea id={id} onChange={(event) => onChange(event.target.value)} rows={4} value={value} />
      <FieldDescription>Enter one DN per line.</FieldDescription>
    </Field>
  )
}

function AttributesField({
  id,
  onChange,
  value,
}: {
  id: string
  onChange: (value: string) => void
  value: string
}) {
  return (
    <Field>
      <FieldLabel htmlFor={id}>Extra attributes</FieldLabel>
      <Textarea
        id={id}
        onChange={(event) => onChange(event.target.value)}
        placeholder="telephoneNumber: +1-555-0100&#10;description: Managed account"
        rows={3}
        value={value}
      />
      <FieldDescription>Use `name: value`, one attribute value per line. Protected attributes are rejected.</FieldDescription>
    </Field>
  )
}

async function loadConsole(): Promise<LoadState> {
  try {
    const session = await fetchJSON<Session>("/api/session")
    return { session, loading: false }
  } catch (error) {
    return {
      loading: false,
      error: error instanceof Error ? error.message : "Unable to load the directory console.",
    }
  }
}

async function fetchJSON<T>(path: string): Promise<T> {
  const response = await fetch(new URL(path, window.location.origin), {
    headers: {
      Accept: "application/json",
    },
  })
  if (!response.ok) {
    throw new Error(errorMessage(response.status))
  }
  return response.json() as Promise<T>
}

async function mutateJSON(path: string, method: string, payload: unknown) {
  const response = await fetch(new URL(path, window.location.origin), {
    body: payload === undefined ? undefined : JSON.stringify(payload),
    headers: {
      Accept: "application/json",
      "Content-Type": "application/json",
    },
    method,
  })
  if (!response.ok) {
    const body = await response.text()
    throw new Error(body.trim() || errorMessage(response.status))
  }
}

function errorMessage(status: number) {
  if (status === 401) {
    return "Sign in with LDAP credentials to open the console."
  }
  if (status === 403) {
    return "This account is not allowed to access that console surface."
  }
  if (status === 404) {
    return "That entry no longer exists or is outside the directory base."
  }
  return `Request failed with HTTP ${status}.`
}

function viewFromLocation(): ViewId {
  const params = new URLSearchParams(window.location.search)
  return normalizeViewId(params.get("view"))
}

function normalizeViewId(value: string | null): ViewId {
  switch (value) {
    case "directory":
    case "users":
    case "groups":
    case "ous":
    case "admin":
    case "account":
      return value
    default:
      return "directory"
  }
}

function navigateToView(
  view: ViewId,
  setActiveView: (view: ViewId) => void,
  replace = false
) {
  const params = new URLSearchParams(window.location.search)
  params.set("view", view)
  const url = `${window.location.pathname}?${params.toString()}${window.location.hash}`
  if (replace) {
    window.history.replaceState(null, "", url)
  } else {
    window.history.pushState(null, "", url)
  }
  setActiveView(view)
}

function normalizeActiveView(activeView: ViewId, navItems: NavItem[], session?: Session): ViewId {
  if (!session) {
    return activeView
  }
  if (navItems.some((item) => item.id === activeView)) {
    return activeView
  }
  return defaultView(session)
}

function defaultView(session: Session): ViewId {
  if (session.roles.directoryRead) {
    return "directory"
  }
  return "account"
}

function buildNavItems(session?: Session): NavItem[] {
  if (!session) {
    return [
      {
        id: "account",
        label: "Account",
        description: "Change your password.",
        icon: KeyRound,
      },
    ]
  }

  const items: NavItem[] = []
  if (session.roles.directoryRead) {
    items.push(
      {
        id: "directory",
        label: "Directory",
        description: "Browse all readable entries.",
        icon: Database,
      },
      {
        id: "users",
        label: "Users",
        description: "Browse user accounts.",
        icon: UserRound,
      },
      {
        id: "groups",
        label: "Groups",
        description: "Browse groups and members.",
        icon: Users,
      },
      {
        id: "ous",
        label: "OUs",
        description: "Browse organizational units.",
        icon: FolderTree,
      }
    )
  }
  if (session.roles.admin) {
    items.push({
      id: "admin",
      label: "Admin",
      description: "Create and maintain entries.",
      icon: Settings2,
    })
  }
  items.push({
    id: "account",
    label: "Account",
    description: "Change your password.",
    icon: KeyRound,
  })
  return items
}

function viewCopy(view: ViewId) {
  switch (view) {
    case "users":
      return {
        title: "Users",
        description: "Browse user accounts under the configured base DN.",
      }
    case "groups":
      return {
        title: "Groups",
        description: "Browse groups and inspect membership counts.",
      }
    case "ous":
      return {
        title: "Organizational units",
        description: "Browse the containers that shape the directory tree.",
      }
    case "admin":
      return {
        title: "Directory administration",
        description: "Create and maintain users, groups, organizational units, and passwords.",
      }
    case "account":
      return {
        title: "Account",
        description: "Change the password for the current bind DN.",
      }
    default:
      return {
        title: "Directory",
        description: "Browse users, groups, and organizational units.",
      }
  }
}

function accessBadgeLabel(session?: Session) {
  if (session?.roles.admin) {
    return "Admin"
  }
  if (session?.roles.directoryRead) {
    return "Read only"
  }
  return "Account only"
}

function ShellSkeleton() {
  return (
    <div className="grid gap-4 xl:grid-cols-[minmax(0,1fr)_20rem]">
      <Card>
        <CardHeader>
          <Skeleton className="h-5 w-48" />
          <Skeleton className="h-4 w-72 max-w-full" />
        </CardHeader>
        <CardContent className="flex flex-col gap-3">
          <Skeleton className="h-12" />
          <Skeleton className="h-12" />
          <Skeleton className="h-12" />
        </CardContent>
      </Card>
      <Card>
        <CardHeader>
          <Skeleton className="h-5 w-32" />
        </CardHeader>
        <CardContent className="flex flex-col gap-3">
          <Skeleton className="h-8" />
          <Skeleton className="h-8" />
        </CardContent>
      </Card>
    </div>
  )
}

function ResultSkeleton() {
  return (
    <div className="flex flex-col gap-3">
      <Skeleton className="h-12" />
      <Skeleton className="h-12" />
      <Skeleton className="h-12" />
      <Skeleton className="h-12" />
    </div>
  )
}

function resultRange(data: DirectorySearchResponse) {
  if (data.total === 0) {
    return "No results"
  }
  const start = (data.page - 1) * data.pageSize + 1
  const end = Math.min(data.page * data.pageSize, data.total)
  return `Showing ${start}-${end} of ${data.total}`
}

function visiblePages(page: number, totalPages: number) {
  const start = Math.max(1, page - 2)
  const end = Math.min(totalPages, start + 4)
  const adjustedStart = Math.max(1, end - 4)
  const pages: number[] = []
  for (let value = adjustedStart; value <= end; value += 1) {
    pages.push(value)
  }
  return pages
}

function directoryTypeLabel(type: DirectorySearchType) {
  switch (type) {
    case "users":
      return "Users"
    case "groups":
      return "Groups"
    case "ous":
      return "OUs"
    default:
      return "All entries"
  }
}

function workflowTypeLabel(type: WorkflowType) {
  switch (type) {
    case "user":
      return "user"
    case "group":
      return "group"
    case "ou":
      return "OU"
  }
}

function entryTypeLabel(type: DirectoryEntryType) {
  switch (type) {
    case "user":
      return "User"
    case "group":
      return "Group"
    case "ou":
      return "OU"
    default:
      return "Entry"
  }
}

function isAdminWorkflowEntry(type: DirectoryEntryType): type is WorkflowType {
  return type === "user" || type === "group" || type === "ou"
}

function workflowTitle(workflow: AdminWorkflow) {
  if (workflow.kind === "create") {
    return `Create ${workflowTypeLabel(workflow.entryType)}`
  }
  if (workflow.kind === "edit") {
    return `Edit ${entryTypeLabel(workflow.entry.type).toLowerCase()}`
  }
  if (workflow.kind === "reset") {
    return "Reset password"
  }
  return "Manage group members"
}

function workflowDescription(workflow: AdminWorkflow) {
  if (workflow.kind === "create") {
    return "Add a new entry below an existing parent DN."
  }
  if (workflow.kind === "edit") {
    return "Update editable attributes for this entry. Protected attributes are rejected by the server."
  }
  if (workflow.kind === "reset") {
    return "Set a new password for the selected user."
  }
  return "Add or remove member DNs for this group."
}

function workflowKey(workflow: AdminWorkflow) {
  if (workflow.kind === "create") {
    return `${workflow.kind}-${workflow.entryType}`
  }
  if (workflow.kind === "edit" || workflow.kind === "members") {
    return `${workflow.kind}-${workflow.entry.dn}`
  }
  return `${workflow.kind}-${workflow.entry.dn}`
}

function entrySummaryText(entry: EntrySummary) {
  if (entry.type === "user") {
    return entry.mail || entry.description || "No email set"
  }
  if (entry.type === "group") {
    const count = entry.members?.length ?? 0
    return `${count} member${count === 1 ? "" : "s"}`
  }
  if (entry.type === "ou") {
    return entry.description || "No description"
  }
  return entry.description || entry.objectClass
}

function requiredMessage(fields: Array<[string, string]>) {
  const missing = fields.find(([, value]) => value.trim() === "")
  return missing ? `${missing[0]} is required.` : ""
}

function validateMemberDNs(value: string) {
  const invalid = lines(value).find((dn) => !dn.includes("=") || !dn.includes(","))
  return invalid ? `Member DN is not valid: ${invalid}` : ""
}

function firstAttribute(entry: EntryDetail, name: string) {
  return entry.attributes[name.toLowerCase()]?.[0] ?? entry.attributes[name]?.[0] ?? ""
}

function attributesToText(attributes: Record<string, string[]>, excluded: string[]) {
  const excludedSet = new Set(excluded.map((name) => name.toLowerCase()))
  return Object.entries(attributes)
    .filter(([name]) => !excludedSet.has(name.toLowerCase()))
    .flatMap(([name, values]) => values.map((value) => `${name}: ${value}`))
    .join("\n")
}

function deletePath(entry: EntrySummary) {
  const encoded = encodeURIComponent(entry.dn)
  if (entry.type === "user") {
    return `/api/users?dn=${encoded}`
  }
  if (entry.type === "group") {
    return `/api/groups?dn=${encoded}`
  }
  if (entry.type === "ou") {
    return `/api/ous?dn=${encoded}`
  }
  return `/api/directory/entry?dn=${encoded}`
}

function parentDN(dn: string) {
  const [, parent = ""] = dn.split(/,(.*)/s)
  return parent
}

function dnLineage(dn: string) {
  const parts: string[] = []
  let current = dn
  while (current) {
    parts.push(current)
    current = parentDN(current)
  }
  return parts
}

function attributeRows(attributes: Record<string, string[]>) {
  return Object.entries(attributes)
    .filter(([name]) => name.toLowerCase() !== "userpassword")
    .flatMap(([name, values]) => values.map((value) => ({ name, value })))
    .sort((left, right) => {
      const nameCompare = left.name.localeCompare(right.name)
      if (nameCompare !== 0) {
        return nameCompare
      }
      return left.value.localeCompare(right.value)
    })
}

function formatTimestamp(value?: string) {
  if (!value) {
    return "Not recorded"
  }
  const date = new Date(value)
  if (Number.isNaN(date.getTime())) {
    return value
  }
  return date.toLocaleString()
}

async function copyText(value: string) {
  if (navigator.clipboard?.writeText) {
    await navigator.clipboard.writeText(value)
    return
  }

  const textarea = document.createElement("textarea")
  textarea.value = value
  textarea.setAttribute("readonly", "")
  textarea.style.position = "fixed"
  textarea.style.left = "-9999px"
  document.body.appendChild(textarea)
  textarea.select()
  const copied = document.execCommand("copy")
  document.body.removeChild(textarea)
  if (!copied) {
    throw new Error("copy failed")
  }
}

function lines(value: string) {
  return value
    .split("\n")
    .map((line) => line.trim())
    .filter(Boolean)
}

function parseAttributes(value: string) {
  const attributes: Record<string, string[]> = {}
  for (const line of lines(value)) {
    const separator = line.indexOf(":")
    if (separator === -1) {
      continue
    }
    const name = line.slice(0, separator).trim()
    const attrValue = line.slice(separator + 1).trim()
    if (name && attrValue) {
      attributes[name] = [...(attributes[name] ?? []), attrValue]
    }
  }
  return attributes
}
