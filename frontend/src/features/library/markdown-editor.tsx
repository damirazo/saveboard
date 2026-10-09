import { useState } from 'react';
import { MDXEditor, headingsPlugin, listsPlugin, quotePlugin, linkPlugin, linkDialogPlugin, toolbarPlugin, BoldItalicUnderlineToggles, ListsToggle, BlockTypeSelect, CreateLink, UndoRedo, codeBlockPlugin, codeMirrorPlugin, tablePlugin, thematicBreakPlugin, markdownShortcutPlugin } from '@mdxeditor/editor';
import '@mdxeditor/editor/style.css';
import { Button } from '../../components/ui/button';

export default function MarkdownEditor({ value, onChange, disabled }: { value: string; onChange: (value: string) => void; disabled: boolean }) {
  const [source, setSource] = useState(false);
  const [error, setError] = useState('');
  return <section className="markdown-editor" aria-label="Текст заметки">
    <div className="editor-modes"><Button variant="filter" aria-pressed={!source} onClick={() => setSource(false)}>Визуальный</Button><Button variant="filter" aria-pressed={source} onClick={() => setSource(true)}>Markdown</Button></div>
    {error && <p role="alert">{error} Переключитесь в Markdown для исправления.</p>}
    {source ? <textarea aria-label="Markdown" value={value} maxLength={100000} disabled={disabled} onChange={e => { onChange(e.target.value); setError(''); }} rows={12} /> :
      <MDXEditor markdown={value} onChange={onChange} readOnly={disabled} suppressHtmlProcessing onError={() => setError('Разметка содержит неподдерживаемые элементы.')} contentEditableClassName="markdown-body" plugins={[
        headingsPlugin(), listsPlugin(), quotePlugin(), tablePlugin(), thematicBreakPlugin(), markdownShortcutPlugin(), linkPlugin({ disableAutoLink: true }), linkDialogPlugin(),
        codeBlockPlugin({ defaultCodeBlockLanguage: 'txt' }), codeMirrorPlugin({ codeBlockLanguages: { txt: 'Текст', js: 'JavaScript', json: 'JSON', go: 'Go' } }),
        toolbarPlugin({ toolbarContents: () => <><UndoRedo /><BoldItalicUnderlineToggles options={['Bold', 'Italic']} /><BlockTypeSelect /><ListsToggle /><CreateLink /></> }),
      ]} />}
    <small>Markdown без HTML и встроенных изображений. Ссылки — только HTTP(S).</small>
  </section>;
}
