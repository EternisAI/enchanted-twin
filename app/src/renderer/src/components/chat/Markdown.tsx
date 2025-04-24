import ReactMarkdown from 'react-markdown'
import remarkGfm from 'remark-gfm'
import rehypeRaw from 'rehype-raw'

export const Markdown = ({ children }: { children: string; isChat?: boolean }) => {
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
        )
      }}
    >
      {children}
    </ReactMarkdown>
  )
}

export default Markdown
