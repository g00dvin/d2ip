/** @type {import('tailwindcss').Config} */
export default {
  content: [
    './index.html',
    './src/**/*.{vue,js,ts,jsx,tsx}',
  ],
  darkMode: 'class',
  theme: {
    extend: {
      colors: {
        brand: {
          DEFAULT: '#5dade2',
          hover: '#3498db',
        },
        surface: {
          primary: '#0f1923',
          sidebar: '#1a2332',
          card: '#1a2332',
          code: '#0a1018',
        },
        border: {
          DEFAULT: '#2a3a4a',
          light: '#3a4a5a',
        },
        txt: {
          primary: '#b0bec5',
          secondary: '#6a7a8a',
          muted: '#4a5a6a',
        },
        ok: '#4caf80',
        warn: '#e0a050',
        err: '#e74c3c',
      },
      fontFamily: {
        mono: ['SF Mono', 'Cascadia Code', 'Fira Code', 'JetBrains Mono', 'Consolas', 'Courier New', 'monospace'],
      },
      fontSize: {
        '2xs': '0.625rem',
      },
    },
  },
  plugins: [],
}