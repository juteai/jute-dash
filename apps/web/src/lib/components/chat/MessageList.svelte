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
        {#if message.role === 'assistant' && !message.content && (!message.interimSteps || message.interimSteps.length === 0) && (message.status === 'sending' || message.status === 'streaming')}
          <div
            class="assistant-activity--working"
            aria-live="polite"
            style="margin-bottom: 8px;"
          >
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
                  {@const isReasoning =
                    step.status === 'thinking' ||
                    step.id.includes(':reasoning:') ||
                    step.id.includes(':thought:') ||
                    step.id.includes(':status-thought')}
                  {@const isTool =
                    step.text.startsWith('Calling tool') ||
                    step.text.startsWith('Called tool') ||
                    step.id.includes(':tool:')}

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
                              <span
                                class="completed-icon"
                                style="color: var(--success);">✓</span
                              >
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
                              <span
                                class="completed-icon"
                                style="color: var(--success);">✓</span
                              >
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
                                <pre class="tool-code"><code
                                    >{JSON.stringify(step.args, null, 2)}</code
                                  ></pre>
                              </div>
                            {/if}
                            {#if step.output}
                              <div class="tool-snippet">
                                <div class="tool-snippet-header">Result</div>
                                <pre class="tool-code"><code
                                    >{typeof step.output === 'string'
                                      ? step.output
                                      : JSON.stringify(
                                          step.output,
                                          null,
                                          2
                                        )}</code
                                  ></pre>
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
                          <span style="color: var(--muted); opacity: 0.5;"
                            >○</span
                          >
                        {/if}
                        <span>{step.text}</span>
                      </div>
                    {/if}
                  </div>
                {/each}

                {#if message.status === 'streaming' || message.status === 'sending' || (state === 'thinking' && messages[messages.length - 1]?.id === message.id)}
                  {#if !message.interimSteps.some((s) => s.status === 'working' || s.status === 'thinking' || s.status === 'running' || s.status === 'pending')}
                    <div
                      class="interim-step-item working"
                      style="margin-top: 4px;"
                    >
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

<style>
  .message-list {
    display: flex;
    flex-direction: column;
    gap: 12px;
    min-height: 0;
    overflow-y: auto;
    padding: 16px;
    scrollbar-gutter: stable;
  }

  .chat-empty {
    display: grid;
    min-height: 100%;
    place-content: center;
    color: var(--muted);
    text-align: center;
  }

  .chat-empty-title {
    color: var(--foreground);
    font-size: 1.45rem;
    font-weight: 780;
  }

  .message-bubble {
    width: fit-content;
    max-width: 75%;
    padding: 10px 32px 10px 16px;
    border-radius: 18px;
    position: relative;
    transition:
      background-color 0.2s ease,
      border-color 0.2s ease;
  }

  .message-bubble--user {
    align-self: flex-end;
    background: var(--foreground);
    color: var(--inverse);
    border: none;
    border-bottom-right-radius: 4px;
  }

  .message-bubble--assistant {
    align-self: flex-start;
    background: transparent;
    border: none;
    padding: 10px 32px 10px 0;
    max-width: 80%;
    border-radius: 0;
    box-shadow: none;
  }

  .message-bubble--system {
    align-self: flex-start;
    background: transparent;
    border: none;
    border-left: 2px solid var(--warning);
    padding: 10px 32px 10px 12px;
    max-width: 80%;
    border-radius: 0;
    box-shadow: none;
  }

  .message-bubble--failed {
    border-left: 2px solid var(--danger);
    padding-left: 12px;
  }

  .message-bubble--sending,
  .message-bubble--queued {
    opacity: 0.62;
  }

  .message-bubble--queued {
    border-style: dashed;
  }

  .message-copy-btn {
    position: absolute;
    bottom: 8px;
    right: 8px;
    width: 24px;
    height: 24px;
    display: flex;
    align-items: center;
    justify-content: center;
    border-radius: 50%;
    border: none;
    background: color-mix(in srgb, var(--surface-strong) 15%, transparent);
    color: var(--muted);
    cursor: pointer;
    opacity: 0;
    transition:
      opacity 0.25s ease,
      background-color 0.2s ease,
      color 0.2s ease;
  }

  .message-copy-btn:hover {
    background: color-mix(in srgb, var(--surface-strong) 30%, transparent);
    color: var(--foreground);
  }

  .message-bubble:hover .message-copy-btn {
    opacity: 1;
  }

  @media (hover: none) {
    .message-copy-btn {
      opacity: 0.35;
    }
  }

  .message-actions {
    display: flex;
    margin-top: 8px;
  }

  .assistant-activity--working {
    display: inline-flex;
    align-items: center;
    gap: 8px;
    color: var(--active);
    font-size: 0.88rem;
    font-weight: 500;
  }

  /* Redesigned Collapsible progress logs */
  .interim-steps-container {
    margin-bottom: 12px;
    font-size: 0.82rem;
    width: 100%;
    overflow: hidden;
  }

  .interim-steps-header {
    display: inline-flex;
    align-items: center;
    justify-content: flex-start;
    gap: 8px;
    padding: 4px 0;
    border: none;
    background: transparent;
    cursor: pointer;
    user-select: none;
    color: var(--muted-strong);
    font-weight: 600;
    transition: color 0.2s ease;
  }

  .interim-steps-header:hover {
    color: var(--foreground);
  }

  .interim-steps-title-row {
    display: flex;
    align-items: center;
    gap: 8px;
  }

  .interim-steps-chevron {
    font-size: 0.85rem;
    transition: transform 0.2s ease;
    display: inline-block;
    color: var(--muted);
  }

  .interim-steps-chevron.expanded {
    transform: rotate(90deg);
  }

  .interim-steps-list {
    border-left: 2px solid var(--border);
    margin-left: 6px;
    padding: 6px 0 8px 16px;
    display: flex;
    flex-direction: column;
    gap: 8px;
  }

  .interim-step-item {
    display: flex;
    align-items: center;
    gap: 8px;
    color: var(--muted);
    font-size: 0.8rem;
  }

  .interim-step-spinner {
    width: 11px;
    height: 11px;
    border: 2px solid color-mix(in srgb, var(--active) 20%, transparent);
    border-top-color: var(--active);
    border-radius: 50%;
    animation: step-spin 0.8s linear infinite;
    flex-shrink: 0;
  }

  .inline-spinner {
    border-top-color: var(--active);
  }

  @keyframes step-spin {
    to {
      transform: rotate(360deg);
    }
  }

  .interim-step-wrapper {
    margin-top: 4px;
  }

  .interim-step-item-container {
    display: flex;
    flex-direction: column;
    border-radius: 12px;
    background: color-mix(in srgb, var(--surface-strong) 12%, transparent);
    border: 1px solid color-mix(in srgb, var(--border) 45%, transparent);
    transition: all 0.2s cubic-bezier(0.16, 1, 0.3, 1);
    overflow: hidden;
    text-align: left;
    width: fit-content;
    min-width: 280px;
    max-width: 100%;
  }

  .interim-step-item-container:hover {
    background: color-mix(in srgb, var(--surface-strong) 20%, transparent);
    border-color: var(--border);
  }

  .interim-step-toggle-btn {
    display: flex;
    align-items: center;
    justify-content: space-between;
    width: 100%;
    padding: 6px 12px;
    border: none;
    background: transparent;
    cursor: pointer;
    text-align: left;
    color: var(--foreground);
    font-size: 0.78rem;
    font-weight: 550;
    transition: color 0.2s ease;
  }

  .interim-step-status-row {
    display: flex;
    align-items: center;
    gap: 8px;
  }

  .completed-icon {
    font-size: 0.85rem;
    display: inline-flex;
    align-items: center;
    justify-content: center;
  }

  .active-pulse-text {
    color: var(--active);
    animation: active-text-pulse 2s ease-in-out infinite;
  }

  @keyframes active-text-pulse {
    0%,
    100% {
      opacity: 0.75;
    }
    50% {
      opacity: 1;
    }
  }

  .interim-step-details {
    border-top: 1px solid color-mix(in srgb, var(--border) 35%, transparent);
    padding: 10px 12px;
    font-size: 0.78rem;
    color: var(--muted-strong);
    line-height: 1.45;
    background: color-mix(in srgb, var(--surface) 60%, transparent);
  }

  .reasoning-details {
    font-style: italic;
    max-height: 240px;
    overflow-y: auto;
    border-left: 2px solid var(--active);
  }

  .tool-details {
    display: flex;
    flex-direction: column;
    gap: 8px;
  }

  .tool-snippet {
    display: flex;
    flex-direction: column;
    gap: 4px;
  }

  .tool-snippet-header {
    font-size: 0.7rem;
    font-weight: 700;
    text-transform: uppercase;
    letter-spacing: 0.05em;
    color: var(--muted);
  }

  .tool-code {
    margin: 0 !important;
    padding: 8px !important;
    font-family:
      ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, monospace !important;
    font-size: 0.72rem !important;
    border-radius: 6px !important;
    background: color-mix(
      in srgb,
      var(--surface-strong) 40%,
      var(--surface)
    ) !important;
    border: 1px solid color-mix(in srgb, var(--border) 25%, transparent) !important;
    overflow-x: auto;
    color: var(--foreground) !important;
    max-height: 160px;
  }

  /* Artifact Card */
  .artifact-card {
    display: flex;
    flex-direction: column;
    gap: 8px;
    margin-top: 8px;
    border: 1px solid var(--border-strong);
    border-radius: 12px;
    background: color-mix(in srgb, var(--surface-strong) 40%, var(--surface));
    padding: 12px;
    box-shadow: 0 4px 12px var(--shadow);
    max-width: 520px;
    text-align: left;
  }

  .artifact-card-header {
    display: flex;
    align-items: center;
    gap: 10px;
  }

  .artifact-card-icon {
    display: flex;
    align-items: center;
    justify-content: center;
    width: 36px;
    height: 36px;
    border-radius: 8px;
    background: color-mix(in srgb, var(--active) 12%, transparent);
    color: var(--active);
    flex-shrink: 0;
  }

  .artifact-card-details {
    display: flex;
    flex-direction: column;
    min-width: 0;
    flex: 1;
  }

  .artifact-card-title {
    font-weight: 760;
    color: var(--foreground);
    font-size: 0.86rem;
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }

  .artifact-card-subtitle {
    font-size: 0.72rem;
    color: var(--muted);
  }

  .artifact-card-actions {
    display: flex;
    justify-content: flex-end;
    gap: 8px;
    margin-top: 4px;
  }

  @media (max-width: 640px) {
    .message-bubble {
      width: 100%;
      max-width: 100%;
    }
  }
</style>
