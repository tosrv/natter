import * as React from "react";
import * as D from "@radix-ui/react-dialog";
import { X } from "lucide-react";
import { cn } from "@/lib/utils";

const Dialog = D.Root;
const DialogTrigger = D.Trigger;
const DialogClose = D.Close;

const DialogOverlay = React.forwardRef<React.ElementRef<typeof D.Overlay>, React.ComponentPropsWithoutRef<typeof D.Overlay>>(({ className, ...p }, ref) => (
  <D.Overlay ref={ref} className={cn("fixed inset-0 z-50 bg-black/50 data-[state=open]:animate-in data-[state=closed]:animate-out data-[state=closed]:fade-out-0 data-[state=open]:fade-in-0", className)} {...p} />
));
DialogOverlay.displayName = "DialogOverlay";

const DialogContent = React.forwardRef<React.ElementRef<typeof D.Content>, React.ComponentPropsWithoutRef<typeof D.Content>>(({ className, children, ...p }, ref) => (
  <D.Portal>
    <DialogOverlay />
    <D.Content ref={ref} className={cn("fixed left-1/2 top-1/2 z-50 grid w-full max-w-lg -translate-x-1/2 -translate-y-1/2 gap-4 border bg-background p-6 shadow-lg sm:rounded-lg", className)} {...p}>
      {children}
      <D.Close className="absolute right-4 top-4 rounded-sm opacity-70 hover:opacity-100"><X className="h-4 w-4" /><span className="sr-only">Close</span></D.Close>
    </D.Content>
  </D.Portal>
));
DialogContent.displayName = "DialogContent";

const DialogHeader = ({ className, ...p }: React.HTMLAttributes<HTMLDivElement>) => <div className={cn("flex flex-col space-y-1.5", className)} {...p} />;
const DialogFooter = ({ className, ...p }: React.HTMLAttributes<HTMLDivElement>) => <div className={cn("flex flex-col-reverse sm:flex-row sm:justify-end sm:space-x-2", className)} {...p} />;
const DialogTitle = React.forwardRef<React.ElementRef<typeof D.Title>, React.ComponentPropsWithoutRef<typeof D.Title>>(({ className, ...p }, ref) => <D.Title ref={ref} className={cn("text-lg font-semibold", className)} {...p} />);
DialogTitle.displayName = "DialogTitle";
const DialogDescription = React.forwardRef<React.ElementRef<typeof D.Description>, React.ComponentPropsWithoutRef<typeof D.Description>>(({ className, ...p }, ref) => <D.Description ref={ref} className={cn("text-sm text-muted-foreground", className)} {...p} />);
DialogDescription.displayName = "DialogDescription";

export { Dialog, DialogTrigger, DialogClose, DialogContent, DialogHeader, DialogFooter, DialogTitle, DialogDescription };
