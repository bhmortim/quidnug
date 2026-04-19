/**
 * @quidnug/react — React hooks + components for the Quidnug protocol.
 *
 * Provides a `<QuidnugProvider>` that exposes a `QuidnugClient` +
 * active `Quid` via React context, and hooks for the most common
 * read and write flows.
 *
 * Hooks are framework-neutral where possible: no TanStack Query /
 * SWR dependency. Results are memoized per-key and refetched on
 * dependency change. For heavier caching, wrap hooks in your own
 * TanStack Query `useQuery(... queryFn: () => quidnug.getTrust(...))`.
 */

export { QuidnugProvider, useQuidnug } from "./provider.js";
export { useQuid } from "./hooks/useQuid.js";
export { useTrust } from "./hooks/useTrust.js";
export { useStream } from "./hooks/useStream.js";
export { useIdentity } from "./hooks/useIdentity.js";
export { useGuardianSet } from "./hooks/useGuardianSet.js";
export { useRegisterIdentity } from "./mutations/useRegisterIdentity.js";
export { useGrantTrust } from "./mutations/useGrantTrust.js";
export { useEmitEvent } from "./mutations/useEmitEvent.js";
export { TrustBadge } from "./components/TrustBadge.js";
export { TrustPath } from "./components/TrustPath.js";
export { GuardianSetCard } from "./components/GuardianSetCard.js";
