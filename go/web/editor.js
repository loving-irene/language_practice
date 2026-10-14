export function escapeHTML(value) {
  return String(value).replace(/[&<>"']/g, character => ({ "&": "&amp;", "<": "&lt;", ">": "&gt;", '"': "&quot;", "'": "&#39;" })[character]);
}

export function highlightGo(source) {
  const pattern = /\/\/[^\n]*|\/\*[\s\S]*?\*\/|"(?:\\.|[^"\\])*"|'(?:\\.|[^'\\])*'|`[^`]*`|\b(?:package|import|func|return|var|const|type|struct|interface|map|chan|go|defer|select|case|default|switch|if|else|for|range|break|continue|fallthrough)\b|\b(?:make|len|cap|append|copy|close|delete|panic|recover|new|int|int64|float64|string|bool|error|any|comparable|true|false|nil)\b|\b\d+(?:\.\d+)?\b/g;
  let output = "";
  let offset = 0;
  for (const match of source.matchAll(pattern)) {
    const token = match[0];
    let kind = "keyword";
    if (token.startsWith("//") || token.startsWith("/*")) kind = "comment";
    else if (/^["'`]/.test(token)) kind = "string";
    else if (/^\d/.test(token)) kind = "number";
    else if (/^(make|len|cap|append|copy|close|delete|panic|recover|new|int|int64|float64|string|bool|error|any|comparable|true|false|nil)$/.test(token)) kind = "builtin";
    output += escapeHTML(source.slice(offset, match.index));
    output += `<span class="token-${kind}">${escapeHTML(token)}</span>`;
    offset = match.index + token.length;
  }
  return output + escapeHTML(source.slice(offset)) + "\n";
}

export function editSelection(source, start, end, key, shift = false) {
  const lineStart = source.lastIndexOf("\n", start - 1) + 1;
  const prefix = source.slice(lineStart, start);
  if (key === "Enter") {
    const baseIndent = prefix.match(/^[\t ]*/)[0];
    const opensBlock = prefix.trimEnd().endsWith("{");
    const indentation = baseIndent + (opensBlock ? "\t" : "");
    let insertion = "\n" + indentation;
    const cursor = start + insertion.length;
    if (opensBlock && source.slice(end).startsWith("}")) insertion += "\n" + baseIndent;
    return { source: source.slice(0, start) + insertion + source.slice(end), start: cursor, end: cursor };
  }
  if (!shift && start === end) {
    return { source: source.slice(0, start) + "\t" + source.slice(end), start: start + 1, end: end + 1 };
  }
  const blockEnd = end > start && source[end - 1] === "\n" ? end - 1 : end;
  const lines = source.slice(lineStart, blockEnd).split("\n");
  let firstChange = 0;
  let totalChange = 0;
  const updated = lines.map((line, index) => {
    const removed = shift ? line.match(/^(\t| {1,4})/)?.[0].length || 0 : 0;
    const change = shift ? -removed : 1;
    if (index === 0) firstChange = change;
    totalChange += change;
    return shift ? line.slice(removed) : "\t" + line;
  }).join("\n");
  return {
    source: source.slice(0, lineStart) + updated + source.slice(blockEnd),
    start: Math.max(lineStart, start + firstChange),
    end: Math.max(lineStart, end + totalChange),
  };
}

export function matchesExpected(result, expected, mode) {
  if (!result.success || result.truncated) return false;
  if (mode === "test") return /(?:^|\n)PASS(?:\r?\n|$)/.test(result.output) && /--- PASS: Test/.test(result.output);
  return result.output.replace(/\r\n/g, "\n").trim() === expected.replace(/\r\n/g, "\n").trim();
}
