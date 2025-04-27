import ReactMarkdown from 'react-markdown'
import remarkGfm from 'remark-gfm'
import rehypeRaw from 'rehype-raw'

export default function Markdown({ children }: { children: string; isChat?: boolean }) {
  const handleLinkClick = (e: React.MouseEvent<HTMLAnchorElement>, href: string) => {
    e.preventDefault()
    if (!href) return
    window.open(href, '_blank', 'noopener,noreferrer')
  }

  return (
    <ReactMarkdown
      remarkPlugins={[remarkGfm]}
      rehypePlugins={[rehypeRaw]}
      components={{
        p: ({ children, ...props }) => (
          <p
            style={{
              marginBottom: '0.5rem',
              fontSize: '1rem',
              lineHeight: '1.5'
            }}
            {...props}
          >
            {children}
          </p>
        ),
        strong: ({ children, ...props }) => (
          <strong style={{ fontWeight: 'bold' }} {...props}>
            {children}
          </strong>
        ),
        img: ({ ...props }: React.ImgHTMLAttributes<HTMLImageElement>) => (
          <img
            {...props}
            style={{
              maxWidth: '100%',
              height: '200px',
              borderRadius: '0.5rem'
            }}
            alt={props.alt || 'markdown image'}
          />
        ),
        pre: ({ className, children, ...props }) => (
          <pre
            className={`w-full max-w-full overflow-x-auto rounded-2xl border border-border bg-background p-2 ${className || ''}`}
            {...props}
          >
            {children}
          </pre>
        ),
        code: ({ className, children, ...props }) => {
          return (
            <code
              className={`rounded bg-gray-100 px-1 py-0.5 text-sm ${className || ''}`}
              {...props}
            >
              {children}
            </code>
          )
        },
        a: ({ href, children, ...props }) => (
          <a
            href={href}
            onClick={(e) => handleLinkClick(e, href || '')}
            className="text-primary underline hover:text-primary/80 transition-colors"
            {...props}
          >
            {children}
          </a>
        ),
        ul: ({ children, ...props }) => (
          <ul className="list-disc pl-6 my-2" {...props}>
            {children}
          </ul>
        ),
        ol: ({ children, ...props }) => (
          <ol className="list-decimal pl-6 my-2" {...props}>
            {children}
          </ol>
        ),
        li: ({ children, ...props }) => (
          <li className="mb-1" {...props}>
            {children}
          </li>
        ),
        table: ({ children, ...props }) => (
          <div className="overflow-x-auto my-4">
            <table className="min-w-full border-collapse border border-border" {...props}>
              {children}
            </table>
          </div>
        ),
        thead: ({ children, ...props }) => (
          <thead className="bg-gray-100" {...props}>
            {children}
          </thead>
        ),
        tbody: ({ children, ...props }) => <tbody {...props}>{children}</tbody>,
        tr: ({ children, ...props }) => (
          <tr className="border-b border-border" {...props}>
            {children}
          </tr>
        ),
        th: ({ children, ...props }) => (
          <th
            className="px-4 py-2 text-left font-medium border-r border-border last:border-r-0"
            {...props}
          >
            {children}
          </th>
        ),
        td: ({ children, ...props }) => (
          <td className="px-4 py-2 border-r border-border last:border-r-0" {...props}>
            {children}
          </td>
        ),
        blockquote: ({ children, ...props }) => (
          <blockquote
            className="pl-4 border-l-4 border-gray-300 italic my-2 text-gray-700"
            {...props}
          >
            {children}
          </blockquote>
        ),
        hr: (props) => <hr className="my-4 border-t border-border" {...props} />,
        h1: ({ children, ...props }) => (
          <h1 className="text-2xl font-bold mt-6 mb-4" {...props}>
            {children}
          </h1>
        ),
        h2: ({ children, ...props }) => (
          <h2 className="text-xl font-bold mt-5 mb-3" {...props}>
            {children}
          </h2>
        ),
        h3: ({ children, ...props }) => (
          <h3 className="text-lg font-bold mt-4 mb-2" {...props}>
            {children}
          </h3>
        ),
        h4: ({ children, ...props }) => (
          <h4 className="text-base font-bold mt-3 mb-2" {...props}>
            {children}
          </h4>
        ),
        h5: ({ children, ...props }) => (
          <h5 className="text-sm font-bold mt-3 mb-1" {...props}>
            {children}
          </h5>
        ),
        h6: ({ children, ...props }) => (
          <h6 className="text-xs font-bold mt-3 mb-1" {...props}>
            {children}
          </h6>
        )
      }}
    >
      {children}
    </ReactMarkdown>
  )
}
