/** @type {import('tailwindcss').Config} */
module.exports = {
  darkMode: 'class',
  content: [
    './internal/views/**/*.html',
  ],
  safelist: [
    { pattern: /^grid-cols-[1-6]$/, variants: ['md'] },
    'text-green-500',
    'text-red-400',
  ],
  theme: {
    extend: {},
  },
  plugins: [],
}
