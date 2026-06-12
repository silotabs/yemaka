<script lang="ts">
  export let size: number | string = 32;
  export let strokeWidth = 1.75;
  export let title = 'Yemaka';
  export let decorative = false;
</script>

<svg
  width={size}
  height={size}
  viewBox="0 0 24 24"
  fill="none"
  stroke="currentColor"
  stroke-width={strokeWidth}
  stroke-linecap="butt"
  stroke-linejoin="miter"
  xmlns="http://www.w3.org/2000/svg"
  role={decorative ? 'presentation' : 'img'}
  aria-hidden={decorative}
  aria-label={decorative ? undefined : title}
>
  {#if !decorative}
    <title>{title}</title>
  {/if}

  <path d="M3.55 8.45L5.05 2.1L7.85 8.8L10.05 3.7L12.25 9.35L15.05 2.1L17.35 8.8L20.05 5.15L20.65 8.55" />

  <path
    d="M5.75 10.6L10.95 11.25L8.15 15.55ZM7.65 12.15L9.05 12.35L8.3 13.55Z"
    fill="currentColor"
    fill-rule="evenodd"
    stroke="none"
  />
  <path
    d="M18.25 10.6L13.05 11.25L15.85 15.55ZM16.35 12.15L14.95 12.35L15.7 13.55Z"
    fill="currentColor"
    fill-rule="evenodd"
    stroke="none"
  />

  <path
    d="M11.35 15.45L9.35 19L15.05 18.35ZM11.65 16.85L10.9 18.1L12.95 17.9Z"
    fill="currentColor"
    fill-rule="evenodd"
    stroke="none"
  />

  <path d="M7.75 21.05L9.1 19.45L10.45 21.05L11.85 19.45L13.25 21.05L14.65 19.45L16.05 21.05" stroke-width={strokeWidth * 0.82} />
</svg>
