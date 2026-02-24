import DOMPurify from 'dompurify';
import { marked } from 'marked';

interface ProseTextProps {
  content: string;
}

export function ProseText(props: ProseTextProps) {
  const html = () => {
    // Configure marked renderer
    const renderer = new marked.Renderer();

    // Open links in new tab
    renderer.link = ({ href, title, tokens }) => {
      const text = marked.Parser.parseInline(tokens);
      const titleAttr = title ? ` title="${title}"` : '';
      return `<a href="${href}"${titleAttr} target="_blank" rel="noopener noreferrer">${text}</a>`;
    };

    // Wrap tables in a div
    renderer.table = (token) => {
      const header = token.header.map((cell: any) => {
        const content = marked.Parser.parseInline(cell.tokens);
        return `<th>${content}</th>`;
      }).join('');

      const rows = token.rows.map((row: any) => {
        const cells = row.map((cell: any) => {
          const content = marked.Parser.parseInline(cell.tokens);
          return `<td>${content}</td>`;
        }).join('');
        return `<tr>${cells}</tr>`;
      }).join('');

      return `<div class="table-wrapper"><table><thead><tr>${header}</tr></thead><tbody>${rows}</tbody></table></div>`;
    };

    const rawHtml = marked.parse(props.content, { renderer }) as string;
    return DOMPurify.sanitize(rawHtml, {
      ADD_ATTR: ['target', 'rel']
    });
  };

  return (
    <div
      innerHTML={html()}
      class="prose-text"
    />
  );
}
