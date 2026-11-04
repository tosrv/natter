/** @type {import('tailwindcss').Config} */
import animate from "tailwindcss-animate";
export default {
  darkMode: ["class"],
  content: ["./index.html", "./src/**/*.{ts,tsx}"],
  theme: { extend: { fontFamily: { sans: ["Geist", "system-ui", "sans-serif"] }, colors: {
    border: "hsl(var(--border))", input: "hsl(var(--input))", ring: "hsl(var(--ring))",
    background: "hsl(var(--background))", foreground: "hsl(var(--foreground))",
    primary: { DEFAULT: "hsl(var(--primary))", solid: "hsl(var(--primary-solid))", text: "hsl(var(--primary-text))", foreground: "hsl(var(--primary-foreground))" },
    secondary: { DEFAULT: "hsl(var(--secondary))", foreground: "hsl(var(--secondary-foreground))" },
    destructive: { DEFAULT: "hsl(var(--destructive))", foreground: "hsl(var(--destructive-foreground))" },
    muted: { DEFAULT: "hsl(var(--muted))", foreground: "hsl(var(--muted-foreground))" },
    accent: { DEFAULT: "hsl(var(--accent))", foreground: "hsl(var(--accent-foreground))" },
    popover: { DEFAULT: "hsl(var(--popover))", foreground: "hsl(var(--popover-foreground))" },
    surface: "hsl(var(--surface))",
    card: { DEFAULT: "hsl(var(--card))", foreground: "hsl(var(--card-foreground))" },
  }, borderRadius: { lg: "0.5rem", md: "calc(0.5rem - 2px)", sm: "calc(0.5rem - 4px)" } } },
  plugins: [animate],
};
