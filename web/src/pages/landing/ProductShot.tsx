import { cn } from "@/lib/utils";

const SHOTS = {
  desktop: { w: 1440, h: 900, alt: "natter chat on desktop: a conversation with Alya showing read receipts and a typing indicator" },
  mobile: { w: 390, h: 844, alt: "natter chat on a phone: a conversation with Alya showing read receipts and a typing indicator" },
  receipts: { w: 996, h: 480, alt: "Close-up of a natter conversation: sent and read ticks, and Alya typing" },
} as const;

export type ProductShotVariant = keyof typeof SHOTS;

export default function ProductShot({ variant, priority, className }: { variant: ProductShotVariant; priority?: boolean; className?: string }) {
  const { w, h, alt } = SHOTS[variant];
  const pic = (theme: "light" | "dark") => {
    const b = `/product/${variant}-${theme}`;
    const set = (ext: string) => `${b}.${ext} 1x, ${b}@2x.${ext} 2x`;
    return (
      <picture key={theme} className={theme === "light" ? "block dark:hidden" : "hidden dark:block"}>
        <source type="image/avif" srcSet={set("avif")} />
        <source type="image/webp" srcSet={set("webp")} />
        <img src={`${b}.png`} srcSet={set("png")} width={w} height={h} alt={alt} decoding="async"
          loading={priority ? "eager" : "lazy"} fetchPriority={priority ? "high" : undefined}
          className="h-auto w-full" />
      </picture>
    );
  };
  return (
    <div className={cn("overflow-hidden rounded-2xl border bg-surface shadow-2xl shadow-black/[0.03]", className)}>
      {pic("light")}{pic("dark")}
    </div>
  );
}
