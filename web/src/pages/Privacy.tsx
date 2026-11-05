import LegalPage from "@/components/LegalPage";
import { useDocumentTitle } from "@/hooks/useDocumentTitle";
import { pageMeta } from "@/seo/meta";

export default function Privacy() {
  useDocumentTitle(pageMeta["/privacy"].title);
  return <LegalPage ns="privacy" />;
}
