import { inject, provide, readonly, ref } from "vue";

const QuidnugSymbol = Symbol.for("@quidnug/vue.context");

/**
 * provideQuidnug({ client, initialQuid?, defaultDomain? })
 *
 * Vue 3 equivalent of React's <QuidnugProvider>. Call inside a
 * top-level component's setup() to install the context. Children
 * read it via useQuidnug().
 *
 * The context shape is intentionally identical to the React one
 * — { client, quid, setQuid, defaultDomain } — so the composables
 * below can mirror the React hooks line-for-line.
 *
 *   import { provideQuidnug } from "@quidnug/vue-reviews";
 *   import QuidnugClient from "@quidnug/client";
 *   import "@quidnug/client/v2";
 *
 *   const client = new QuidnugClient({ defaultNode: "http://localhost:8080" });
 *   provideQuidnug({ client, defaultDomain: "reviews.public.other" });
 */
export function provideQuidnug({ client, initialQuid = null, defaultDomain = "default" }) {
    if (!client) throw new Error("provideQuidnug: client is required");
    const quid = ref(initialQuid);
    const ctx = {
        client,
        quid: readonly(quid),
        setQuid: (q) => { quid.value = q; },
        defaultDomain,
    };
    provide(QuidnugSymbol, ctx);
    return ctx;
}

/** Composable returning `{ client, quid, setQuid, defaultDomain }`. */
export function useQuidnug() {
    const ctx = inject(QuidnugSymbol, null);
    if (ctx === null) {
        throw new Error("useQuidnug must be called inside provideQuidnug()");
    }
    return ctx;
}
