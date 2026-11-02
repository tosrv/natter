import * as React from "react";
import * as S from "@radix-ui/react-scroll-area";
import { cn } from "@/lib/utils";

const ScrollArea = React.forwardRef<React.ElementRef<typeof S.Root>, React.ComponentPropsWithoutRef<typeof S.Root>>(({ className, children, ...p }, ref) => (
  <S.Root ref={ref} className={cn("relative overflow-hidden", className)} {...p}>
    <S.Viewport className="h-full w-full rounded-[inherit]">{children}</S.Viewport>
    <ScrollBar />
    <S.Corner />
  </S.Root>
));
ScrollArea.displayName = "ScrollArea";

const ScrollBar = React.forwardRef<React.ElementRef<typeof S.ScrollAreaScrollbar>, React.ComponentPropsWithoutRef<typeof S.ScrollAreaScrollbar>>(({ className, orientation = "vertical", ...p }, ref) => (
  <S.ScrollAreaScrollbar ref={ref} orientation={orientation} className={cn("flex touch-none select-none transition-colors", orientation === "vertical" && "h-full w-2.5 border-l border-l-transparent p-[1px]", orientation === "horizontal" && "h-2.5 flex-col border-t border-t-transparent p-[1px]", className)} {...p}>
    <S.ScrollAreaThumb className="relative flex-1 rounded-full bg-border" />
  </S.ScrollAreaScrollbar>
));
ScrollBar.displayName = "ScrollBar";
export { ScrollArea, ScrollBar };
