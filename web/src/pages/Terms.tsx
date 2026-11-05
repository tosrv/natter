import LegalPage from "@/components/LegalPage";
import { useDocumentTitle } from "@/hooks/useDocumentTitle";
import { pageMeta } from "@/seo/meta";

export default function Terms() {
  useDocumentTitle(pageMeta["/terms"].title);
  return <LegalPage ns="terms" />;
}
