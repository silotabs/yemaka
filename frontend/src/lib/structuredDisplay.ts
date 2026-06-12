export type StructuredDataLanguage = 'auto' | 'json' | 'yaml' | 'text';

export type StructuredTokenKind =
  | 'plain'
  | 'key'
  | 'string'
  | 'number'
  | 'boolean'
  | 'null'
  | 'punct'
  | 'comment';

export type StructuredToken = {
  text: string;
  kind: StructuredTokenKind;
  title?: string;
};

export type StructuredLine = {
  parts: StructuredToken[];
};

export type StructuredData = {
  language: Exclude<StructuredDataLanguage, 'auto'>;
  rawText: string;
  formattedText: string;
  lines: StructuredLine[];
  empty: boolean;
};

const structuredAcronyms = new Set([
  'ai',
  'api',
  'cli',
  'cpu',
  'docx',
  'fts',
  'gpu',
  'html',
  'http',
  'https',
  'id',
  'json',
  'llm',
  'pdf',
  'rag',
  'sql',
  'sqlite',
  'svg',
  'tui',
  'ui',
  'url',
  'yaml'
]);

function normalizeText(value: unknown) {
  if (value === null || value === undefined) return '';
  if (typeof value === 'string') return value;
  try {
    return JSON.stringify(value, null, 2);
  } catch {
    return String(value);
  }
}

function prettyJson(value: unknown, rawText: string) {
  if (typeof value !== 'string') return JSON.stringify(value, null, 2);
  const trimmed = rawText.trim();
  if (!trimmed) return '';
  return JSON.stringify(JSON.parse(trimmed), null, 2);
}

function looksLikeJson(text: string) {
  const trimmed = text.trim();
  return (
    (trimmed.startsWith('{') && trimmed.endsWith('}')) ||
    (trimmed.startsWith('[') && trimmed.endsWith(']'))
  );
}

function looksLikeYaml(text: string, filename = '') {
  const lowerName = filename.toLowerCase();
  if (lowerName.endsWith('.yaml') || lowerName.endsWith('.yml')) return true;
  const lines = text.split(/\r?\n/).filter((line) => line.trim() && !line.trim().startsWith('#'));
  if (lines.length < 2) return false;
  const yamlLike = lines.filter((line) => /^\s*(?:-\s*)?[A-Za-z0-9_.-]+\s*:\s*.+/.test(line));
  return yamlLike.length >= Math.min(3, lines.length);
}

export function humanizeStructuredKey(value: string) {
  const spaced = String(value || '')
    .replace(/([a-z0-9])([A-Z])/g, '$1 $2')
    .replace(/([A-Z]+)([A-Z][a-z])/g, '$1 $2')
    .replace(/[_.-]+/g, ' ')
    .trim();
  const words = spaced.split(/\s+/).filter(Boolean);
  if (!words.length) return value;
  return words
    .map((word) => {
      const lower = word.toLowerCase();
      if (structuredAcronyms.has(lower)) return lower.toUpperCase();
      return lower.charAt(0).toUpperCase() + lower.slice(1);
    })
    .join(' ');
}

function detectLanguage(value: unknown, rawText: string, requested: StructuredDataLanguage, filename = '') {
  if (requested !== 'auto') return requested;
  if (typeof value !== 'string') return 'json';
  if (looksLikeJson(rawText)) return 'json';
  if (looksLikeYaml(rawText, filename)) return 'yaml';
  return 'text';
}

function tokenValue(value: string): StructuredToken[] {
  const parts: StructuredToken[] = [];
  const pattern = /"(?:\\.|[^"\\])*"|-?\d+(?:\.\d+)?(?:e[+-]?\d+)?|true|false|null|[{}\[\],]/gi;
  let cursor = 0;
  for (const match of value.matchAll(pattern)) {
    const index = match.index ?? 0;
    if (index > cursor) parts.push({ text: value.slice(cursor, index), kind: 'plain' });
    const token = match[0];
    if (token.startsWith('"')) parts.push({ text: token, kind: 'string' });
    else if (/^(true|false)$/i.test(token)) parts.push({ text: token, kind: 'boolean' });
    else if (/^null$/i.test(token)) parts.push({ text: token, kind: 'null' });
    else if (/^-?\d/.test(token)) parts.push({ text: token, kind: 'number' });
    else parts.push({ text: token, kind: 'punct' });
    cursor = index + token.length;
  }
  if (cursor < value.length) parts.push({ text: value.slice(cursor), kind: 'plain' });
  return parts.length ? parts : [{ text: value, kind: 'plain' }];
}

function jsonLineParts(line: string): StructuredToken[] {
  const match = /^(\s*)"((?:\\.|[^"\\])*)":(.*)$/.exec(line);
  if (!match) return tokenValue(line);
  const [, indent, rawKey, rest] = match;
  const key = rawKey.replace(/\\"/g, '"');
  return [
    { text: indent, kind: 'plain' },
    { text: humanizeStructuredKey(key), kind: 'key', title: key },
    { text: ':', kind: 'punct' },
    ...tokenValue(rest)
  ];
}

function yamlLineParts(line: string): StructuredToken[] {
  const comment = /^(\s*#.*)$/.exec(line);
  if (comment) return [{ text: line, kind: 'comment' }];
  const match = /^(\s*)(-\s*)?([A-Za-z0-9_.-]+)(\s*:\s*)(.*)$/.exec(line);
  if (!match) return tokenValue(line);
  const [, indent, bullet = '', rawKey, separator, rest] = match;
  return [
    { text: indent, kind: 'plain' },
    { text: bullet, kind: 'punct' },
    { text: humanizeStructuredKey(rawKey), kind: 'key', title: rawKey },
    { text: separator.trimEnd(), kind: 'punct' },
    ...tokenValue(separator.endsWith(' ') ? ` ${rest}` : rest)
  ];
}

function tokenizeLines(text: string, language: Exclude<StructuredDataLanguage, 'auto'>): StructuredLine[] {
  const lines = text.split(/\r?\n/);
  return lines.map((line) => ({
    parts:
      language === 'json'
        ? jsonLineParts(line)
        : language === 'yaml'
          ? yamlLineParts(line)
          : tokenValue(line)
  }));
}

export function formatStructuredData(
  value: unknown,
  language: StructuredDataLanguage = 'auto',
  filename = ''
): StructuredData {
  const rawText = normalizeText(value);
  const detected = detectLanguage(value, rawText, language, filename);
  let formattedText = rawText;
  let finalLanguage = detected;
  if (detected === 'json') {
    try {
      formattedText = prettyJson(value, rawText);
    } catch {
      finalLanguage = 'text';
      formattedText = rawText;
    }
  }
  return {
    language: finalLanguage,
    rawText,
    formattedText,
    lines: tokenizeLines(formattedText, finalLanguage),
    empty: rawText.trim().length === 0
  };
}
