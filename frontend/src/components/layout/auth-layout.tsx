import { useId, type ReactNode } from 'react';

import { Link } from '@tanstack/react-router';
import { ArrowRight } from 'lucide-react';
import { Brand } from './sidebar';

export function AuthLayout({ children, title, description }: { children: ReactNode; title?: string; description?: string }) {
  return <div className="auth-screen"><header><Brand /><Link to="/">На главную <ArrowRight size={16} /></Link></header>
    <main>{title && <div><h1>{title}</h1>{description && <p>{description}</p>}</div>}{children}</main></div>;
}

export function FormCard({ title, description, children, footer }: { title: string; description?: string; children: ReactNode; footer?: ReactNode }) {
  const id = useId();
  return <section className="auth-card" aria-labelledby={id}><h1 id={id}>{title}</h1>{description && <p>{description}</p>}{children}{footer}</section>;
}
