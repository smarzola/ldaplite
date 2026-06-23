import { useEffect, useMemo, useState } from "react"
import { AlertCircle, Database, KeyRound, ShieldCheck } from "lucide-react"

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

const capabilityLabels: Array<[keyof Session["roles"], string]> = [
  ["directoryRead", "Directory read"],
  ["directoryWrite", "Directory write"],
  ["admin", "Admin UI"],
  ["passwordSelf", "Own password"],
  ["passwordReset", "Password reset"],
]

export default function App() {
  const [state, setState] = useState<LoadState>({ loading: true })

  useEffect(() => {
    let cancelled = false

    async function load() {
      try {
        const session = await fetchJSON<Session>("/api/session")
        const directory = session.roles.directoryRead
          ? await fetchJSON<Directory>("/api/directory")
          : undefined

        if (!cancelled) {
          setState({ session, directory, loading: false })
        }
      } catch (error) {
        if (!cancelled) {
          setState({
            loading: false,
            error: error instanceof Error ? error.message : "Unable to load the directory console.",
          })
        }
      }
    }

    void load()
    return () => {
      cancelled = true
    }
  }, [])

  const session = state.session
  const directory = state.directory
  const navItems = useMemo(() => buildNavItems(session), [session])

  return (
    <main className="min-h-svh bg-background text-foreground">
      <div className="mx-auto flex min-h-svh w-full max-w-6xl flex-col gap-6 px-4 py-6 sm:px-6 lg:px-8">
        <header className="flex flex-col gap-5 border-b pb-5 lg:flex-row lg:items-end lg:justify-between">
          <div className="flex flex-col gap-2">
            <p className="font-mono text-xs uppercase tracking-normal text-muted-foreground">
              {session?.baseDN ?? "Loading directory"}
            </p>
            <div className="flex flex-col gap-1">
              <h1 className="text-2xl font-semibold tracking-normal">LDAPLite directory console</h1>
              <p className="max-w-2xl text-sm text-muted-foreground">
                Role-aware operations for users, groups, OUs, and account access.
              </p>
            </div>
          </div>

          <div className="flex flex-wrap items-center gap-2">
            {navItems.map((item) => (
              <Button key={item} variant={item === "Admin" ? "default" : "outline"} size="sm">
                {item === "Account" ? <KeyRound data-icon="inline-start" /> : null}
                {item}
              </Button>
            ))}
          </div>
        </header>

        {state.error ? (
          <Alert variant="destructive">
            <AlertCircle />
            <AlertTitle>Console unavailable</AlertTitle>
            <AlertDescription>{state.error}</AlertDescription>
          </Alert>
        ) : null}

        <section className="grid gap-4 lg:grid-cols-[minmax(0,1fr)_19rem]">
          <Card>
            <CardHeader>
              <div className="flex flex-col gap-2 sm:flex-row sm:items-start sm:justify-between">
                <div className="flex flex-col gap-1">
                  <CardTitle>Directory workbench</CardTitle>
                  <CardDescription>
                    {session?.roles.directoryWrite
                      ? "Administrative actions are available for this session."
                      : "Read-only directory access is active for this session."}
                  </CardDescription>
                </div>
                <Badge variant={session?.roles.admin ? "default" : "secondary"}>
                  {session?.roles.admin ? "Admin" : "Read only"}
                </Badge>
              </div>
            </CardHeader>
            <CardContent>
              {state.loading ? <DirectorySkeleton /> : <DirectoryTables directory={directory} />}
            </CardContent>
          </Card>

          <div className="flex flex-col gap-4">
            <Card>
              <CardHeader>
                <CardTitle>Capability rail</CardTitle>
                <CardDescription>Permissions resolved by the server for this request.</CardDescription>
              </CardHeader>
              <CardContent>
                {state.loading ? (
                  <div className="flex flex-col gap-2">
                    {capabilityLabels.map(([, label]) => (
                      <Skeleton key={label} className="h-6" />
                    ))}
                  </div>
                ) : (
                  <div className="flex flex-col gap-2">
                    {capabilityLabels.map(([key, label]) => (
                      <div key={key} className="flex items-center justify-between gap-3">
                        <span className="text-sm">{label}</span>
                        <Badge variant={session?.roles[key] ? "default" : "secondary"}>
                          {session?.roles[key] ? "Allowed" : "Denied"}
                        </Badge>
                      </div>
                    ))}
                  </div>
                )}
              </CardContent>
            </Card>

            <Card>
              <CardHeader>
                <CardTitle>Session</CardTitle>
                <CardDescription>Authenticated LDAP actor.</CardDescription>
              </CardHeader>
              <CardContent className="flex flex-col gap-3">
                {state.loading ? (
                  <>
                    <Skeleton className="h-5" />
                    <Skeleton className="h-5" />
                  </>
                ) : (
                  <>
                    <div className="flex items-center gap-2">
                      <ShieldCheck />
                      <span className="text-sm font-medium">{session?.userID}</span>
                    </div>
                    <p className="break-all font-mono text-xs text-muted-foreground">{session?.userDN}</p>
                    <Separator />
                    <p className="text-sm text-muted-foreground">
                      {session?.roles.passwordSelf
                        ? "Self-service password changes are available."
                        : "Password self-service is not available for this actor."}
                    </p>
                  </>
                )}
              </CardContent>
            </Card>
          </div>
        </section>
      </div>
    </main>
  )
}

async function fetchJSON<T>(path: string): Promise<T> {
  const response = await fetch(new URL(path, window.location.origin), {
    headers: {
      Accept: "application/json",
    },
  })
  if (!response.ok) {
    if (response.status === 401) {
      throw new Error("Sign in with LDAP credentials to open the console.")
    }
    if (response.status === 403) {
      throw new Error("This account is not allowed to access that console surface.")
    }
    throw new Error(`Request failed with HTTP ${response.status}.`)
  }
  return response.json() as Promise<T>
}

function buildNavItems(session?: Session) {
  if (!session) {
    return ["Account"]
  }

  const items = ["Account"]
  if (session.roles.directoryRead) {
    items.unshift("Directory")
  }
  if (session.roles.admin) {
    items.push("Admin")
  }
  return items
}

function DirectorySkeleton() {
  return (
    <div className="flex flex-col gap-4">
      <Skeleton className="h-24" />
      <Skeleton className="h-24" />
      <Skeleton className="h-24" />
    </div>
  )
}

function DirectoryTables({ directory }: { directory?: Directory }) {
  if (!directory) {
    return (
      <Empty>
        <EmptyHeader>
          <EmptyMedia variant="icon">
            <Database />
          </EmptyMedia>
          <EmptyTitle>No directory access</EmptyTitle>
          <EmptyDescription>This session can only access account-level functionality.</EmptyDescription>
        </EmptyHeader>
      </Empty>
    )
  }

  return (
    <div className="flex flex-col gap-6">
      <EntryTable title="Users" entries={directory.users} detail="mail" />
      <EntryTable title="Groups" entries={directory.groups} detail="members" />
      <EntryTable title="Organizational units" entries={directory.ous} detail="description" />
    </div>
  )
}

function EntryTable({
  title,
  entries,
  detail,
}: {
  title: string
  entries: EntrySummary[]
  detail: "mail" | "members" | "description"
}) {
  return (
    <div className="flex flex-col gap-3">
      <div className="flex items-center justify-between gap-3">
        <h2 className="text-sm font-medium">{title}</h2>
        <Badge variant="secondary">{entries.length}</Badge>
      </div>
      {entries.length === 0 ? (
        <Empty>
          <EmptyHeader>
            <EmptyMedia variant="icon">
              <Database />
            </EmptyMedia>
            <EmptyTitle>No {title.toLowerCase()} found</EmptyTitle>
            <EmptyDescription>Create entries through an admin-capable session.</EmptyDescription>
          </EmptyHeader>
        </Empty>
      ) : (
        <Table>
          <TableHeader>
            <TableRow>
              <TableHead>Name</TableHead>
              <TableHead>DN</TableHead>
              <TableHead className="hidden sm:table-cell">Detail</TableHead>
            </TableRow>
          </TableHeader>
          <TableBody>
            {entries.map((entry) => (
              <TableRow key={entry.dn}>
                <TableCell className="font-medium">{entry.name}</TableCell>
                <TableCell className="max-w-40 truncate font-mono text-xs text-muted-foreground sm:max-w-72">
                  {entry.dn}
                </TableCell>
                <TableCell className="hidden sm:table-cell">{entryDetail(entry, detail)}</TableCell>
              </TableRow>
            ))}
          </TableBody>
        </Table>
      )}
    </div>
  )
}

function entryDetail(entry: EntrySummary, detail: "mail" | "members" | "description") {
  if (detail === "members") {
    return entry.members?.length ? `${entry.members.length} member${entry.members.length === 1 ? "" : "s"}` : "No members"
  }
  return entry[detail] || "Not set"
}
