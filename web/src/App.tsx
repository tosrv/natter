import { lazy, Suspense, type ReactNode } from "react";
import { BrowserRouter, Navigate, Route, Routes } from "react-router-dom";
import { useTranslation } from "react-i18next";
import { Toaster } from "@/components/ui/sonner";
// Public, prerendered pages are imported eagerly: renderToString cannot wait for lazy(),
// and the first client render must match the prerendered markup for hydration.
import Landing from "@/pages/Landing";
import Privacy from "@/pages/Privacy";
import Terms from "@/pages/Terms";

const Onboarding = lazy(() => import("@/pages/Onboarding"));
const Chat = lazy(() => import("@/pages/Chat"));
const Settings = lazy(() => import("@/pages/Settings"));

function RouteFallback() {
  const { t } = useTranslation();
  return <p className="p-6 text-muted-foreground">{t("common.loading")}</p>;
}

/** Router-agnostic tree, shared by the browser (BrowserRouter) and the prerenderer (StaticRouter). */
/** `children` render inside the Suspense boundary, so their effects run only once the routes have hydrated. */
export function AppRoutes({ children }: { children?: ReactNode }) {
  return (
    <>
      <Suspense fallback={<RouteFallback />}>
        <Routes>
          <Route path="/" element={<Landing />} />
          <Route path="/onboarding" element={<Onboarding />} />
          <Route path="/terms" element={<Terms />} />
          <Route path="/privacy" element={<Privacy />} />
          <Route path="/chat" element={<Chat />} />
          <Route path="/settings" element={<Settings />} />
          <Route path="*" element={<Navigate to="/" replace />} />
        </Routes>
        {children}
      </Suspense>
      <Toaster />
    </>
  );
}

export default function App({ children }: { children?: ReactNode }) {
  return <BrowserRouter><AppRoutes>{children}</AppRoutes></BrowserRouter>;
}
