# @quidnug/react

React hooks and components for Quidnug — declarative per-observer
trust in your UI. Layered on top of [`@quidnug/client`](../js/)
(JS SDK v2).

## Install

```bash
npm install @quidnug/react @quidnug/client react
```

Requires React 18+. No `react-dom` runtime dependency in the package
itself — it's a peerDependency.

## Thirty-second example

```jsx
import { QuidnugProvider, useTrust, TrustBadge } from "@quidnug/react";

function App() {
    return (
        <QuidnugProvider node="http://localhost:8080" defaultDomain="company.home">
            <Header />
        </QuidnugProvider>
    );
}

function Header() {
    // declarative trust query with automatic loading state
    const { data, loading } = useTrust("my-quid", "them-quid", "company.home");
    return loading ? "…" : <span>trust {data.trustLevel.toFixed(2)}</span>;
}
```

## What's in the box

### `<QuidnugProvider>`

Wraps the tree in a React context carrying the SDK client and the
active user Quid:

```jsx
<QuidnugProvider
    node="https://api.example.com"
    initialQuid={myQuid}              // Quid object from @quidnug/client
    defaultDomain="company.home"
    maxRetries={3}
    retryBaseDelayMs={1000}
>
    <App />
</QuidnugProvider>
```

### Hooks

| Hook | What it returns |
| --- | --- |
| `useQuid()` | `{ quid, setQuid }` — active signer Quid. |
| `useQuidnug()` | Full context: `{ client, quid, setQuid, defaultDomain }`. |
| `useTrust(observer, target, domain, { maxDepth })` | `{ data: { trustLevel, trustPath, pathDepth }, error, loading, refetch }` |
| `useIdentity(quidId, { domain })` | Identity record or `null` (on 404). |
| `useStream(subjectId, { limit, offset, domain })` | Paginated event-stream slice. |
| `useGuardianSet(quidId)` | Guardian set (QDP-0002) or `null`. |
| `useRegisterIdentity()` | `{ mutate, data, error, pending }` |
| `useGrantTrust()` | `{ mutate, data, error, pending }` |
| `useEmitEvent()` | `{ mutate, data, error, pending }` |

### Components

#### `<TrustBadge>`

```jsx
<TrustBadge
    observer="alice-id"
    target="bob-id"
    domain="company.home"
    threshold={0.7}           // optional: colors green/red by cutoff
/>
```

Renders a colored chip with the computed trust level. Falls back
to "?" on error, "…" while loading.

#### `<TrustPath>`

```jsx
<TrustPath observer="alice-id" target="bob-id" domain="company.home" />
```

Renders the best path as `alice → carol → bob`. Shows "no path" when
the target is unreachable.

#### `<GuardianSetCard>`

```jsx
<GuardianSetCard quidId="alice-id" />
```

Visualization of guardian threshold, recovery delay, and guardian
roster.

## Examples

Runnable examples in [`examples/`](examples/):

| File | Shows |
| --- | --- |
| `TrustDashboard.jsx` | Team-wide trust matrix, audit log, guardian card. |
| `TrustGatedAction.jsx` | Button gated on relational-trust threshold. |

## Caching + refetch

The built-in hooks memoize by their key dependencies and refetch on
change. There's no SWR or TanStack Query under the hood — to add
those, wrap the primitive SDK calls in your own `useQuery`:

```jsx
import { useQuery } from "@tanstack/react-query";
import { useQuidnug } from "@quidnug/react";

function MyPanel() {
    const { client } = useQuidnug();
    const { data } = useQuery({
        queryKey: ["trust", alice, bob, "company.home"],
        queryFn: () => client.getTrustLevel(alice, bob, "company.home"),
    });
    return <div>{data?.trustLevel}</div>;
}
```

This pattern gives you TanStack's cache invalidation, retry config,
stale-while-revalidate, devtools, etc.

## Mutations — write flows

```jsx
function SignupForm() {
    const { mutate, pending, error } = useRegisterIdentity();

    return (
        <form onSubmit={async (e) => {
            e.preventDefault();
            await mutate({ name: "Alice", homeDomain: "company.home" });
        }}>
            {error && <p style={{ color: "red" }}>{String(error)}</p>}
            <button disabled={pending}>Register</button>
        </form>
    );
}
```

## Tests

```bash
cd clients/react
npm test
```

Smoke tests validate the module surface and the export graph. Full
React rendering tests live in consuming apps — use your framework's
preferred test renderer (React Testing Library, Vitest, etc.).

## Server-side rendering

Every hook is safe for SSR: they start in a "no data / not loading"
state until the client mounts. No `window` accesses in hook setup
paths. Avoid calling mutations from SSR — they require a real client
runtime.

## TypeScript

`.d.ts` files shipped alongside the `.js` modules with
`"types": "./src/index.d.ts"` in package.json (planned for next
release). For now, annotate at the call site using the upstream
`@quidnug/client` types.

## Protocol version compatibility

| SDK | Node | QDPs |
| --- | --- | --- |
| 2.x | 2.x | 0001–0010 |

## License

Apache-2.0.
