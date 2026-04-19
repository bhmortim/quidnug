import { useQuidnug } from "../provider.js";

/**
 * Returns the currently-active Quid + a setter.
 *
 * Usage:
 *   const { quid, setQuid } = useQuid();
 *
 * The Quid is whatever the nearest <QuidnugProvider> was given via
 * initialQuid prop, or what the app has since set via setQuid().
 */
export function useQuid() {
    const { quid, setQuid } = useQuidnug();
    return { quid, setQuid };
}
