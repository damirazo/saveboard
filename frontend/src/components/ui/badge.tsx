import type { ReactNode } from 'react';

type BadgeProps = { tone: 'violet' | 'mint' | 'pink' | 'amber'; children: ReactNode };

export function Badge({ tone, children }: BadgeProps) {
  return <span className="ui-badge" data-tone={tone}>{children}</span>;
}
