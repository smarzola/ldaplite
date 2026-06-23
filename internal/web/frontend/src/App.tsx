import { type FormEvent, useEffect, useMemo, useState } from "react"
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

const capabilityLabels: Array<[keyof Session["roles"], string]> = [
  ["directoryRead", "Directory read"],
  ["directoryWrite", "Directory write"],
  ["admin", "Admin UI"],
  ["passwordSelf", "Own password"],
  ["passwordReset", "Password reset"],
]

export default function App() {
  const [state, setState] = useState<LoadState>({ loading: true })
  const [notice, setNotice] = useState<Notice>()
  const [mutating, setMutating] = useState(false)

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

  const session = state.session
  const directory = state.directory
  const navItems = useMemo(() => buildNavItems(session), [session])
  const accessLabel = accessBadgeLabel(session)

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

        {notice ? (
          <Alert variant={notice.kind === "error" ? "destructive" : "default"}>
            {notice.kind === "error" ? <AlertCircle /> : <ShieldCheck />}
            <AlertTitle>{notice.kind === "error" ? "Request failed" : "Saved"}</AlertTitle>
            <AlertDescription>{notice.text}</AlertDescription>
          </Alert>
        ) : null}

        <section className="grid gap-4 lg:grid-cols-[minmax(0,1fr)_19rem]">
          <Card>
            <CardHeader>
              <div className="flex flex-col gap-2 sm:flex-row sm:items-start sm:justify-between">
                <div className="flex flex-col gap-1">
                  <CardTitle>Directory workbench</CardTitle>
                  <CardDescription>
                    {directoryAccessDescription(session)}
                  </CardDescription>
                </div>
                <Badge variant={session?.roles.admin ? "default" : "secondary"}>
                  {accessLabel}
                </Badge>
              </div>
            </CardHeader>
            <CardContent>
              {state.loading ? <DirectorySkeleton /> : <DirectoryTables directory={directory} />}
            </CardContent>
          </Card>

          <div className="flex flex-col gap-4">
            <CapabilityRail loading={state.loading} session={session} />
            <SessionCard loading={state.loading} session={session} />
          </div>
        </section>

        {session && !state.loading ? (
          <section className="grid gap-4 lg:grid-cols-[20rem_minmax(0,1fr)]">
            <AccountPanel disabled={mutating} onMutate={runMutation} session={session} />
            {session.roles.admin ? (
              <AdminPanel
                baseDN={session.baseDN}
                directory={directory}
                disabled={mutating}
                onMutate={runMutation}
              />
            ) : null}
          </section>
        ) : null}
      </div>
    </main>
  )
}

function CapabilityRail({ loading, session }: { loading: boolean; session?: Session }) {
  return (
    <Card>
      <CardHeader>
        <CardTitle>Capability rail</CardTitle>
        <CardDescription>Permissions resolved by the server for this request.</CardDescription>
      </CardHeader>
      <CardContent>
        {loading ? (
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
  )
}

function SessionCard({ loading, session }: { loading: boolean; session?: Session }) {
  return (
    <Card>
      <CardHeader>
        <CardTitle>Session</CardTitle>
        <CardDescription>Authenticated LDAP actor.</CardDescription>
      </CardHeader>
      <CardContent className="flex flex-col gap-3">
        {loading ? (
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
        <CardTitle>Admin operations</CardTitle>
        <CardDescription>Create, update, delete, reset passwords, and edit group membership.</CardDescription>
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

function accessBadgeLabel(session?: Session) {
  if (session?.roles.admin) {
    return "Admin"
  }
  if (session?.roles.directoryRead) {
    return "Read only"
  }
  return "Account only"
}

function directoryAccessDescription(session?: Session) {
  if (session?.roles.directoryWrite) {
    return "Administrative actions are available for this session."
  }
  if (session?.roles.directoryRead) {
    return "Read-only directory access is active for this session."
  }
  return "Account-only access is active for this session."
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
