import { expect, test } from 'vitest';
import { extractText } from './extract.js';

test('extracts readable RU/EN text and removes navigation, forms, scripts and hidden content', () => {
  const paragraph = 'Содержательная статья про хранение знаний и поиск по документам. Unicode: ёж, café, knowledge retrieval. ';
  const result = extractText(`<title>Статья</title><nav>MENU NOISE</nav><article><h1>Статья</h1><p>${paragraph.repeat(5)}</p><p>Второй абзац ${paragraph}</p></article><aside>ADVERTISEMENT</aside><footer>FOOTER NOISE</footer><form>PASSWORD INPUT</form><div hidden>HIDDEN TEXT</div><script>throw new Error('executed')</script>`, 'https://example.com/article');
  expect(result.text).toContain('ёж, café'); expect(result.text).toContain('\n\n');
  for (const noise of ['MENU NOISE', 'ADVERTISEMENT', 'FOOTER NOISE', 'PASSWORD INPUT', 'HIDDEN TEXT', 'executed', '<p>']) expect(result.text).not.toContain(noise);
  expect(result.url).toBe('https://example.com/article');
});
test('rejects empty/unreadable and oversized pages', () => {
  expect(() => extractText('<nav>Menu</nav><script>ignored</script>', 'https://example.com')).toThrow();
  expect(() => extractText('x'.repeat(5 * 1024 * 1024 + 1), 'https://example.com')).toThrow();
});
