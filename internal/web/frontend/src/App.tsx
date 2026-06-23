import { type FormEvent, type MouseEvent, useEffect, useMemo, useState } from "react"
import {
  AlertCircle,
  Copy,
  Database,
  Eye,
  FolderTree,
  KeyRound,
  RefreshCw,
  Search,
  Settings2,
  ShieldCheck,
  UserRound,
  Users,
  type LucideIcon,
} from "lucide-react"

import { Alert, AlertDescription, AlertTitle } from "@/components/ui/alert"
import { Badge } from "@/components/ui/badge"
import { Button } from "@/components/ui/button"
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card"
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
  FieldLegend,
  FieldSeparator,
  FieldSet,
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
import { Skeleton } from "@/components/ui/skeleton"
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from "@/components/ui/table"
import { Tabs, TabsContent, TabsList, TabsTrigger } from "@/components/ui/tabs"
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

type Directory = {
  baseDN: string
  users: EntrySummary[]
  groups: EntrySummary[]
  ous: EntrySummary[]
}

type LoadState = {
  session?: Session
  directory?: Directory
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
  const directory = state.directory
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
      setNotice({
        kind: "error",
        text: error instanceof Error ? error.message : "The request failed.",
      })
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
              directory={directory}
              mutating={mutating}
              onNavigate={(view) => navigateToView(view, setActiveView)}
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
  directory,
  mutating,
  onNavigate,
  onMutate,
  onNotice,
  session,
}: {
  activeView: ViewId
  directory?: Directory
  mutating: boolean
  onNavigate: (view: ViewId) => void
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
        directory={directory}
        disabled={mutating}
        onMutate={onMutate}
      />
    )
  }

  if (activeView === "users") {
    return (
      <DirectorySearchView
        fixedType="users"
        onNavigate={onNavigate}
        onNotice={onNotice}
        session={session}
      />
    )
  }

  if (activeView === "groups") {
    return (
      <DirectorySearchView
        fixedType="groups"
        onNavigate={onNavigate}
        onNotice={onNotice}
        session={session}
      />
    )
  }

  if (activeView === "ous") {
    return (
      <DirectorySearchView
        fixedType="ous"
        onNavigate={onNavigate}
        onNotice={onNotice}
        session={session}
      />
    )
  }

  return (
    <DirectorySearchView onNavigate={onNavigate} onNotice={onNotice} session={session} />
  )
}

function DirectorySearchView({
  fixedType,
  onNavigate,
  onNotice,
  session,
}: {
  fixedType?: Exclude<DirectorySearchType, "all">
  onNavigate: (view: ViewId) => void
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

  const effectiveType = fixedType ?? entryType

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

  function submit(event: FormEvent) {
    event.preventDefault()
    setPage(1)
    setSubmittedQuery(query.trim())
  }

  async function copyDN(entry: EntrySummary) {
    try {
      await copyText(entry.dn)
      onNotice({ kind: "success", text: "DN copied." })
    } catch {
      onNotice({ kind: "error", text: "Could not copy the DN." })
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
          <form className="flex flex-col gap-3 lg:flex-row lg:items-end" onSubmit={submit}>
            <Field className="min-w-0 flex-1">
              <FieldLabel htmlFor="directory-search">Search directory</FieldLabel>
              <div className="flex gap-2">
                <Input
                  id="directory-search"
                  onChange={(event) => setQuery(event.target.value)}
                  placeholder="uid, cn, mail, DN, group, OU"
                  value={query}
                />
                <Button type="submit">
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
                onAdmin={() => onNavigate("admin")}
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

      {selectedEntry ? <SelectedEntryCard entry={selectedEntry} onCopyDN={copyDN} /> : null}
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
  onAdmin: () => void
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
  onAdmin: () => void
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
      {showAdminAction ? (
        <Button onClick={onAdmin} size="sm" type="button" variant="ghost">
          <Settings2 data-icon="inline-start" />
          Admin
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

function SelectedEntryCard({
  entry,
  onCopyDN,
}: {
  entry: EntrySummary
  onCopyDN: (entry: EntrySummary) => void
}) {
  return (
    <Card>
      <CardHeader>
        <div className="flex flex-col gap-2 sm:flex-row sm:items-start sm:justify-between">
          <div className="flex flex-col gap-1">
            <CardTitle>{entry.name || entry.dn}</CardTitle>
            <CardDescription>{entryTypeLabel(entry.type)} selected from search results.</CardDescription>
          </div>
          <Badge variant="secondary">{entry.objectClass}</Badge>
        </div>
      </CardHeader>
      <CardContent className="flex flex-col gap-3">
        <div className="flex flex-col gap-1">
          <p className="text-sm font-medium">DN</p>
          <p className="break-all font-mono text-xs leading-relaxed text-muted-foreground">{entry.dn}</p>
        </div>
        <Separator />
        <p className="text-sm text-muted-foreground">{entrySummaryText(entry)}</p>
        <div className="flex flex-wrap gap-2">
          <Button onClick={() => onCopyDN(entry)} size="sm" type="button" variant="outline">
            <Copy data-icon="inline-start" />
            Copy DN
          </Button>
        </div>
      </CardContent>
    </Card>
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
    ).then(() => setPassword(""))
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
  directory,
  disabled,
  onMutate,
}: {
  baseDN: string
  directory?: Directory
  disabled: boolean
  onMutate: (path: string, method: string, payload: unknown, success: string, reload?: boolean) => Promise<void>
}) {
  return (
    <Card>
      <CardHeader>
        <CardTitle>Directory administration</CardTitle>
        <CardDescription>Create and maintain users, groups, organizational units, and passwords.</CardDescription>
      </CardHeader>
      <CardContent>
        <Tabs defaultValue="users">
          <TabsList>
            <TabsTrigger value="users">Users</TabsTrigger>
            <TabsTrigger value="groups">Groups</TabsTrigger>
            <TabsTrigger value="ous">OUs</TabsTrigger>
          </TabsList>
          <TabsContent value="users" className="pt-4">
            <UserForms baseDN={baseDN} disabled={disabled} directory={directory} onMutate={onMutate} />
          </TabsContent>
          <TabsContent value="groups" className="pt-4">
            <GroupForms baseDN={baseDN} disabled={disabled} directory={directory} onMutate={onMutate} />
          </TabsContent>
          <TabsContent value="ous" className="pt-4">
            <OUForms baseDN={baseDN} disabled={disabled} directory={directory} onMutate={onMutate} />
          </TabsContent>
        </Tabs>
      </CardContent>
    </Card>
  )
}

function UserForms({
  baseDN,
  directory,
  disabled,
  onMutate,
}: {
  baseDN: string
  directory?: Directory
  disabled: boolean
  onMutate: (path: string, method: string, payload: unknown, success: string, reload?: boolean) => Promise<void>
}) {
  const userDN = directory?.users[0]?.dn ?? `uid=admin,ou=users,${baseDN}`
  const [create, setCreate] = useState({
    parentDN: `ou=users,${baseDN}`,
    uid: "",
    cn: "",
    sn: "",
    mail: "",
    password: "",
    attributes: "",
  })
  const [update, setUpdate] = useState({
    dn: userDN,
    cn: "",
    sn: "",
    givenName: "",
    mail: "",
    attributes: "",
  })
  const [reset, setReset] = useState({ dn: userDN, password: "" })
  const [deleteDN, setDeleteDN] = useState(userDN)

  return (
    <div className="flex flex-col gap-6">
      <form
        className="flex flex-col gap-4"
        onSubmit={(event) => {
          event.preventDefault()
          void onMutate(
            "/api/users",
            "POST",
            { ...create, attributes: parseAttributes(create.attributes) },
            "User created."
          )
        }}
      >
        <FieldSet>
          <FieldLegend>Create user</FieldLegend>
          <FieldGroup className="grid gap-4 md:grid-cols-2">
            <TextField id="create-user-parent" label="Parent DN" value={create.parentDN} onChange={(parentDN) => setCreate({ ...create, parentDN })} />
            <TextField id="create-user-uid" label="UID" value={create.uid} onChange={(uid) => setCreate({ ...create, uid })} />
            <TextField id="create-user-cn" label="Common name" value={create.cn} onChange={(cn) => setCreate({ ...create, cn })} />
            <TextField id="create-user-sn" label="Surname" value={create.sn} onChange={(sn) => setCreate({ ...create, sn })} />
            <TextField id="create-user-mail" label="Email" value={create.mail} onChange={(mail) => setCreate({ ...create, mail })} type="email" />
            <TextField id="create-user-password" label="Initial password" value={create.password} onChange={(password) => setCreate({ ...create, password })} type="password" />
          </FieldGroup>
          <AttributesField id="create-user-attributes" value={create.attributes} onChange={(attributes) => setCreate({ ...create, attributes })} />
        </FieldSet>
        <Button disabled={disabled} type="submit">Create user</Button>
      </form>

      <FieldSeparator>Update</FieldSeparator>

      <form
        className="flex flex-col gap-4"
        onSubmit={(event) => {
          event.preventDefault()
          void onMutate(
            `/api/users?dn=${encodeURIComponent(update.dn)}`,
            "PUT",
            { ...update, attributes: parseAttributes(update.attributes) },
            "User updated."
          )
        }}
      >
        <FieldSet>
          <FieldLegend>Edit user</FieldLegend>
          <FieldGroup className="grid gap-4 md:grid-cols-2">
            <TextField id="update-user-dn" label="Target DN" value={update.dn} onChange={(dn) => setUpdate({ ...update, dn })} />
            <TextField id="update-user-cn" label="Common name" value={update.cn} onChange={(cn) => setUpdate({ ...update, cn })} />
            <TextField id="update-user-sn" label="Surname" value={update.sn} onChange={(sn) => setUpdate({ ...update, sn })} />
            <TextField id="update-user-given" label="Given name" value={update.givenName} onChange={(givenName) => setUpdate({ ...update, givenName })} />
            <TextField id="update-user-mail" label="Email" value={update.mail} onChange={(mail) => setUpdate({ ...update, mail })} type="email" />
          </FieldGroup>
          <AttributesField id="update-user-attributes" value={update.attributes} onChange={(attributes) => setUpdate({ ...update, attributes })} />
        </FieldSet>
        <Button disabled={disabled} type="submit">Save user</Button>
      </form>

      <FieldSeparator>Password</FieldSeparator>

      <form
        className="grid gap-4 md:grid-cols-[1fr_1fr_auto]"
        onSubmit={(event) => {
          event.preventDefault()
          void onMutate("/api/users/password", "POST", reset, "Password reset.").then(() =>
            setReset({ ...reset, password: "" })
          )
        }}
      >
        <TextField id="reset-user-dn" label="User DN" value={reset.dn} onChange={(dn) => setReset({ ...reset, dn })} />
        <TextField id="reset-user-password" label="New password" value={reset.password} onChange={(password) => setReset({ ...reset, password })} type="password" />
        <Button className="self-end" disabled={disabled || reset.password === ""} type="submit">Reset</Button>
      </form>

      <DeleteForm disabled={disabled} dn={deleteDN} id="delete-user-dn" label="Delete user" onChange={setDeleteDN} onSubmit={() => onMutate(`/api/users?dn=${encodeURIComponent(deleteDN)}`, "DELETE", undefined, "User deleted.")} />
    </div>
  )
}

function GroupForms({
  baseDN,
  directory,
  disabled,
  onMutate,
}: {
  baseDN: string
  directory?: Directory
  disabled: boolean
  onMutate: (path: string, method: string, payload: unknown, success: string, reload?: boolean) => Promise<void>
}) {
  const memberDN = directory?.users[0]?.dn ?? `uid=admin,ou=users,${baseDN}`
  const groupDN = directory?.groups[0]?.dn ?? `cn=ldaplite.admin,ou=groups,${baseDN}`
  const [create, setCreate] = useState({
    parentDN: `ou=groups,${baseDN}`,
    cn: "",
    description: "",
    members: memberDN,
    attributes: "",
  })
  const [update, setUpdate] = useState({
    dn: groupDN,
    description: "",
    members: memberDN,
    attributes: "",
  })
  const [deleteDN, setDeleteDN] = useState(groupDN)

  return (
    <div className="flex flex-col gap-6">
      <form
        className="flex flex-col gap-4"
        onSubmit={(event) => {
          event.preventDefault()
          void onMutate(
            "/api/groups",
            "POST",
            { ...create, members: lines(create.members), attributes: parseAttributes(create.attributes) },
            "Group created."
          )
        }}
      >
        <FieldSet>
          <FieldLegend>Create group</FieldLegend>
          <FieldGroup className="grid gap-4 md:grid-cols-2">
            <TextField id="create-group-parent" label="Parent DN" value={create.parentDN} onChange={(parentDN) => setCreate({ ...create, parentDN })} />
            <TextField id="create-group-cn" label="CN" value={create.cn} onChange={(cn) => setCreate({ ...create, cn })} />
            <TextField id="create-group-description" label="Description" value={create.description} onChange={(description) => setCreate({ ...create, description })} />
          </FieldGroup>
          <LinesField id="create-group-members" label="Members" value={create.members} onChange={(members) => setCreate({ ...create, members })} />
          <AttributesField id="create-group-attributes" value={create.attributes} onChange={(attributes) => setCreate({ ...create, attributes })} />
        </FieldSet>
        <Button disabled={disabled} type="submit">Create group</Button>
      </form>

      <FieldSeparator>Update</FieldSeparator>

      <form
        className="flex flex-col gap-4"
        onSubmit={(event) => {
          event.preventDefault()
          void onMutate(
            `/api/groups?dn=${encodeURIComponent(update.dn)}`,
            "PUT",
            { ...update, members: lines(update.members), attributes: parseAttributes(update.attributes) },
            "Group updated."
          )
        }}
      >
        <FieldSet>
          <FieldLegend>Edit group membership</FieldLegend>
          <FieldGroup className="grid gap-4 md:grid-cols-2">
            <TextField id="update-group-dn" label="Group DN" value={update.dn} onChange={(dn) => setUpdate({ ...update, dn })} />
            <TextField id="update-group-description" label="Description" value={update.description} onChange={(description) => setUpdate({ ...update, description })} />
          </FieldGroup>
          <LinesField id="update-group-members" label="Members" value={update.members} onChange={(members) => setUpdate({ ...update, members })} />
          <AttributesField id="update-group-attributes" value={update.attributes} onChange={(attributes) => setUpdate({ ...update, attributes })} />
        </FieldSet>
        <Button disabled={disabled} type="submit">Save group</Button>
      </form>

      <DeleteForm disabled={disabled} dn={deleteDN} id="delete-group-dn" label="Delete group" onChange={setDeleteDN} onSubmit={() => onMutate(`/api/groups?dn=${encodeURIComponent(deleteDN)}`, "DELETE", undefined, "Group deleted.")} />
    </div>
  )
}

function OUForms({
  baseDN,
  directory,
  disabled,
  onMutate,
}: {
  baseDN: string
  directory?: Directory
  disabled: boolean
  onMutate: (path: string, method: string, payload: unknown, success: string, reload?: boolean) => Promise<void>
}) {
  const ouDN = directory?.ous[0]?.dn ?? `ou=users,${baseDN}`
  const [create, setCreate] = useState({ parentDN: baseDN, ou: "", description: "", attributes: "" })
  const [update, setUpdate] = useState({ dn: ouDN, description: "", attributes: "" })
  const [deleteDN, setDeleteDN] = useState(ouDN)

  return (
    <div className="flex flex-col gap-6">
      <form
        className="flex flex-col gap-4"
        onSubmit={(event) => {
          event.preventDefault()
          void onMutate(
            "/api/ous",
            "POST",
            { ...create, attributes: parseAttributes(create.attributes) },
            "OU created."
          )
        }}
      >
        <FieldSet>
          <FieldLegend>Create OU</FieldLegend>
          <FieldGroup className="grid gap-4 md:grid-cols-2">
            <TextField id="create-ou-parent" label="Parent DN" value={create.parentDN} onChange={(parentDN) => setCreate({ ...create, parentDN })} />
            <TextField id="create-ou-name" label="OU" value={create.ou} onChange={(ou) => setCreate({ ...create, ou })} />
            <TextField id="create-ou-description" label="Description" value={create.description} onChange={(description) => setCreate({ ...create, description })} />
          </FieldGroup>
          <AttributesField id="create-ou-attributes" value={create.attributes} onChange={(attributes) => setCreate({ ...create, attributes })} />
        </FieldSet>
        <Button disabled={disabled} type="submit">Create OU</Button>
      </form>

      <FieldSeparator>Update</FieldSeparator>

      <form
        className="flex flex-col gap-4"
        onSubmit={(event) => {
          event.preventDefault()
          void onMutate(
            `/api/ous?dn=${encodeURIComponent(update.dn)}`,
            "PUT",
            { ...update, attributes: parseAttributes(update.attributes) },
            "OU updated."
          )
        }}
      >
        <FieldSet>
          <FieldLegend>Edit OU</FieldLegend>
          <FieldGroup className="grid gap-4 md:grid-cols-2">
            <TextField id="update-ou-dn" label="OU DN" value={update.dn} onChange={(dn) => setUpdate({ ...update, dn })} />
            <TextField id="update-ou-description" label="Description" value={update.description} onChange={(description) => setUpdate({ ...update, description })} />
          </FieldGroup>
          <AttributesField id="update-ou-attributes" value={update.attributes} onChange={(attributes) => setUpdate({ ...update, attributes })} />
        </FieldSet>
        <Button disabled={disabled} type="submit">Save OU</Button>
      </form>

      <DeleteForm disabled={disabled} dn={deleteDN} id="delete-ou-dn" label="Delete OU" onChange={setDeleteDN} onSubmit={() => onMutate(`/api/ous?dn=${encodeURIComponent(deleteDN)}`, "DELETE", undefined, "OU deleted.")} />
    </div>
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

function DeleteForm({
  disabled,
  dn,
  id,
  label,
  onChange,
  onSubmit,
}: {
  disabled: boolean
  dn: string
  id: string
  label: string
  onChange: (value: string) => void
  onSubmit: () => Promise<void>
}) {
  return (
    <form
      className="flex flex-col gap-4"
      onSubmit={(event) => {
        event.preventDefault()
        void onSubmit()
      }}
    >
      <FieldSet>
        <FieldLegend>{label}</FieldLegend>
        <TextField id={id} label="DN" value={dn} onChange={onChange} />
        <FieldDescription>Deletion is immediate and may fail when child entries still exist.</FieldDescription>
      </FieldSet>
      <Button disabled={disabled || dn === ""} type="submit" variant="destructive">
        {label}
      </Button>
    </form>
  )
}

async function loadConsole(): Promise<LoadState> {
  try {
    const session = await fetchJSON<Session>("/api/session")
    const directory = session.roles.directoryRead
      ? await fetchJSON<Directory>("/api/directory")
      : undefined
    return { session, directory, loading: false }
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
