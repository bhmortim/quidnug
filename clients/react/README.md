# @quidnug/react (scaffold)

Status: **SCAFFOLD — not yet on npm.**

React hooks and components layered on `@quidnug/client` (JS SDK v2),
giving idiomatic declarative access to the Quidnug trust graph.

## Planned API

```jsx
import { QuidnugProvider, useTrust, useStream, useQuid } from "@quidnug/react";

<QuidnugProvider node="http://localhost:8080">
  <App />
</QuidnugProvider>
```

Hooks:

| Hook | Returns |
| --- | --- |
| `useQuid()` | the active signer Quid, or `null` |
| `useTrust(observer, target, domain)` | `{ trustLevel, path, pending, error }` |
| `useStream(subjectId)` | `{ events, pagination, pending }` (SWR-cached) |
| `useGuardianSet(quidId)` | current guardian set |
| `useRegisterIdentity()` | callback; returns `{ mutate, pending, error }` |
| `useGrantTrust()` | same pattern |
| `useEmitEvent()` | same pattern |

Components:

- `<TrustBadge observer target domain />` — visual trust chip with color
  intensity mapped to trust level.
- `<TrustPath observer target domain />` — renders the best path with
  per-hop levels.
- `<GuardianSetCard quidId />` — guardian roster visualization.

## Roadmap

1. Extract `QuidnugProvider` + core hooks.
2. SWR / TanStack Query integration for cache + invalidation.
3. Storybook with all components.
4. Ship as `@quidnug/react@2.0.0` on npm.

## License

Apache-2.0.
