<!--
 * <QuidnugWriteReview product topic @success />
 *
 * Inline review-writing form. Requires a signed-in Quid via
 * `provideQuidnug({ client, quid })`.
-->
<script setup>
import { ref, computed } from "vue";
import { useQuidnug } from "../context.js";
import { useWriteReview } from "../composables/useWriteReview.js";

const props = defineProps({
    product: { type: String, required: true },
    topic: { type: String, required: true },
});
const emit = defineEmits(["success"]);

const { quid } = useQuidnug();
const { mutate, loading, error } = useWriteReview();

const rating = ref(5);
const title = ref("");
const body = ref("");

const canPost = computed(() => !!quid.value?.has_private_key);

async function submit(e) {
    e?.preventDefault?.();
    try {
        await mutate({
            product: props.product,
            topic: props.topic,
            rating: rating.value,
            title: title.value,
            body: body.value,
        });
        title.value = "";
        body.value = "";
        rating.value = 5;
        emit("success");
    } catch { /* shown via error state */ }
}
</script>

<template>
    <p v-if="!canPost"
       :style="{ color: '#666', fontStyle: 'italic', marginTop: '16px' }">
        Sign in with your Quidnug identity to write a review.
    </p>
    <form v-else
          @submit="submit"
          :style="{
              marginTop: '16px', padding: '12px', border: '1px solid #ddd',
              borderRadius: '8px', background: '#fafafa',
          }">
        <h4 :style="{ margin: '0 0 8px', fontSize: '14px' }">Write a review</h4>

        <div :style="{ marginBottom: '8px', display: 'flex', gap: '8px', alignItems: 'center' }">
            <label :style="{ fontSize: '13px' }">Your rating:</label>
            <input type="number" :min="0" :max="5" :step="0.5"
                   v-model.number="rating"
                   :style="{ width: '60px', padding: '4px' }" />
        </div>

        <input type="text" placeholder="Review title"
               v-model="title"
               :style="{ width: '100%', padding: '8px', marginBottom: '8px',
                         border: '1px solid #ccc', borderRadius: '4px',
                         boxSizing: 'border-box' }" />

        <textarea placeholder="Your experience..."
                  v-model="body"
                  :rows="5"
                  :style="{ width: '100%', padding: '8px',
                            border: '1px solid #ccc', borderRadius: '4px',
                            boxSizing: 'border-box',
                            fontFamily: 'inherit', fontSize: '13px' }"></textarea>

        <div v-if="error" :style="{ color: '#c0392b', fontSize: '12px', margin: '8px 0' }">
            {{ String(error.message ?? error) }}
        </div>

        <button type="submit" :disabled="loading"
                :style="{ marginTop: '8px', padding: '8px 16px', background: '#2E8B57',
                          color: '#fff', border: 'none', borderRadius: '4px',
                          cursor: loading ? 'not-allowed' : 'pointer',
                          opacity: loading ? 0.6 : 1 }">
            {{ loading ? "Submitting…" : "Post review" }}
        </button>

        <p :style="{ color: '#777', fontSize: '11px', marginTop: '8px' }">
            Signed with your Quidnug identity. Published to <code>reviews.public.*</code> —
            your reputation carries across every site that speaks QRP-0001.
        </p>
    </form>
</template>
