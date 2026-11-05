import { useSyncExternalStore } from "react";
import { getThemePref, isDarkNow, subscribeDark, subscribePref, type ThemePref } from "@/lib/theme";

// The server snapshot is also used during hydration, so prerendered markup always matches
// the first client render; the real value is applied right after hydration.
export const useIsDark = () => useSyncExternalStore(subscribeDark, isDarkNow, () => false);
export const useThemePref = () => useSyncExternalStore<ThemePref>(subscribePref, getThemePref, () => "system");
