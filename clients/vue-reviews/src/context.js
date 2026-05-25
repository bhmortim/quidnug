/**
 * Quidnug Vue context — provide/inject equivalent of the React
 * `<QuidnugProvider>` / `useQuidnug()` pair.
 *
 * Apps that want to use the composables and components from this
 * package must call `provideQuidnug({ client, quid })` somewhere
 * higher in the tree (typically in the root component's setup, or
 * via a small Vue plugin).
 *
 *   // App.vue
 *   <script setup>
 *   import { ref } from "vue";
 *   import QuidnugClient from "@quidnug/client";
 *   import { provideQuidnug } from "@quidnug/vue-reviews";
 *
 *   const client = new QuidnugClient({ defaultNode: "https://node.example" });
 *   const quid = ref(null); // or a loaded Quid
 *   provideQuidnug({ client, quid });
 *   </script>
 *
 * `client` may be a raw QuidnugClient or a Vue ref to one.
 * `quid` may be a raw Quid (or null) or a Vue ref. The composables
 * unwrap both forms.
 */
import { inject, provide, ref, isRef } from "vue";

export const QuidnugKey = Symbol("quidnug");

/**
 * Provide a Quidnug client + observer quid to descendants. Both
 * values may be plain or reactive refs.
 */
export function provideQuidnug({ client, quid = null, defaultDomain = "default" } = {}) {
    const clientRef = isRef(client) ? client : ref(client);
    const quidRef = isRef(quid) ? quid : ref(quid);
    const domainRef = isRef(defaultDomain) ? defaultDomain : ref(defaultDomain);
    const ctx = { client: clientRef, quid: quidRef, defaultDomain: domainRef };
    provide(QuidnugKey, ctx);
    return ctx;
}

/**
 * Composable returning `{ client, quid, defaultDomain }` as refs.
 * Throws if no provider is mounted above.
 */
export function useQuidnug() {
    const ctx = inject(QuidnugKey, null);
    if (ctx === null) {
        throw new Error("useQuidnug must be called inside a component tree where provideQuidnug() ran");
    }
    return ctx;
}
