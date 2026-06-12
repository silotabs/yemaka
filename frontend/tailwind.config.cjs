/** @type {import('tailwindcss').Config} */
module.exports = {
  content: ['./index.html', './src/**/*.{svelte,ts}'],
  theme: {
    extend: {
      colors: {
        ink: 'rgb(var(--color-ink) / <alpha-value>)',
        field: 'rgb(var(--color-field) / <alpha-value>)',
        line: 'rgb(var(--color-line) / <alpha-value>)',
        pine: 'rgb(var(--color-pine) / <alpha-value>)',
        ember: 'rgb(var(--color-ember) / <alpha-value>)',
        hover: 'rgb(var(--color-hover) / <alpha-value>)',
        panel: 'rgb(var(--color-panel) / <alpha-value>)',
        muted: 'rgb(var(--color-muted) / <alpha-value>)'
      },
      fontFamily: {
        sans: ['Inter', 'ui-sans-serif', 'system-ui', '-apple-system', 'BlinkMacSystemFont', 'Segoe UI', 'sans-serif']
      }
    }
  },
  plugins: []
};
