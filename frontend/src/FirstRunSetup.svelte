<script lang="ts">
  import BitSelect from './BitSelect.svelte';
  import ActionButton from './ActionButton.svelte';

  type ModelInfo = {
    name: string;
    size: number;
  };

  type SetupState = {
    setupComplete: boolean;
    ollamaOk: boolean;
    ollamaError: string;
    configPath: string;
    profilePath: string;
    installedModels: ModelInfo[];
  };

  export let setupState: SetupState;
  export let setupMode = 'student_laptop';
  export let setupModeDirty = false;
  export let setupModel = '';
  export let setupBusy = false;
  export let refresh: () => Promise<void> | void = () => {};
  export let completeSetup: () => Promise<void> | void = () => {};
</script>

<div class="fixed inset-0 z-20 grid place-items-center bg-slate-950/30 px-4">
  <section class="bg-white w-full max-w-3xl rounded-xl border border-line">
    <div class="border-b border-line px-5 py-4">
      <div class="text-base font-semibold">First Run Setup For Yemaka Environment</div>
      <div class="mt-1 text-sm text-slate-600">Local mode, installed models only, no downloads.</div>
    </div>
    <div class="grid gap-4 p-5 md:grid-cols-[1fr_1fr]">
      <button
        class={`rounded-md border p-4 text-left ${setupMode === 'student_laptop' ? 'border-pine bg-emerald-50' : 'border-line bg-white'}`}
        type="button"
        aria-pressed={setupMode === 'student_laptop'}
        onclick={() => {
          setupMode = 'student_laptop';
          setupModeDirty = true;
        }}
      >
        <div class="mb-2 text-sm font-semibold">Student Laptop</div>
        <div class="text-sm text-slate-700">4 GB target, conservative context, one tool at a time.</div>
      </button>
      <button
        class={`rounded-md border p-4 text-left ${setupMode === 'useful_local' ? 'border-pine bg-emerald-50' : 'border-line bg-white'}`}
        type="button"
        aria-pressed={setupMode === 'useful_local'}
        onclick={() => {
          setupMode = 'useful_local';
          setupModeDirty = true;
        }}
      >
        <div class="mb-2 text-sm font-semibold">Useful Local Agent</div>
        <div class="text-sm text-slate-700">8+ GB target, still local-first, slightly larger context.</div>
      </button>
    </div>
    <div class="border-t border-line p-5">
      <div class="mb-2 flex items-center gap-2 text-sm">
        <span class={`h-2.5 w-2.5 rounded-full ${setupState.ollamaOk ? 'bg-pine' : 'bg-ember'}`}></span>
        <span>{setupState.ollamaOk ? 'Ollama detected' : 'Ollama not reachable'}</span>
      </div>
      {#if setupState.ollamaError}
        <div class="mb-3 rounded-md border border-amber-200 bg-amber-50 px-3 py-2 text-sm text-amber-900">{setupState.ollamaError}</div>
      {/if}
      <label class="mb-2 block text-sm font-medium" for="setup-model">Low-memory model</label>
      <BitSelect
        className="mb-4"
        bind:value={setupModel}
        placeholder={(setupState.installedModels ?? []).length === 0 ? 'No installed models detected' : 'Choose installed model'}
        disabled={(setupState.installedModels ?? []).length === 0}
        options={(setupState.installedModels ?? []).map((model) => ({
          value: model.name,
          label: model.name,
          meta: `${Math.round(model.size / 1024 / 1024)} MB`
        }))}
      />
      <div class="mb-4 grid gap-2 text-sm md:grid-cols-2">
        <div class="rounded-md border border-line px-3 py-2 overflow-auto">Config: {setupState.configPath}</div>
        <div class="rounded-md border border-line px-3 py-2 overflow-auto">Profile: {setupState.profilePath}</div>
      </div>
      <div class="flex justify-end gap-2">
        <ActionButton variant="secondary" icon="retry" onclick={refresh}>Recheck</ActionButton>
        <ActionButton
          variant="primary"
          icon="check"
          disabled={setupBusy || ((setupState.installedModels ?? []).length > 0 && !setupModel)}
          disabledReason={setupBusy ? 'Setup is already saving.' : 'Choose an installed model before saving setup.'}
          busy={setupBusy}
          busyLabel="Saving"
          onclick={completeSetup}
        >
          Save Setup
        </ActionButton>
      </div>
    </div>
  </section>
</div>
