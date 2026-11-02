import * as React from "react";
import * as M from "@radix-ui/react-dropdown-menu";
import { cn } from "@/lib/utils";

const DropdownMenu = M.Root;
const DropdownMenuTrigger = M.Trigger;

const DropdownMenuContent = React.forwardRef<React.ElementRef<typeof M.Content>, React.ComponentPropsWithoutRef<typeof M.Content>>(({ className, sideOffset = 4, ...p }, ref) => (
  <M.Portal>
    <M.Content ref={ref} sideOffset={sideOffset} className={cn("z-50 min-w-[8rem] overflow-hidden rounded-md border bg-popover p-1 text-popover-foreground shadow-md", className)} {...p} />
  </M.Portal>
));
DropdownMenuContent.displayName = "DropdownMenuContent";

const DropdownMenuItem = React.forwardRef<React.ElementRef<typeof M.Item>, React.ComponentPropsWithoutRef<typeof M.Item>>(({ className, ...p }, ref) => (
  <M.Item ref={ref} className={cn("relative flex cursor-default select-none items-center rounded-sm px-2 py-1.5 text-sm outline-none focus:bg-accent data-[disabled]:opacity-50", className)} {...p} />
));
DropdownMenuItem.displayName = "DropdownMenuItem";

const DropdownMenuSeparator = React.forwardRef<React.ElementRef<typeof M.Separator>, React.ComponentPropsWithoutRef<typeof M.Separator>>(({ className, ...p }, ref) => (
  <M.Separator ref={ref} className={cn("-mx-1 my-1 h-px bg-border", className)} {...p} />
));
DropdownMenuSeparator.displayName = "DropdownMenuSeparator";

export { DropdownMenu, DropdownMenuTrigger, DropdownMenuContent, DropdownMenuItem, DropdownMenuSeparator };
