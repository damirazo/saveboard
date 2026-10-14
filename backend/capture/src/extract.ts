import { Readability } from '@mozilla/readability';
import { JSDOM } from 'jsdom';

// jsdom never executes scripts or loads resources. Return plain text only.
export function extractText(html: string, url: string) {
  if (Buffer.byteLength(html) > 5 * 1024 * 1024) throw new Error('DOM too large');
  const dom = new JSDOM(html, { url });
  try {
    const doc = dom.window.document;
    doc.querySelectorAll('script,style,noscript,nav,footer,aside,form,button,iframe,object,embed,svg,canvas,[hidden],[aria-hidden="true"],[role="navigation"],[role="dialog"],[role="banner"],[role="contentinfo"]').forEach(el => el.remove());
    doc.querySelectorAll('p,div,section,h1,h2,h3,h4,li,br,tr,blockquote,pre').forEach(el => el.append(doc.createTextNode('\n')));
    const article = new Readability(doc, { maxElemsToParse: 50000, charThreshold: 80, disableJSONLD: true }).parse();
    if (!article) throw new Error('No readable article');
    // eslint-disable-next-line no-control-regex -- Remove control characters from untrusted plain text.
    const text = (article.textContent ?? '').normalize('NFC').replace(/[\u0000-\u0008\u000b\u000c\u000e-\u001f\u007f]/g, '').split('\n').map(line => line.replace(/\s+/g, ' ').trim()).filter(Boolean).join('\n\n');
    if (text.length < 80 || Array.from(text).length > 100000) throw new Error('Article text outside limits');
    return { title: (article.title ?? '').trim().slice(0, 500), text, url };
  } finally { dom.window.close(); }
}
