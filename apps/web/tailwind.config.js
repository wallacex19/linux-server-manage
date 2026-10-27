/** @type {import('tailwindcss').Config} */
export default {
  content: [
    "./index.html",
    "./src/**/*.{js,ts,jsx,tsx}",
  ],
  theme: {
    extend: {
      colors: {
        border: "hsl(var(--border, 214.3 31.8% 91.4%))",
        background: "hsl(var(--background, 222.2 84% 4.9%))",
        foreground: "hsl(var(--foreground, 210 40% 98%))",
        primary: {
          DEFAULT: "hsl(var(--primary, 217.2 91.2% 59.8%))",
          foreground: "hsl(var(--primary-foreground, 222.2 47.4% 11.2%))",
        },
      },
    },
  },
  plugins: [],
}
