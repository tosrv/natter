import { createElement } from "react";
import { useReveal } from "@/hooks/useReveal";
import { cn } from "@/lib/utils";

/** Fades/slides its content in (<= 8px, 250ms) the first time it scrolls into view. */
export default function Reveal({ as = "div", className, children }: { as?: "div" | "h2"; className?: string; children: React.ReactNode }) {
  const ref = useReveal<HTMLElement>();
  return createElement(as, { ref, className: cn("reveal", className) }, children);
}
