import { createFileRoute } from '@tanstack/react-router'

export const Route = createFileRoute('/privacy')({
  component: PrivacyPage
})

function PrivacyPage() {
  return (
    <div className="flex flex-col items-center pt-20 min-h-screen w-full mx-auto p-6 overflow-y-auto scrollbar-thin scrollbar-thumb-gray-300 voice-chat-scrollbar">
      <div className="w-full max-w-3xl">
        <h1 className="text-4xl font-bold mb-8 text-center">Privacy-preserving AI interface</h1>

        <div className="flex flex-col gap-4 prose prose-gray dark:prose-invert max-w-none pb-20">
          <p className="text-base leading-relaxed">
            You want to use the best LLMs available today: Opus 4, o3 pro, and others. However,
            sharing personal information with these services creates real privacy risks. Your
            queries might contain sensitive details about your work, health, finances, or personal
            life. Even with encryption in transit, your data still reaches the model provider&apos;s
            servers where it could potentially be logged, analyzed, or exposed in a breach.
          </p>
          <h2 className="text-2xl font-semibold mt-2">Why not just use TEEs?</h2>
          <p className="text-base leading-relaxed mb-2">
            You might wonder: why not run these models inside Trusted Execution Environments (TEEs)
            with hardware-level privacy guarantees? While we can and do serve open-source models
            from GPU-enabled TEEs, this approach hits fundamental limitations:
          </p>
          <ul className="list-disc list-inside space-y-3">
            <li className="text-base">
              The best models are closed-source: Claude, GPT-4, and other leading models aren&apos;t
              available for self-hosting
            </li>
            <li className="text-base">
              Hardware constraints: Modern language models are massive, a single H100 GPU can&apos;t
              host all 671B parameters of the best open source model (currently, DeepSeek R1), and
              confidential compute with PCIe bus encryption only works within a single GPU.
            </li>
            <li className="text-base">
              Access restrictions: Model providers understandably protect their intellectual
              property and won&apos;t allow third-party hosting
            </li>
          </ul>
          <p className="text-base leading-relaxed mb-2">
            Current privacy solutions force you to choose between capability and confidentiality.
            You can run smaller, less capable models locally or in TEEs for privacy, or you can use
            powerful closed-source models and accept the privacy trade-offs. We believe you
            shouldn&apos;t have to make this choice.
          </p>
          <h2 className="text-2xl font-semibold mt-2">
            Our approach: deterministic anonymization with semantic preservation
          </h2>
          <p className="text-base leading-relaxed mb-2">
            Instead of trusting model providers with your raw data, our system automatically
            identifies and replaces private information before your query ever leaves your device.
            Unlike other approaches that try to rewrite your entire prompt (often losing important
            context), we use a surgical approach: replace only what needs replacing, and do it
            consistently.
          </p>
          <h3 className="text-xl font-semibold">How it works</h3>
          <ul className="list-decimal list-inside space-y-3">
            <li className="text-base">
              <strong>Local identification:</strong> A small models running entirely on your device
              identifies private information in your query
            </li>
            <li className="text-base">
              <strong>Smart replacement:</strong> Each piece of private data is replaced with a
              semantically equivalent alternative that preserves the context needed for a good
              response
            </li>
            <li className="text-base">
              <strong>Secure routing:</strong> Your anonymized query is sent through
              privacy-preserving network layers to reach the model
            </li>
            <li className="text-base">
              <strong>Automatic restoration:</strong> When the response comes back, we automatically
              restore your original information
            </li>
          </ul>
          <h3 className="text-xl font-semibold">Example in action</h3>
          <p className="text-base leading-relaxed mb-2 font-bold">Three connected queries:</p>
          <div className="flex flex-col gap-4">
            <div className="bg-muted/30 rounded-lg p-4 border-l-4 border-primary">
              <p className="text-sm font-medium text-muted-foreground mb-2">Query 1:</p>
              <p className="text-base">
                &quot;I discovered my manager at Google is systematically inflating sales numbers
                for the cloud infrastructure division&quot;
              </p>
            </div>

            <div className="bg-muted/30 rounded-lg p-4 border-l-4 border-primary">
              <p className="text-sm font-medium text-muted-foreground mb-2">Query 2:</p>
              <p className="text-base">
                &quot;I&apos;m considering becoming a whistleblower to the SEC about financial fraud
                at my tech company - could this affect my H1-B visa status?&quot;
              </p>
            </div>

            <div className="bg-muted/30 rounded-lg p-4 border-l-4 border-primary">
              <p className="text-sm font-medium text-muted-foreground mb-2">Query 3:</p>
              <p className="text-base">
                &quot;My skip-level is Jennifer who reports directly to Marc - should I talk to her
                first or go straight to the authorities?&quot;
              </p>
            </div>
          </div>
          <p className="text-base leading-relaxed mb-2 font-bold mt-2">
            What the model provider sees (three separate, unconnected queries):
          </p>
          <div className="flex flex-col gap-4">
            <div className="bg-muted/30 rounded-lg p-4 border-l-4 border-primary">
              <p className="text-sm font-medium text-muted-foreground mb-2">Query 1:</p>
              <p className="text-base">
                &quot;I discovered my manager at TechCorp is systematically inflating sales numbers
                for the enterprise software division&quot;
              </p>
            </div>

            <div className="bg-muted/30 rounded-lg p-4 border-l-4 border-primary">
              <p className="text-sm font-medium text-muted-foreground mb-2">Query 2:</p>
              <p className="text-base">
                &quot;I&apos;m considering becoming a whistleblower to the SEC about financial fraud
                at my tech company - could this affect my H1-B visa status?&quot;
              </p>
            </div>

            <div className="bg-muted/30 rounded-lg p-4 border-l-4 border-primary">
              <p className="text-sm font-medium text-muted-foreground mb-2">Query 3:</p>
              <p className="text-base">
                &quot;My skip-level is Michelle who reports directly to Robert - should I talk to
                her first or go straight to the authorities?&quot;
              </p>
            </div>
          </div>
          <p className="text-base leading-relaxed mb-2">
            Connected together, these queries would let Google instantly identify the whistleblower
            - there&apos;s probably only one H1-B employee in cloud infrastructure whose skip-level
            is Jennifer reporting to Larry. But as three anonymous queries from different
            &quot;people,&quot; you get solid legal advice while staying completely protected. The
            model provider doesn&apos;t know these queries are related, but still provides help.
          </p>
          <h2 className="text-2xl font-semibold mt-2">The privacy guarantees</h2>
          <h3 className="text-xl font-semibold">Content-level protection</h3>
          <p className="text-base leading-relaxed">
            Our deterministic anonymization follows these principles:
          </p>
          <ul className="list-disc list-inside space-y-3">
            <li className="text-base">
              <strong>Personal names</strong> are replaced with culturally and contextually similar
              alternatives
            </li>
            <li className="text-base">
              <strong>Company names</strong> become fictional entities from the same industry and
              size
            </li>
            <li className="text-base">
              <strong>Locations</strong> under 100k population are mapped to equivalent synthetic
              locations
            </li>
            <li className="text-base">
              <strong>Dates and times</strong> are shifted consistently to preserve relative timing
            </li>
            <li className="text-base">
              <strong>Financial amounts</strong> are adjusted within a small range to maintain
              context Identifiers (emails, phone numbers, URLs) are replaced with format-valid
              dummies
            </li>
          </ul>

          <h3 className="text-xl font-semibold">Network-level protection</h3>
          <p>
            Even with perfect content anonymization, your query patterns could reveal information.
            We add network-level privacy through:
          </p>
          <ul className="list-decimal list-inside space-y-3">
            <li className="text-base">
              <strong>Multi-hop routing:</strong> Your queries are encrypted and routed through
              intermediate nodes, similar to Tor
            </li>
            <li className="text-base">
              <strong>TEE proxies:</strong> Optional routing through Trusted Execution Environments
              that cryptographically guarantee they don&apos;t log or store your queries
            </li>
            <li className="text-base">
              <strong>Traffic mixing:</strong> Your queries blend with thousands of others, making
              individual tracking statistically impossible
            </li>
          </ul>

          <h2 className="text-2xl font-semibold mt-2">What this means for your privacy</h2>

          <h3 className="text-xl font-semibold">What we protect</h3>
          <ul className="list-disc list-inside space-y-3">
            <li className="text-base">
              <strong>Identity:</strong> Model providers cannot connect queries to you personally
            </li>
            <li className="text-base">
              <strong>Relationships:</strong> Names, companies, and associations remain private
            </li>
            <li className="text-base">
              <strong>Location:</strong> Your specific geographic information stays confidential
            </li>
            <li className="text-base">
              <strong>Timing:</strong> Exact dates and schedules are obscured
            </li>
            <li className="text-base">
              <strong>Financial data:</strong> Amounts and transactions are protected
            </li>
            <li className="text-base">
              <strong>Writing patterns:</strong> Network routing prevents behavioral fingerprinting
            </li>
          </ul>

          <h3 className="text-xl font-semibold">What remains visible</h3>
          <ul className="list-disc list-inside space-y-3">
            <li className="text-base">
              <strong>General topics:</strong> The LLM still knows you&apos;re asking about meeting
              preparation, coding, or whatever your query concerns
            </li>
            <li className="text-base">
              <strong>Language and structure:</strong> Your query&apos;s grammatical structure and
              language remain unchanged
            </li>
            <li className="text-base">
              <strong>Public information:</strong> Well-known facts, and common knowledge
              aren&apos;t altered
            </li>
          </ul>

          <h2 className="text-2xl font-semibold mt-2">Performance and user experience</h2>

          <p className="text-base leading-relaxed mb-2">
            Our goal is to build this system and only add minimal overhead to your LLM interactions:
          </p>

          <ul className="list-disc list-inside space-y-3">
            <li className="text-base">
              <strong>Latency:</strong> Less than 500ms added to query time (targeting sub-100ms in
              future versions)
            </li>
            <li className="text-base">
              <strong>Quality:</strong> Response quality matches direct API usage in over 99% of
              cases
            </li>
            <li className="text-base">
              <strong>Reliability:</strong> Deterministic replacement ensures consistent behavior
            </li>
            <li className="text-base">
              <strong>Compatibility:</strong> Works with any text-based LLM API
            </li>
          </ul>

          <h2 className="text-2xl font-semibold mt-2">Understanding the limitations</h2>

          <p className="text-base leading-relaxed mb-2">
            While our approach significantly improves privacy, some limitations exist:
          </p>

          <ul className="list-disc list-inside space-y-3">
            <li className="text-base">
              <strong>Network strength:</strong> Privacy routing requires building either a network
              of relay nodes with similar availability to the Tor network or replicate it with
              trusted execution environments that adds a hardware trust assumption
            </li>
            <li className="text-base">
              <strong>Trust boundaries:</strong> You&apos;re still trusting our local anonymization
              model and routing infrastructure
            </li>
            <li className="text-base">
              <strong>Inference limits:</strong> Extremely unique queries might still be
              theoretically traceable despite anonymization
            </li>
          </ul>

          <h2 className="text-2xl font-semibold mt-2">Making an informed decision</h2>

          <p className="text-base leading-relaxed mb-2">We designed this system for users who:</p>

          <ul className="list-disc list-inside space-y-3">
            <li className="text-base">Want to use powerful LLMs for sensitive tasks</li>
            <li className="text-base">Value practical privacy improvements</li>
            <li className="text-base">
              Are comfortable with the trade-offs between utility and anonymization
            </li>
          </ul>

          <p className="text-base leading-relaxed mb-2">
            By combining local content anonymization with network-level privacy protection, we
            create multiple independent barriers against various attack vectors.
          </p>

          <h2 className="text-2xl font-semibold mt-2">Technical transparency</h2>

          <p className="text-base leading-relaxed mb-2">
            For those interested in the implementation details:
          </p>

          <ul className="list-disc list-inside space-y-3">
            <li className="text-base">
              <strong>Local model:</strong> &lt;3B parameters, optimized for on-device inference
            </li>
            <li className="text-base">
              <strong>Anonymization:</strong> Deterministic mapping with semantic buckets
            </li>
            <li className="text-base">
              <strong>Network routing:</strong> 2-hop minimum or single TEE proxy hop
            </li>
            <li className="text-base">
              <strong>Open audit:</strong> Anonymization rules and model behavior are verifiable
            </li>
          </ul>

          <p className="text-base leading-relaxed mb-2">
            We believe privacy shouldn&apos;t require blind trust. Our approach is designed to be
            understandable, verifiable, and aligned with your privacy expectations.
          </p>
        </div>
      </div>

      <style>{`
        .voice-chat-scrollbar::-webkit-scrollbar {
          width: 8px;
        }
        .voice-chat-scrollbar::-webkit-scrollbar-track {
          background: #f1f1f1;
          border-radius: 4px;
        }
        .voice-chat-scrollbar::-webkit-scrollbar-thumb {
          background: #888;
          border-radius: 4px;
        }
        .voice-chat-scrollbar::-webkit-scrollbar-thumb:hover {
          background: #555;
        }
      `}</style>
    </div>
  )
}
