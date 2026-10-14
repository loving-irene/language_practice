import test from "node:test";
import assert from "node:assert/strict";
import { editSelection, escapeHTML, highlightGo, matchesExpected } from "../web/editor.js";

test("source and terminal-like HTML never become executable markup", () => {
  const source = 'fmt.Println("<img src=x onerror=alert(1)>")';
  const highlighted = highlightGo(source);
  assert.ok(!highlighted.includes("<img"));
  assert.ok(highlighted.includes("&lt;img"));
  assert.equal(escapeHTML('<script>"&\'</script>'), "&lt;script&gt;&quot;&amp;&#39;&lt;/script&gt;");
});

test("comments containing keywords are one token", () => {
  assert.equal(highlightGo("// func return 42"), '<span class="token-comment">// func return 42</span>\n');
});

test("enter between braces creates an indented blank line", () => {
  const source = "func main() {}";
  const position = source.indexOf("}");
  const edited = editSelection(source, position, position, "Enter");
  assert.equal(edited.source, "func main() {\n\t\n}");
  assert.equal(edited.source.slice(0, edited.start), "func main() {\n\t");
  assert.equal(edited.start, edited.end);
});

test("tab replaces a caret with a tab and supports selected lines", () => {
  assert.deepEqual(editSelection("hello", 0, 0, "Tab"), { source: "\thello", start: 1, end: 1 });
  const edited = editSelection("one\ntwo\nthree", 0, 8, "Tab");
  assert.equal(edited.source, "\tone\n\ttwo\nthree");
  assert.equal(edited.end, 10);
});

test("shift-tab unindents tabs or up to four spaces", () => {
  const edited = editSelection("\tone\n    two", 0, 12, "Tab", true);
  assert.equal(edited.source, "one\ntwo");
  assert.equal(edited.start, 0);
  assert.equal(edited.end, 7);
  assert.deepEqual(editSelection("hello", 2, 2, "Tab", true), { source: "hello", start: 2, end: 2 });
});

test("output matching normalizes newlines but requires successful complete output", () => {
  assert.equal(matchesExpected({ success: true, output: "hello\r\nGo\r\n" }, "hello\nGo", "run"), true);
  assert.equal(matchesExpected({ success: false, output: "42" }, "42", "run"), false);
  assert.equal(matchesExpected({ success: true, output: "42", truncated: true }, "42", "run"), false);
  assert.equal(matchesExpected({ success: true, output: "43" }, "42", "run"), false);
});

test("test mode requires real named passing tests, not just the word PASS", () => {
  assert.equal(matchesExpected({ success: true, output: "PASS\n" }, "PASS", "test"), false);
  assert.equal(matchesExpected({ success: true, output: "--- PASS: TestAdd (0.00s)\nPASS\n" }, "PASS", "test"), true);
});
