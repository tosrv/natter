/** @type {import('tailwindcss').Config} */
import animate from "tailwindcss-animate";
export default {
  darkMode: ["class"],
  content: ["./index.html", "./src/**/*.{ts,tsx}"],
  theme: { extend: { colors: {
    border: "hsl(var(--border))", input: "hsl(var(--border))", ring: "hsl(var(--ring))",
    background: "hsl(var(--background))", foreground: "hsl(var(--foreground))",
    primary: { DEFAULT: "hsl(var(--primary))", foreground: "hsl(var(--primary-foreground))" },
    secondary: { DEFAULT: "hsl(var(--secondary))", foreground: "hsl(var(--secondary-foreground))" },
    destructive: { DEFAULT: "hsl(var(--destructive))", foreground: "hsl(var(--primary-foreground))" },
    muted: { DEFAULT: "hsl(var(--secondary))", foreground: "hsl(var(--muted-foreground))" },
    accent: { DEFAULT: "hsl(var(--secondary))", foreground: "hsl(var(--secondary-foreground))" },
    popover: { DEFAULT: "hsl(var(--background))", foreground: "hsl(var(--foreground))" },
  }, borderRadius: { lg: "0.5rem", md: "calc(0.5rem - 2px)", sm: "calc(0.5rem - 4px)" } } },
  plugins: [animate],
};
