import { Badge } from '../ui/badge';
import { postTypes } from './post-types';

export function PostTypeBadge({ type }: { type: string }) {
  const entry = postTypes.find(item => item.value === type);
  if (!entry) return null;
  const { Icon, label, tone } = entry;
  return <Badge tone={tone}><Icon aria-hidden="true" />{label}</Badge>;
}
