import { useEffect } from "react";

/** Sets document.title after render (no-op during prerender). */
export function useDocumentTitle(title: string): void {
  useEffect(() => { document.title = title; }, [title]);
}
