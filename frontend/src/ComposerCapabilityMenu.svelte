<script lang="ts">
  import { DropdownMenu, ScrollArea } from 'bits-ui';
  import Icon from './Icon.svelte';
  import type { Tab } from './lib/appOptions';
  import Badge from './Badge.svelte';

  type Skill = {
    name: string;
  };

  type ContextItem = {
    label: string;
    icon: string;
    tone: string;
  };

  type SettingsView = {
    shellEnabled?: boolean;
  };

  type Status = {
    shellEnabled?: boolean;
  };

  export let composerMenuOpen = false;
  export let selectedSkill = '';
  export let composerRouteOpen = false;
  export let settingLowMemory = false;
  export let settingRAGEnabled = false;
  export let settingInternetEnabled = false;
  export let settings: SettingsView | null = null;
  export let status: Status | null = null;
  export let enabledSkills: Skill[] = [];
  export let chatContextItems: ContextItem[] = [];
  export let skillDisplayLabel: (skill: string) => string = (skill) => skill || 'Auto skill';
  export let toggleComposerSetting: (kind: 'lowMemory' | 'rag' | 'internet') => Promise<void> | void = () => {};
  export let setActiveTab: (tab: Tab) => Promise<void> | void = () => {};

  $: activeContextCount = chatContextItems.filter((item) => item.tone === 'ok' || item.tone === 'warn').length;
  $: primarySkills = enabledSkills.slice(0, 5);
  $: moreSkills = enabledSkills.slice(5);
</script>

<DropdownMenu.Root bind:open={composerMenuOpen}>
  <DropdownMenu.Trigger class="icon-button composer-plus-button" title="Yemaka controls">
    <Icon name="plus" size={20} />
  </DropdownMenu.Trigger>
  <DropdownMenu.Portal>
    <DropdownMenu.Content class="composer-menu bits-menu-content" sideOffset={10} align="start">
      <div class="composer-menu-label">Skill</div>
      <DropdownMenu.Item class={`composer-menu-item ${!selectedSkill ? 'is-selected' : ''}`} onSelect={() => (selectedSkill = '')}>
        <span class="composer-menu-check">{!selectedSkill ? '✓' : ''}</span>
        <Icon name="spark" size={15} />
        <span>Auto skill</span>
      </DropdownMenu.Item>
      {#each primarySkills as skill}
        <DropdownMenu.Item class={`composer-menu-item ${selectedSkill === skill.name ? 'is-selected' : ''}`} onSelect={() => (selectedSkill = skill.name)}>
          <span class="composer-menu-check">{selectedSkill === skill.name ? '✓' : ''}</span>
          <Icon name="skills" size={15} />
          <span title={skill.name}>{skillDisplayLabel(skill.name)}</span>
        </DropdownMenu.Item>
      {/each}
      {#if moreSkills.length}
        <div class="composer-menu-label composer-menu-label-subtle">More skills</div>
        <ScrollArea.Root class="composer-skill-scroll" type="auto">
          <ScrollArea.Viewport class="composer-skill-scroll-viewport">
            {#each moreSkills as skill}
              <DropdownMenu.Item class={`composer-menu-item ${selectedSkill === skill.name ? 'is-selected' : ''}`} onSelect={() => (selectedSkill = skill.name)}>
                <span class="composer-menu-check">{selectedSkill === skill.name ? '✓' : ''}</span>
                <Icon name="skills" size={15} />
                <span title={skill.name}>{skillDisplayLabel(skill.name)}</span>
              </DropdownMenu.Item>
            {/each}
          </ScrollArea.Viewport>
          <ScrollArea.Scrollbar class="composer-skill-scrollbar" orientation="vertical">
            <ScrollArea.Thumb class="composer-skill-thumb" />
          </ScrollArea.Scrollbar>
        </ScrollArea.Root>
      {/if}

      <DropdownMenu.Separator class="chat-menu-divider" />
      <div class="composer-route-group">
        <button
          class="composer-route-trigger"
          type="button"
          aria-expanded={composerRouteOpen}
          onclick={(event) => {
            event.stopPropagation();
            composerRouteOpen = !composerRouteOpen;
          }}
        >
          <span class="composer-route-title">
            <Icon name="automation" size={15} />
            Current route
          </span>
          <span class="composer-route-summary">{activeContextCount} active</span>
          <Icon name="chevron" size={14} />
        </button>
        {#if composerRouteOpen}
          <div class="composer-status-grid mt-2">
            {#each chatContextItems as item}
              <Badge variant={item.label === 'internet on' ? 'warning' : item.label === 'standard' ? 'neutral' : 'success'} title={item.label} icon={item.icon} className="composer-route-item" size="xs">
                <span class="truncate">{item.label}</span>
              </Badge>
            {/each}
          </div>
        {/if}
      </div>

      <DropdownMenu.Separator class="chat-menu-divider" />
      <div class="composer-menu-label">Yemaka tuning</div>
      <DropdownMenu.Item
        class="composer-menu-item composer-menu-toggle"
        onSelect={(event) => {
          event.preventDefault();
          void toggleComposerSetting('lowMemory');
        }}
      >
        <span class:toggle-dot-on={settingLowMemory} class="toggle-dot"></span>
        <Icon name="settings" size={15} />
        <span>Low memory</span>
        <small>{settingLowMemory ? 'on' : 'off'}</small>
      </DropdownMenu.Item>
      <DropdownMenu.Item
        class="composer-menu-item composer-menu-toggle"
        onSelect={(event) => {
          event.preventDefault();
          void toggleComposerSetting('rag');
        }}
      >
        <span class:toggle-dot-on={settingRAGEnabled} class="toggle-dot"></span>
        <Icon name="documents" size={15} />
        <span>RAG</span>
        <small>{settingRAGEnabled ? 'on' : 'off'}</small>
      </DropdownMenu.Item>
      <DropdownMenu.Item
        class="composer-menu-item composer-menu-toggle"
        onSelect={(event) => {
          event.preventDefault();
          void toggleComposerSetting('internet');
        }}
      >
        <span class:toggle-dot-on={settingInternetEnabled} class="toggle-dot"></span>
        <Icon name="search" size={15} />
        <span>Internet</span>
        <small>{settingInternetEnabled ? 'on' : 'off'}</small>
      </DropdownMenu.Item>
      <div class="composer-menu-item composer-menu-toggle composer-menu-readonly">
        <span class:toggle-dot-on={settings?.shellEnabled ?? status?.shellEnabled} class="toggle-dot"></span>
        <Icon name="tools" size={15} />
        <span>Tools</span>
        <small>{settings?.shellEnabled ?? status?.shellEnabled ? 'safe allowlist' : 'off'}</small>
      </div>

      <DropdownMenu.Separator class="chat-menu-divider" />
      <DropdownMenu.Item class="composer-menu-item" onSelect={() => setActiveTab('settings')}>
        <span class="composer-menu-check"></span>
        <Icon name="settings" size={15} />
        <span>Open settings</span>
      </DropdownMenu.Item>
    </DropdownMenu.Content>
  </DropdownMenu.Portal>
</DropdownMenu.Root>
