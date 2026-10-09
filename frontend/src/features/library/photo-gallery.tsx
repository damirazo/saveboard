import { useEffect, useRef, useState } from 'react';
import Lightbox from 'yet-another-react-lightbox';
import 'yet-another-react-lightbox/styles.css';
import { type Photo } from './post-api';
import { usePhotoURLs } from './use-photo-urls';
import { Button } from '../../components/ui/button';

export function PhotoGallery({ photos }: { photos: Photo[] }) {
  const urls = usePhotoURLs(photos); const grid = useRef<HTMLDivElement>(null);
  const [columns, setColumns] = useState(1); const [expanded, setExpanded] = useState(false); const [index, setIndex] = useState(-1);
  useEffect(() => {
    const element = grid.current; if (!element) return;
    const observer = new ResizeObserver(() => setColumns(getComputedStyle(element).gridTemplateColumns.split(' ').length));
    observer.observe(element); return () => observer.disconnect();
  }, []);
  const visible = expanded ? photos : photos.slice(0, columns * 3);
  return <section className="photo-gallery">
    <div className="photo-grid" ref={grid}>{visible.map((p, i) => <button type="button" key={p.id} disabled={!urls[p.id]} onClick={() => setIndex(i)} aria-label={`Открыть фото ${i + 1}`}>{urls[p.id] ? <img src={urls[p.id]} alt={`Фото ${i + 1}`} loading="lazy" /> : <span>Фото загружается или недоступно</span>}</button>)}</div>
    {photos.length > columns * 3 && <Button variant="outline" aria-expanded={expanded} onClick={() => setExpanded(!expanded)}>{expanded ? 'Свернуть фото' : `Показать все фото (${photos.length})`}</Button>}
    <Lightbox open={index >= 0} close={() => setIndex(-1)} index={index} slides={photos.map((p, i) => ({ src: urls[p.id] || '', alt: `Фото ${i + 1}` }))} labels={{ Close: 'Закрыть', Next: 'Следующее фото', Previous: 'Предыдущее фото' }} />
  </section>;
}
