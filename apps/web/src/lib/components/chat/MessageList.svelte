<script lang="ts">
  import Markdown from '$lib/components/chat/Markdown.svelte';
  import Button from '$lib/components/ui/Button.svelte';
  import type { ChatMessage, ChatState } from '$lib/types';
  import { Copy, Check } from 'lucide-svelte';

  export let messages: ChatMessage[] = [];
  export let state: ChatState = 'idle';
  export let emptyTitle = 'Ask Jute anything';
  export let emptyMessage = 'Choose an agent and start with a short request.';
  export let onRetry: (message: ChatMessage) => Promise<void> | void = () => {};
  export let onSelectArtifact: (artifact: {
    id: string;
    title: string;
    content: string;
  }) => void = () => {};

  let stepsExpanded: Record<string, boolean> = {};
  let stepExpansion: Record<string, boolean> = {};
  let copiedId = '';
  let copiedTimeout: ReturnType<typeof setTimeout> | undefined;

  function toggleSteps(messageId: string, messageStatus?: string) {
    const current = isExpanded(messageId, messageStatus);
    stepsExpanded = {
      ...stepsExpanded,
      [messageId]: !current
    };
  }

  function isExpanded(messageId: string, messageStatus?: string) {
    if (stepsExpanded[messageId] !== undefined) {
      return stepsExpanded[messageId];
    }
    return messageStatus === 'streaming' || messageStatus === 'sending';
  }

  function toggleStep(stepId: string) {
    stepExpansion = {
      ...stepExpansion,
      [stepId]: !stepExpansion[stepId]
    };
  }

  function isStepExpanded(step: any) {
    if (stepExpansion[step.id] !== undefined) {
      return stepExpansion[step.id];
    }
    // Expand by default if currently active
    return step.status === 'thinking' || step.status === 'working';
  }

  function copyToClipboard(text: string, messageId: string) {
    if (!text) return;
    navigator.clipboard.writeText(text).then(() => {
      copiedId = messageId;
      if (copiedTimeout) clearTimeout(copiedTimeout);
      copiedTimeout = setTimeout(() => {
        copiedId = '';
      }, 2000);
    });
  }
</script>

<div class="message-list" aria-live="polite">
  {#if messages.length === 0}
    <div class="chat-empty">
      <div class="chat-empty-title">{emptyTitle}</div>
      <p>{emptyMessage}</p>
    </div>
  {:else}
    {#each messages as message (message.id)}
      <article
        class="message-bubble"
        class:message-bubble--user={message.role === 'user'}
        class:message-bubble--assistant={message.role === 'assistant'}
        class:message-bubble--system={message.role === 'system'}
        class:message-bubble--sending={message.status === 'sending'}
        class:message-bubble--streaming={message.status === 'streaming'}
        class:message-bubble--sent={message.status === 'sent'}
        class:message-bubble--failed={message.status === 'failed'}
        class:message-bubble--queued={message.status === 'queued'}
      >
        {#if message.role === 'assistant' && (!message.content && (!message.interimSteps || message.interimSteps.length === 0) && (message.status === 'sending' || message.status === 'streaming'))}
          <div class="assistant-activity--working" aria-live="polite" style="margin-bottom: 8px;">
            <div class="interim-step-spinner inline-spinner"></div>
            <span>Working...</span>
          </div>
        {/if}

        {#if message.role === 'assistant' && message.interimSteps && message.interimSteps.length > 0}
          <!-- Collapsible activity checklist -->
          <div class="interim-steps-container">
            <button
              type="button"
              class="interim-steps-header"
              on:click={() => toggleSteps(message.id, message.status)}
            >
              <div class="interim-steps-title-row">
                {#if message.status === 'sending' || (!message.content && message.interimSteps.some((s) => s.status === 'thinking' || s.status === 'working'))}
                  <div class="interim-step-spinner inline-spinner"></div>
                  <span>Thinking...</span>
                {:else if message.thinkingDurationMs !== undefined}
                  <span
                    >Thought for {(message.thinkingDurationMs / 1000).toFixed(
                      1
                    )}s</span
                  >
                {:else}
                  <span>Thought process</span>
                {/if}
              </div>
              <span
                class="interim-steps-chevron"
                class:expanded={isExpanded(message.id, message.status)}>›</span
              >
            </button>
            {#if isExpanded(message.id, message.status)}
              <div class="interim-steps-list">
                {#each message.interimSteps as step (step.id)}
                  {@const isReasoning = step.status === 'thinking' || step.id.includes(':reasoning:') || step.id.includes(':thought:') || step.id.includes(':status-thought')}
                  {@const isTool = step.text.startsWith('Calling tool') || step.text.startsWith('Called tool') || step.id.includes(':tool:')}

                  <div class="interim-step-wrapper">
                    {#if isReasoning}
                      <div class="interim-step-item-container">
                        <button
                          type="button"
                          class="interim-step-toggle-btn"
                          on:click={() => toggleStep(step.id)}
                        >
                          <div class="interim-step-status-row">
                            {#if step.status === 'thinking'}
                              <div class="interim-step-spinner"></div>
                              <span class="active-pulse-text">Thinking...</span>
                            {:else}
                              <span class="completed-icon" style="color: var(--success);">✓</span>
                              <span>Thought</span>
                            {/if}
                          </div>
                          <span
                            class="interim-steps-chevron"
                            class:expanded={isStepExpanded(step)}>›</span
                          >
                        </button>
                        {#if isStepExpanded(step)}
                          <div class="interim-step-details reasoning-details">
                            <Markdown content={step.text} />
                          </div>
                        {/if}
                      </div>
                    {:else if isTool}
                      <div class="interim-step-item-container">
                        <button
                          type="button"
                          class="interim-step-toggle-btn"
                          on:click={() => toggleStep(step.id)}
                        >
                          <div class="interim-step-status-row">
                            {#if step.status === 'working'}
                              <div class="interim-step-spinner"></div>
                              <span class="active-pulse-text">{step.text}</span>
                            {:else}
                              <span class="completed-icon" style="color: var(--success);">✓</span>
                              <span>{step.text}</span>
                            {/if}
                          </div>
                          <span
                            class="interim-steps-chevron"
                            class:expanded={isStepExpanded(step)}>›</span
                          >
                        </button>
                        {#if isStepExpanded(step)}
                          <div class="interim-step-details tool-details">
                            {#if step.args}
                              <div class="tool-snippet">
                                <div class="tool-snippet-header">Arguments</div>
                                <pre class="tool-code"><code>{JSON.stringify(step.args, null, 2)}</code></pre>
                              </div>
                            {/if}
                            {#if step.output}
                              <div class="tool-snippet">
                                <div class="tool-snippet-header">Result</div>
                                <pre class="tool-code"><code>{typeof step.output === 'string' ? step.output : JSON.stringify(step.output, null, 2)}</code></pre>
                              </div>
                            {/if}
                          </div>
                        {/if}
                      </div>
                    {:else}
                      <div class="interim-step-item {step.status}">
                        {#if step.status === 'working' || step.status === 'thinking' || step.status === 'running' || step.status === 'pending'}
                          <div class="interim-step-spinner"></div>
                        {:else if step.status === 'completed'}
                          <span style="color: var(--success);">✓</span>
                        {:else if step.status === 'failed'}
                          <span style="color: var(--danger);">✗</span>
                        {:else}
                          <span style="color: var(--muted); opacity: 0.5;">○</span>
                        {/if}
                        <span>{step.text}</span>
                      </div>
                    {/if}
                  </div>
                {/each}

                {#if message.status === 'streaming' || message.status === 'sending' || (state === 'thinking' && messages[messages.length - 1]?.id === message.id)}
                  {#if !message.interimSteps.some((s) => s.status === 'working' || s.status === 'thinking' || s.status === 'running' || s.status === 'pending')}
                    <div class="interim-step-item working" style="margin-top: 4px;">
                      <div class="interim-step-spinner"></div>
                      <span class="active-pulse-text">Working...</span>
                    </div>
                  {/if}
                {/if}
              </div>
            {/if}
          </div>
        {/if}

        {#if message.artifact}
          <!-- Renders artifact card instead of standard text bubble -->
          <div class="artifact-card">
            <div class="artifact-card-header">
              <div class="artifact-card-icon">
                <svg
                  xmlns="http://www.w3.org/2000/svg"
                  width="20"
                  height="20"
                  viewBox="0 0 24 24"
                  fill="none"
                  stroke="currentColor"
                  stroke-width="2"
                  stroke-linecap="round"
                  stroke-linejoin="round"
                  class="lucide lucide-file-text"
                  ><path
                    d="M15 2H6a2 2 0 0 0-2 2v16a2 2 0 0 0 2 2h12a2 2 0 0 0 2-2V7Z"
                  /><path d="M14 2v4a2 2 0 0 0 2 2h4" /><path
                    d="M10 9H8"
                  /><path d="M16 13H8" /><path d="M16 17H8" /></svg
                >
              </div>
              <div class="artifact-card-details">
                <span class="artifact-card-title">{message.artifact.title}</span
                >
                <span class="artifact-card-subtitle"
                  >Task result artifact · {message.artifact.content.length} chars</span
                >
              </div>
            </div>
            <div class="artifact-card-actions">
              <Button
                size="sm"
                variant="outline"
                on:click={() =>
                  message.artifact && onSelectArtifact(message.artifact)}
              >
                View Artifact
              </Button>
            </div>
          </div>
        {:else if message.content}
          <Markdown content={message.content} />
        {/if}

        {#if message.content && !message.artifact}
          <button
            type="button"
            class="message-copy-btn"
            on:click={() => copyToClipboard(message.content, message.id)}
            aria-label="Copy message"
          >
            {#if copiedId === message.id}
              <Check size={13} style="color: var(--success);" />
            {:else}
              <Copy size={13} />
            {/if}
          </button>
        {/if}

        {#if message.status === 'failed'}
          <div class="message-actions">
            <Button
              size="sm"
              variant="outline"
              on:click={() => onRetry(message)}>Retry</Button
            >
          </div>
        {/if}
      </article>
    {/each}

    {#if state === 'thinking' && (messages.length === 0 || messages[messages.length - 1].role === 'user')}
      <article class="message-bubble message-bubble--assistant">
        <div class="assistant-activity--working" aria-live="polite">
          <div class="interim-step-spinner inline-spinner"></div>
          <span>Working...</span>
        </div>
      </article>
    {/if}
  {/if}
</div>
