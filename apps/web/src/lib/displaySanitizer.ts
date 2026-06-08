import { type Part as A2APart } from '@a2a-js/sdk';

type PartWithData = A2APart & {
  data?: {
    id?: string;
    name?: string;
    response?: {
      output?: unknown;
    };
  };
};

const hiddenReasoningBlocks = [
  /<think>[\s\S]*?<\/think>/gi,
  /<thinking>[\s\S]*?<\/thinking>/gi,
  /<reasoning>[\s\S]*?<\/reasoning>/gi,
  /<scratchpad>[\s\S]*?<\/scratchpad>/gi,
  /```(?:thinking|reasoning|scratchpad)\s+[\s\S]*?```/gi,
  /<tool_call>[\s\S]*?<\/tool_call>/gi,
  /<tool_response>[\s\S]*?<\/tool_response>/gi
];

function looksLikeReasoningParagraph(paragraph: string): boolean {
  const lower = paragraph.trim().toLowerCase();
  if (
    lower.startsWith('okay, the user') ||
    lower.startsWith('the user ') ||
    lower.startsWith('we need ') ||
    lower.startsWith('i need ') ||
    lower.startsWith('i should ') ||
    lower.startsWith('let me ')
  ) {
    return true;
  }
  const phrases = [
    'the user',
    'i should',
    "i'll",
    'i will',
    'no need to',
    'need to call',
    'call any function',
    'call tools',
    'use the tool',
    'tool choice',
    'final answer'
  ];
  let signals = 0;
  for (const phrase of phrases) {
    if (lower.includes(phrase)) {
      signals++;
    }
  }
  return signals >= 2;
}

/**
 * Detects ADK-style tool invocations that models emit as plain text instead
 * of structured function calls. Covers patterns like:
 *   "jute_skill_read_context - {\"skillId\": ...}"
 *   "function_name - {\"key\": ...}"
 *   Bare JSON tool-response payloads with known keys.
 */
export function looksLikeToolInvocation(text: string): boolean {
  const trimmed = text.trim();
  if (!trimmed) return false;

  // ADK-style: "function_name - {json_args}"
  if (/^[a-zA-Z_]\w*\s*-\s*\{/.test(trimmed)) {
    return true;
  }
  // Known hub tool-function prefixes emitted as plain text
  if (
    trimmed.startsWith('jute_skill_') ||
    trimmed.startsWith('mcp_') ||
    trimmed.startsWith('function_call')
  ) {
    return true;
  }
  // Bare JSON that looks like a tool response payload
  if (
    /^\{[\s\S]*"(?:skillId|tool_call_id|function_call|actionId)"/.test(trimmed)
  ) {
    return true;
  }
  return false;
}

export function isReasoningArtifact(artifact?: {
  artifactId?: string;
  name?: string;
  description?: string;
  parts?: A2APart[];
}): boolean {
  if (!artifact) return false;
  const idLower = (artifact.artifactId || '').toLowerCase();
  const nameLower = (artifact.name || '').toLowerCase();
  const descLower = (artifact.description || '').toLowerCase();

  const keywords = [
    'reasoning',
    'thinking',
    'scratchpad',
    'thought',
    'internal-thought',
    'internal_thought',
    'chain-of-thought',
    'chain_of_thought',
    'cot',
    'planning',
    'plan',
    'tool-selection',
    'tool_selection'
  ];

  if (
    keywords.some(
      (k) =>
        idLower.includes(k) || nameLower.includes(k) || descLower.includes(k)
    )
  ) {
    return true;
  }

  const artRecord = artifact as Record<string, unknown>;
  const metadata = artRecord.metadata as Record<string, unknown> | undefined;
  const typeLower = (
    String(artRecord.type || '') ||
    String(artRecord.kind || '') ||
    String(metadata?.adk_type || '') ||
    String(metadata?.type || '') ||
    String(metadata?.kind || '') ||
    ''
  ).toLowerCase();
  if (keywords.some((k) => typeLower.includes(k))) {
    return true;
  }

  if (artifact.parts && Array.isArray(artifact.parts)) {
    // If any part of the artifact is a function call, function response, or tool invocation,
    // the entire artifact is classified as a reasoning/internal steps artifact.
    const hasToolOrFunction = artifact.parts.some((part) => {
      if (
        part.metadata?.adk_type === 'function_call' ||
        part.metadata?.adk_type === 'function_response'
      ) {
        return true;
      }
      const data = getPartData(part) as PartWithData['data'];
      if (data && (data.name || data.id || data.response)) {
        return true;
      }
      const text = getPartText(part);
      if (text) {
        const trimmed = text.trim();
        if (
          trimmed.startsWith('<tool_call>') ||
          trimmed.endsWith('</tool_call>') ||
          trimmed.startsWith('<tool_response>') ||
          trimmed.endsWith('</tool_response>') ||
          trimmed.includes('<tool_call>') ||
          trimmed.includes('<tool_response>') ||
          looksLikeToolInvocation(trimmed)
        ) {
          return true;
        }
      }
      return false;
    });

    if (hasToolOrFunction) {
      return true;
    }

    const hasOnlyThoughtsAndTools = artifact.parts.every((part) => {
      let matched = false;
      if (part.metadata?.adk_thought === true) matched = true;
      else if (
        part.metadata?.adk_type === 'function_call' ||
        part.metadata?.adk_type === 'function_response'
      )
        matched = true;
      else {
        const mediaType = (part.mediaType || '').toLowerCase();
        if (keywords.some((k) => mediaType.includes(k))) matched = true;
        else {
          const text = getPartText(part);
          if (!text) {
            matched = true;
          } else {
            const trimmed = text.trim();
            if (!trimmed) matched = true;
            else if (
              trimmed.startsWith('<tool_call>') ||
              trimmed.endsWith('</tool_call>') ||
              trimmed.startsWith('<tool_response>') ||
              trimmed.endsWith('</tool_response>') ||
              trimmed.includes('<tool_call>') ||
              trimmed.includes('<tool_response>') ||
              looksLikeToolInvocation(trimmed) ||
              looksLikeReasoningParagraph(trimmed)
            )
              matched = true;
          }
        }
      }
      return matched;
    });

    if (artifact.parts.length > 0 && hasOnlyThoughtsAndTools) {
      return true;
    }
  }

  return false;
}

export function sanitizeDisplayText(text: string): string {
  let cleaned = text.trim();
  if (!cleaned) return '';

  // Handle active streaming / open tags defensively
  const openTags = [
    { start: '<think>', end: '</think>' },
    { start: '<thinking>', end: '</thinking>' },
    { start: '<reasoning>', end: '</reasoning>' },
    { start: '<scratchpad>', end: '</scratchpad>' },
    { start: '<tool_call>', end: '</tool_call>' },
    { start: '<tool_response>', end: '</tool_response>' }
  ];

  for (const tag of openTags) {
    const startIdx = cleaned.indexOf(tag.start);
    if (startIdx > -1) {
      const endIdx = cleaned.indexOf(tag.end);
      if (endIdx > -1) {
        cleaned =
          cleaned.slice(0, startIdx) + cleaned.slice(endIdx + tag.end.length);
      } else {
        cleaned = cleaned.slice(0, startIdx);
      }
    }
  }

  for (const pattern of hiddenReasoningBlocks) {
    cleaned = cleaned.replace(pattern, '');
  }
  cleaned = cleaned.trim();
  if (!cleaned) return '';

  const paragraphs = cleaned
    .replace(/\r\n/g, '\n')
    .split('\n\n')
    .map((p) => p.trim())
    .filter(Boolean);

  while (paragraphs.length > 1 && looksLikeReasoningParagraph(paragraphs[0])) {
    paragraphs.shift();
  }

  return paragraphs.join('\n\n').trim();
}

export function getPartText(part: A2APart): string {
  if ('text' in part && (part as { text?: string }).text) {
    return (part as { text?: string }).text as string;
  }
  return part.content?.$case === 'text' ? part.content.value : '';
}

export function getPartData(
  part: A2APart
): Record<string, unknown> | undefined {
  const p = part as unknown as {
    data?: Record<string, unknown>;
    content?: {
      $case: string;
      value?: Record<string, unknown>;
    };
  };
  if (p.data) {
    return p.data;
  }
  if (p.content?.$case === 'data') {
    return p.content.value;
  }
  return undefined;
}

export function textFromParts(parts: A2APart[] | undefined): string {
  const raw = (parts ?? [])
    .map((part) => {
      if (part.metadata?.adk_thought === true) {
        return '';
      }
      const text = (
        'text' in part && (part as { text?: string }).text
          ? (part as { text?: string }).text
          : part.content?.$case === 'text'
            ? part.content.value
            : ''
      ) as string;
      // Filter out parts that are raw tool invocations
      if (text && looksLikeToolInvocation(text)) {
        return '';
      }
      return text;
    })
    .join('');
  return sanitizeDisplayText(raw);
}

export function isStructuredArtifact(parts: A2APart[] | undefined): boolean {
  return (parts ?? [])
    .filter((part) => {
      if (part.metadata?.adk_thought === true) return false;
      if (
        part.metadata?.adk_type === 'function_call' ||
        part.metadata?.adk_type === 'function_response'
      ) {
        return false;
      }
      return true;
    })
    .some((part) => {
      const isLegacyText = 'text' in part && (part as { text?: string }).text;
      const isCaseText = part.content?.$case === 'text';
      if (!isLegacyText && !isCaseText) {
        return true; // Not a text part -> structured
      }
      if (part.mediaType) {
        const mt = part.mediaType.toLowerCase();
        if (mt !== 'text/plain' && mt !== 'text/markdown' && mt !== '') {
          return true;
        }
      }
      return false;
    });
}

export function textFromReasoningParts(parts: A2APart[] | undefined): string {
  return (parts ?? [])
    .map((part) => {
      if ('text' in part && (part as { text?: string }).text) {
        return (part as { text?: string }).text;
      }
      return part.content?.$case === 'text' ? part.content.value : '';
    })
    .join('');
}
