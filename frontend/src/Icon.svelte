<script lang="ts">
  export let name = 'chat';
  export let size = 18;
</script>

<svg
  width={size}
  height={size}
  class=""
  viewBox="0 0 24 24"
  fill="none"
  stroke="currentColor"
  stroke-width="1"
  stroke-linecap="round"
  stroke-linejoin="round"
  aria-hidden="true"
>
  {#if name === 'spark'}
    <path d="M12 2v20" />
    <path d="M2 12h20" />
    <path d="m4.9 4.9 14.2 14.2" />
    <path d="m19.1 4.9-14.2 14.2" />
  {:else if name === 'chat'}
    <path d="M21 12a8 8 0 0 1-8 8H7l-4 2 1.6-4.8A8 8 0 1 1 21 12Z" />
  {:else if name === 'models'}
    <path d="M8 3h8l4 4v10l-4 4H8l-4-4V7l4-4Z" />
    <path d="M9 9h6" />
    <path d="M9 13h6" />
  {:else if name === 'memory'}
    <ellipse cx="12" cy="5" rx="7" ry="3" />
    <path d="M5 5v6c0 1.7 3.1 3 7 3s7-1.3 7-3V5" />
    <path d="M5 11v6c0 1.7 3.1 3 7 3s7-1.3 7-3v-6" />
  {:else if name === 'documents'}
    <path d="M7 3h7l4 4v14H7z" />
    <path d="M14 3v5h5" />
    <path d="M9 13h6" />
    <path d="M9 17h5" />
  {:else if name === 'skills'}
    <path d="M12 3 3 8l9 5 9-5-9-5Z" />
    <path d="m3 13 9 5 9-5" />
  {:else if name === 'extensions'}
    <path d="M8 4h8v5h4v8h-5v3H7v-3H4V9h4z" />
  {:else if name === 'automation'}
    <path d="M12 8v4l3 2" />
    <circle cx="12" cy="12" r="8" />
    <path d="M4 4v5h5" />
  {:else if name === 'learning'}
    <path d="M4 19.5A2.5 2.5 0 0 1 6.5 17H20" />
    <path d="M6.5 3H20v18H6.5A2.5 2.5 0 0 1 4 18.5v-13A2.5 2.5 0 0 1 6.5 3Z" />
    <path d="M9 7h7" />
    <path d="M9 11h5" />
  {:else if name === 'tools'}
    <path d="m14.7 6.3 3 3" />
    <path d="M4 20 16.5 7.5a2.1 2.1 0 0 1 3 3L7 23H4z" />
  {:else if name === 'settings'}
    <circle cx="12" cy="12" r="3" />
    <path d="M19.4 15a1.7 1.7 0 0 0 .3 1.9l.1.1-2 3-.2-.1a1.8 1.8 0 0 0-2-.2 1.7 1.7 0 0 0-1 1.5V21h-5v-.2a1.7 1.7 0 0 0-1-1.5 1.8 1.8 0 0 0-2 .2l-.2.1-2-3 .1-.1A1.7 1.7 0 0 0 4.6 15a1.7 1.7 0 0 0-1.5-1H3v-4h.1a1.7 1.7 0 0 0 1.5-1 1.7 1.7 0 0 0-.3-1.9L4.2 7l2-3 .2.1a1.8 1.8 0 0 0 2 .2 1.7 1.7 0 0 0 1-1.5V3h5v.2a1.7 1.7 0 0 0 1 1.5 1.8 1.8 0 0 0 2-.2l.2-.1 2 3-.1.1a1.7 1.7 0 0 0-.3 1.9 1.7 1.7 0 0 0 1.5 1h.3v4h-.3a1.7 1.7 0 0 0-1.3.6Z" />
  {:else if name === 'send'}
    <path d="m22 2-7 20-4-9-9-4z" />
    <path d="M22 2 11 13" />
  {:else if name === 'stop'}
    <rect x="7" y="7" width="10" height="10" rx="1.5" />
  {:else if name === 'plus'}
    <path d="M12 5v14" />
    <path d="M5 12h14" />
  {:else if name === 'search'}
    <circle cx="11" cy="11" r="7" />
    <path d="m20 20-3.5-3.5" />
  {:else if name === 'write'}
    <path d="M12 20h9" />
    <path d="M16.5 3.5a2.1 2.1 0 0 1 3 3L8 18l-4 1 1-4Z" />
  {:else if name === 'code'}
    <path d="m8 9-4 3 4 3" />
    <path d="m16 9 4 3-4 3" />
    <path d="m14 5-4 14" />
  {:else if name === 'health'}
    <path d="M20 7h-4l-2 10-4-14-2 10H4" />
  {:else if name === 'more'}
    <circle cx="5" cy="12" r="1.2" />
    <circle cx="12" cy="12" r="1.2" />
    <circle cx="19" cy="12" r="1.2" />
  {:else if name === 'sidebar'}
    <rect x="3" y="4" width="18" height="16" rx="2" />
    <path d="M9 4v16" />
  {:else if name === 'chevron'}
    <path d="m9 18 6-6-6-6" />
  {:else if name === 'eye'}
    <path d="M3 12s3.4-6.5 9-6.5 9 6.5 9 6.5-3.4 6.5-9 6.5S3 12 3 12Z" />
    <circle cx="12" cy="12" r="2.8" />
  {:else if name === 'eye-off'}
    <path d="M4.1 9.4C5.7 7.4 8.4 5.5 12 5.5c5.6 0 9 6.5 9 6.5a16 16 0 0 1-3 4" />
    <path d="M14.2 17.9a8.8 8.8 0 0 1-2.2.3C6.4 18.2 3 12 3 12a16 16 0 0 1 2.4-3.2" />
    <path d="M9.9 9.9a2.8 2.8 0 0 1 4.2 3.7" />
    <path d="M3.5 3.5 20.5 20.5" />
  {:else if name === 'star'}
    <path d="m12 3 2.7 5.5 6.1.9-4.4 4.3 1 6.1-5.4-2.9-5.4 2.9 1-6.1-4.4-4.3 6.1-.9z" />
  {:else if name === 'copy'}
    <rect x="8" y="8" width="11" height="11" rx="2" />
    <path d="M5 15H4a2 2 0 0 1-2-2V5a2 2 0 0 1 2-2h8a2 2 0 0 1 2 2v1" />
  {:else if name === 'close'}
    <path d="m7.5 7.5 9 9" />
    <path d="m16.5 7.5-9 9" />
  {:else if name === 'check'}
    <path d="m5 12 4 4L19 6" />
  {:else if name === 'retry'}
    <path d="M4 12a8 8 0 0 1 13.6-5.7L20 8" />
    <path d="M20 4v4h-4" />
    <path d="M20 12a8 8 0 0 1-13.6 5.7L4 16" />
    <path d="M4 20v-4h4" />
  {:else if name === 'trash'}
    <path d="M4 7h16" />
    <path d="M10 11v6" />
    <path d="M14 11v6" />
    <path d="M6 7l1 14h10l1-14" />
    <path d="M9 7V4h6v3" />
  {:else if name === 'user'}
    <path d="M20 21v-2a4 4 0 0 0-3-3.87" />
    <path d="M4 21v-2a4 4 0 0 1 3-3.87" />
    <circle cx="12" cy="7" r="4" />
  {:else if name === 'assistant'}
    <path d="M12 2a10 10 0 1 0 10 10A10 10 0 0 0 12 2Z" />
    <path d="M8 14s1.5-2 4-2 4 2 4 2" />
    <line x1="9" y1="9" x2="9.01" y2="9" />
    <line x1="15" y1="9" x2="15.01" y2="9" />
  {:else if name === 'system'}
    <path d="M12 2a10 10 0 1 0 10 10A10 10 0 0 0 12 2Z" />
    <path d="M8 14s1.5-2 4-2 4 2 4 2" />
    <line x1="9" y1="9" x2="9.01" y2="9" />
    <line x1="15" y1="9" x2="15.01" y2="9" />
  {:else if name === 'previous'}
    <path d="M15 19 8 12l7-7" />
  {:else if name === 'next'}
    <path d="M9 5 15.5 12l-7 7" />
  {:else if name === 'success'}
    <circle cx="12" cy="12" r="8.5" />
    <path d="m8.7 12.2 2.2 2.2 4.6-4.8" />
  {:else if name === 'error'}
    <circle cx="12" cy="12" r="8.5" />
    <path d="m9.2 9.2 5.6 5.6" />
    <path d="m14.8 9.2-5.6 5.6" />
  {:else if name === 'warning'}
    <path d="M12 3.7 21 19.2H3L12 3.7Z" />
    <path d="M12 9.3v4.4" />
    <path d="M12 17.1h.01" />
  {:else if name === 'info'}
    <circle cx="12" cy="12" r="8.5" />
    <path d="M12 11.2v5" />
    <path d="M12 7.8h.01" />
  {:else if name === 'arrow-right'}
    <path d="M5 12h14" />
    <path d="m15 6 6 6-6 6" />
  {:else if name === 'arrow-left'}
    <path d="M19 12H5" />
    <path d="m9 6-6 6 6 6" />
  {:else if name === 'question'}
    <path d="M9.1 9a3 3 0 1 1 5.8 1c0 2-3 3-3 3" />
    <path d="M12 17h.01" />
  {:else}
    <circle cx="12" cy="12" r="8" />
  {/if}
</svg>
