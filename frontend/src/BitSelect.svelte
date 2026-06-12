<script lang="ts">
  import { DropdownMenu } from 'bits-ui';
  import Icon from './Icon.svelte';

  type BitSelectOption = {
    value: string;
    label: string;
    disabled?: boolean;
    meta?: string;
  };

  export let value = '';
  export let options: BitSelectOption[] = [];
  export let placeholder = 'Select';
  export let disabled = false;
  export let className = '';
  export let contentClass = '';
  export let onChange: (value: string) => void = () => {};

  let open = false;

  $: selected = options.find((option) => option.value === value);

  function choose(option: BitSelectOption) {
    if (disabled || option.disabled) return;
    value = option.value;
    onChange(option.value);
    open = false;
  }
</script>

<DropdownMenu.Root bind:open>
  <DropdownMenu.Trigger class={`bit-select-trigger ${className}`} disabled={disabled} title={selected?.label || placeholder}>
    <span class="bit-select-label">{selected?.label || placeholder}</span>
    <Icon name="chevron" size={14} />
  </DropdownMenu.Trigger>
  <DropdownMenu.Portal>
    <DropdownMenu.Content class={`bit-select-content bits-menu-content ${contentClass}`} sideOffset={6} align="start">
      {#each options as option}
        <DropdownMenu.Item
          class={`bit-select-item ${option.value === value ? 'is-selected' : ''}`}
          disabled={option.disabled}
          onSelect={() => choose(option)}
        >
          <span class="bit-select-check">{option.value === value ? '✓' : ''}</span>
          <span class="bit-select-copy">
            <span>{option.label}</span>
            {#if option.meta}
              <small>{option.meta}</small>
            {/if}
          </span>
        </DropdownMenu.Item>
      {/each}
    </DropdownMenu.Content>
  </DropdownMenu.Portal>
</DropdownMenu.Root>
