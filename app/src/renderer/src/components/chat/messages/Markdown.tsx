import ReactMarkdown from 'react-markdown'
import remarkGfm from 'remark-gfm'
import remarkMath from 'remark-math'
import rehypeRaw from 'rehype-raw'
import rehypeHighlight from 'rehype-highlight'
import rehypeKatex from 'rehype-katex'
import * as React from 'react'
import { cn } from '@renderer/lib/utils'
import { useCallback } from 'react'
import { CopyButton } from '../../ui/CopyButton'
import { useTheme } from '@renderer/lib/theme'
import 'katex/dist/katex.min.css'

interface CodeBlockProps {
  children: React.ReactNode
  className?: string
  language?: string
}

function CodeBlock({ children, className, language }: CodeBlockProps) {
  const codeRef = React.useRef<HTMLElement>(null)
  const { theme } = useTheme()

  const extractText = useCallback((node: React.ReactNode): string => {
    if (typeof node === 'string') return node
    if (typeof node === 'number') return node.toString()
    if (Array.isArray(node)) return node.map(extractText).join('')
    if (node && typeof node === 'object' && 'props' in node) {
      const element = node as React.ReactElement & { props: { children?: React.ReactNode } }
      return extractText(element.props.children)
    }
    return ''
  }, [])

  const codeText = React.useMemo(() => {
    let codeText = ''
    if (codeRef.current) {
      codeText = codeRef.current.textContent || ''
    }
    if (!codeText) {
      codeText = extractText(children)
    }
    return codeText
  }, [children, extractText])

  return (
    <div className={cn('relative group/codeblock mt-2 mb-4', theme === 'dark' && 'dark')}>
      {language && (
        <div className="flex select-none items-center justify-between bg-muted px-4 py-2 pb-0 rounded-t-md">
          <span className="text-xs font-mono text-muted-foreground uppercase">{language}</span>
          <CopyButton showLabel text={codeText} />
        </div>
      )}
      <pre
        className={cn(
          `w-full p-4 bg-muted max-w-full overflow-x-auto font-mono`,
          language ? 'rounded-t-none rounded-b-md' : 'rounded-md',
          className
        )}
      >
        <code ref={codeRef}>{children}</code>
      </pre>
    </div>
  )
}

export default function Markdown({ children }: { children: string; isChat?: boolean }) {
  const handleLinkClick = (e: React.MouseEvent<HTMLAnchorElement>, href: string) => {
    e.preventDefault()
    if (!href) return
    window.open(href, '_blank', 'noopener,noreferrer')
  }

  // Convert LaTeX delimiters from \( \) to $ $ and \[ \] to $$ $$
  const processLatexDelimiters = (content: string): string => {
    return (
      content
        // Convert \( ... \) to $ ... $
        .replace(/\\\((.*?)\\\)/g, '$$$1$$')
        // Convert \[ ... \] to $$ ... $$
        .replace(/\\\[(.*?)\\\]/gs, '$$$$$$1$$$$$$')
        // Also handle the case where they're wrapped in parentheses like ( \frac{a}{b} )
        .replace(/\(\s*\\([^)]+)\s*\)/g, '$$$$$1$$$')
    )
  }

  return (
    <ReactMarkdown
      remarkPlugins={[remarkGfm, remarkMath]}
      rehypePlugins={[rehypeRaw, rehypeHighlight, rehypeKatex]}
      components={{
        p: ({ children, ...props }) => (
          <p className="text-base font-normal leading-relaxed mb-3" {...props}>
            {children}
          </p>
        ),
        strong: ({ children, ...props }) => (
          <strong className="font-bold" {...props}>
            {children}
          </strong>
        ),
        img: ({ alt, ...props }: React.ImgHTMLAttributes<HTMLImageElement>) => (
          <img {...props} alt={alt || 'markdown image'} className="w-full h-auto rounded-md" />
        ),
        pre: ({ className, children, ...props }) => {
          const codeElement = Array.isArray(children) ? children[0] : children
          const codeProps = (codeElement as React.ReactElement)?.props as { className?: string }
          const language = codeProps?.className?.match(/language-(\w+)/)?.[1]

          return (
            <CodeBlock className={className} language={language} {...props}>
              {children}
            </CodeBlock>
          )
        },
        code: ({ className, children, ...props }) => {
          const match = /language-(\w+)/.exec(className || '')
          const isInline = !match

          if (isInline) {
            return (
              <code className={cn(`rounded bg-muted px-1 py-0.5 text-sm`, className)} {...props}>
                {children}
              </code>
            )
          }

          return (
            <code className={cn(`text-sm`, className)} {...props}>
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
          <ul
            className="list-disc pl-6 my-3 space-y-1 [&_ul]:list-[circle] [&_ul_ul]:list-[square] [&_ul]:mt-1 [&_ul]:mb-0"
            {...props}
          >
            {children}
          </ul>
        ),
        ol: ({ children, ...props }) => (
          <ol className="list-decimal pl-6 my-3 space-y-1" {...props}>
            {children}
          </ol>
        ),
        li: ({ children, ...props }) => (
          <li className="leading-relaxed" {...props}>
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
          <thead className="bg-muted" {...props}>
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
            className="pl-4 border-l-4 border-accent italic my-4 text-muted-foreground leading-relaxed"
            {...props}
          >
            {children}
          </blockquote>
        ),
        hr: (props) => <hr className="my-6 border-t border-border" {...props} />,
        h1: ({ children, ...props }) => (
          <h1 className="text-2xl font-bold mt-8 mb-4 leading-tight" {...props}>
            {children}
          </h1>
        ),
        h2: ({ children, ...props }) => (
          <h2 className="text-xl font-bold mt-6 mb-3 leading-tight" {...props}>
            {children}
          </h2>
        ),
        h3: ({ children, ...props }) => (
          <h3 className="text-lg font-bold mt-5 mb-2 leading-tight" {...props}>
            {children}
          </h3>
        ),
        h4: ({ children, ...props }) => (
          <h4 className="text-base font-bold mt-4 mb-2 leading-tight" {...props}>
            {children}
          </h4>
        ),
        h5: ({ children, ...props }) => (
          <h5 className="text-sm font-bold mt-4 mb-2 leading-tight" {...props}>
            {children}
          </h5>
        ),
        h6: ({ children, ...props }) => (
          <h6 className="text-xs font-bold mt-4 mb-2 leading-tight" {...props}>
            {children}
          </h6>
        )
      }}
    >
      {processLatexDelimiters(children)}
    </ReactMarkdown>
  )
}
