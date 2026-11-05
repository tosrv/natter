import { cn } from "@/lib/utils";
import { useIsDark } from "@/hooks/useTheme";

type Shot = { w: number; h: number };
const SHOTS = {
  desktop: { w: 1440, h: 760 },
  mobile: { w: 390, h: 844 },
  receipts: { w: 390, h: 560 },
} as const satisfies Record<string, Shot>;

const ALT = {
  desktop: "natter chat on desktop: a conversation with Alya showing read receipts and a typing indicator",
  mobile: "natter chat on a phone: a conversation with Alya showing read receipts and a typing indicator",
  receipts: "Close-up of a natter conversation: sent and read ticks, and Alya typing",
} as const;

/** "hero" art-directs desktop (≥768px) and mobile (<768px) captures in one <picture>, so only one downloads. */
export type ProductShotVariant = keyof typeof SHOTS | "hero";

const DESKTOP_MQ = "(min-width: 768px)";
const MOBILE_MQ = "(max-width: 767px)";

export default function ProductShot({ variant, priority, className }: { variant: ProductShotVariant; priority?: boolean; className?: string }) {
  const dark = useIsDark();
  const pic = (theme: "light" | "dark") => {
    // The theme not currently shown is lazy so it does not download until the user switches.
    const active = theme === (dark ? "dark" : "light");
    const eager = priority && active;
    const set = (v: keyof typeof SHOTS, ext: string) => `/product/${v}-${theme}.${ext} 1x, /product/${v}-${theme}@2x.${ext} 2x`;
    const sources = (v: keyof typeof SHOTS, media?: string) => (["avif", "webp"] as const).map((ext) => (
      <source key={`${v}-${ext}`} type={`image/${ext}`} media={media} srcSet={set(v, ext)} width={SHOTS[v].w} height={SHOTS[v].h} />
    ));
    const base: keyof typeof SHOTS = variant === "hero" ? "mobile" : variant;
    return (
      <picture key={theme} className={theme === "light" ? "block dark:hidden" : "hidden dark:block"}>
        {variant === "hero" ? [...sources("desktop", DESKTOP_MQ), ...sources("mobile", MOBILE_MQ)] : sources(base)}
        <img src={`/product/${base}-${theme}.png`} srcSet={set(base, "png")} width={SHOTS[base].w} height={SHOTS[base].h}
          alt={variant === "hero" ? ALT.desktop : ALT[base]} decoding="async"
          loading={eager ? "eager" : "lazy"} fetchPriority={eager ? "high" : undefined} className="h-auto w-full" />
      </picture>
    );
  };
  return (
    <div className={cn("overflow-hidden rounded-2xl border bg-surface shadow-2xl shadow-black/[0.03]", className)}>
      {pic("light")}{pic("dark")}
    </div>
  );
}
