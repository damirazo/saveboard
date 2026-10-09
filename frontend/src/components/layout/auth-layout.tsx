import { useId, type ReactNode } from 'react';
import { BookMarked } from 'lucide-react';

export function AuthLayout({ title, description, children }: { title: string; description: string; children: ReactNode }) {
  return <div className="auth-layout"><section className="auth-intro"><span className="auth-symbol"><BookMarked aria-hidden="true" /></span><p className="eyebrow">ВАША ЛИЧНАЯ БИБЛИОТЕКА</p><h1>{title}</h1><p>{description}</p><div className="auth-intro-note"><span>01</span><p>Ссылки, заметки и файлы.<br /><strong>С контекстом, который важен вам.</strong></p></div></section>{children}</div>;
}

export function FormCard({ title, description, children, footer }: { title: string; description?: string; children: ReactNode; footer?: ReactNode }) {
  const id = useId();
  return <section className="auth-card" aria-labelledby={id}><h2 id={id}>{title}</h2>{description && <p>{description}</p>}{children}{footer}</section>;
}
