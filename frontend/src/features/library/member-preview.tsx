import { useSearch } from '@tanstack/react-router';
import { LibraryPage } from './library-page';

export function MemberPreview() {
  const { section } = useSearch({ from: '/feed' });
  return <LibraryPage member section={section} />;
}
