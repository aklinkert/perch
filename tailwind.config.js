/** @type {import('tailwindcss').Config} */
module.exports = {
  darkMode: 'class',
  content: [
    './internal/views/**/*.html',
  ],
  safelist: [
    { pattern: /^columns-[1-6]$/, variants: ['md'] },
    'break-inside-avoid',
  ],
  theme: {
    extend: {},
  },
  plugins: [],
}
