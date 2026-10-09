import { useEffect, useRef, useState } from 'react';
import { photoBlob, type Photo } from './post-api';

export function usePhotoURLs(photos: Photo[]) {
  const [loaded, setLoaded] = useState<Record<string, string>>({});
  const resources = useRef(new Map<string, { controller: AbortController; url?: string }>());
  const key = photos.map(p => `${p.id}:${p.url}`).join('|');
  useEffect(() => {
    const current = resources.current;
    const wanted = new Set(photos.map(p => p.id));
    for (const [id, resource] of current) {
      if (!wanted.has(id)) { resource.controller.abort(); if (resource.url) URL.revokeObjectURL(resource.url); current.delete(id); }
    }
    for (const photo of photos) {
      if (current.has(photo.id)) continue;
      const resource: { controller: AbortController; url?: string } = { controller: new AbortController() };
      current.set(photo.id, resource);
      photoBlob(photo, resource.controller.signal).then(blob => {
        if (resource.controller.signal.aborted) return;
        resource.url = URL.createObjectURL(blob);
        setLoaded(previous => ({ ...previous, [photo.id]: resource.url! }));
      }).catch(() => { /* Keep the unavailable placeholder; do not expose private image URLs. */ });
    }
    // The key contains all IDs and endpoint URLs; each live resource is fetched once.
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [key]);
  useEffect(() => {
    const current = resources.current;
    return () => { for (const resource of current.values()) { resource.controller.abort(); if (resource.url) URL.revokeObjectURL(resource.url); } current.clear(); };
  }, []);
  return loaded;
}
