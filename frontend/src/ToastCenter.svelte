<script lang="ts">
  import Icon from './Icon.svelte';
  import { toastIcon, type ToastItem } from './lib/toastCenter';

  export let toasts: ToastItem[] = [];
  export let dismissToast: (id: number) => void = () => {};
</script>

{#if toasts.length}
  <section class="toast-center" aria-label="Yemaka notifications" aria-live="polite">
    {#each toasts as toast (toast.id)}
      <article class={`toast-card toast-${toast.kind}`} role={toast.kind === 'error' ? 'alert' : 'status'}>
        <div class="toast-icon">
          <Icon name={toastIcon(toast.kind)} size={20} />
        </div>
        <div class="toast-copy">
          <div class="toast-title">{toast.title}</div>
          <div class="toast-message">{toast.message}</div>
        </div>
        <button class="toast-dismiss" type="button" aria-label="Dismiss notification" onclick={() => dismissToast(toast.id)}>
          <Icon name="close" size={14} />
        </button>
      </article>
    {/each}
  </section>
{/if}
