import { escapeHTML, highlightGo, editSelection, matchesExpected } from "./editor.js";

const element = id => document.getElementById(id);
const storageKey = "language-practice.go.v1";
const editor = element("code-editor");
let storageWarning = "";
let toastTimer;
let saveTimer;
let activeRequest;
let escapeTab = false;
let lessons = [];
let config = null;
let busy = false;
let exercise = "example";
let lastResult = null;
const state = restoreState();

function restoreState() {
  const empty = { version: 1, lesson: "hello", view: "learn", drafts: {}, completed: [], history: [] };
  try {
    const raw = localStorage.getItem(storageKey);
    if (!raw) return empty;
    const saved = JSON.parse(raw);
    if (saved?.version !== 1 || !saved.drafts || typeof saved.drafts !== "object" || Array.isArray(saved.drafts)) throw new Error("无法识别保存的数据格式");
    return {
      ...empty,
      lesson: typeof saved.lesson === "string" ? saved.lesson : "hello",
      view: saved.view === "lab" ? "lab" : "learn",
      drafts: saved.drafts,
      completed: Array.isArray(saved.completed) ? saved.completed.filter(item => typeof item === "string") : [],
      history: Array.isArray(saved.history) ? saved.history.filter(item => typeof item?.title === "string" && typeof item?.output === "string" && typeof item?.time === "number").slice(0, 8) : [],
    };
  } catch (error) {
    storageWarning = `无法恢复本地记录：${error.message}。本次仍可继续学习。`;
    return empty;
  }
}

function persist() {
  try {
    localStorage.setItem(storageKey, JSON.stringify(state));
    element("save-status").textContent = "草稿已保存在本机";
  } catch (error) {
    element("save-status").textContent = "草稿未保存";
    if (!storageWarning) {
      storageWarning = `浏览器未能保存草稿：${error.message}。离开前请自行复制代码。`;
      toast(storageWarning);
    }
  }
}

function currentLesson() {
  return lessons.find(lesson => lesson.id === state.lesson) || lessons[0];
}

function draftKey() {
  return `${state.lesson}:${exercise}`;
}

function saveDraft(immediate = false) {
  if (!lessons.length) return;
  state.drafts[draftKey()] = { code: editor.value, input: element("stdin").value, mode: element("execution-mode").value };
  clearTimeout(saveTimer);
  if (immediate) persist();
  else {
    element("save-status").textContent = "正在保存…";
    saveTimer = setTimeout(persist, 250);
  }
}

function toast(message) {
  clearTimeout(toastTimer);
  element("toast").textContent = message;
  element("toast").hidden = false;
  toastTimer = setTimeout(() => { element("toast").hidden = true; }, 4200);
}

function renderCourseList() {
  const query = element("course-search").value.trim().toLowerCase();
  const groups = [...new Set(lessons.map(lesson => lesson.group))];
  let count = 0;
  element("course-list").innerHTML = groups.map((group, groupIndex) => {
    const filtered = lessons.filter(lesson => lesson.group === group && `${lesson.title} ${lesson.english} ${lesson.tags.join(" ")}`.toLowerCase().includes(query));
    count += filtered.length;
    if (!filtered.length) return "";
    return `<section class="course-group"><h3 class="group-title"><span>${String(groupIndex + 1).padStart(2, "0")}</span>${escapeHTML(group)}</h3>${filtered.map(lesson => {
      const active = state.lesson === lesson.id;
      const completed = state.completed.includes(lesson.id);
      const index = lessons.indexOf(lesson) + 1;
      return `<button class="course-button ${active ? "active" : ""} ${completed ? "completed" : ""}" data-lesson="${lesson.id}" ${active ? 'aria-current="step"' : ""} ${busy ? "disabled" : ""}><span class="course-number">${String(index).padStart(2, "0")}</span><span>${escapeHTML(lesson.title)}</span><span class="course-indicator" aria-label="${completed ? "已掌握" : "未完成"}">✓</span></button>`;
    }).join("")}</section>`;
  }).join("");
  if (!count) element("course-list").innerHTML = '<p class="search-empty">没有找到匹配的概念。试试「接口」或「channel」。</p>';
}

function renderProgress() {
  const completed = new Set(state.completed.filter(id => lessons.some(lesson => lesson.id === id))).size;
  element("progress-text").innerHTML = `学习进度 <strong>${completed} / ${lessons.length}</strong>`;
  element("progress-bar").max = lessons.length;
  element("progress-bar").value = completed;
  const known = state.completed.includes(state.lesson);
  element("complete-button").classList.toggle("completed", known);
  element("complete-button").innerHTML = known ? "<span>✓</span> 已掌握 · 撤销" : "<span>✓</span> 标记为已掌握";
  element("complete-button").setAttribute("aria-pressed", String(known));
}

function renderLesson() {
  const lesson = currentLesson();
  const index = lessons.indexOf(lesson);
  element("breadcrumb-group").textContent = lesson.group;
  element("lesson-number").textContent = `${String(index + 1).padStart(2, "0")} / ${lessons.length}`;
  element("lesson-duration").textContent = `约 ${lesson.minutes} 分钟`;
  element("lesson-english").textContent = lesson.english;
  element("lesson-title").textContent = lesson.title;
  element("lesson-summary").textContent = lesson.summary;
  element("lesson-tags").innerHTML = lesson.tags.map(tag => `<span>${escapeHTML(tag)}</span>`).join("");
  element("lesson-sections").innerHTML = lesson.sections.map((section, sectionIndex) => `<section class="lesson-section"><h3><span class="section-index">${String(sectionIndex + 1).padStart(2, "0")}</span>${escapeHTML(section.title)}</h3><p>${escapeHTML(section.text)}</p><pre><code>${highlightGo(section.code)}</code></pre></section>`).join("");
  element("lesson-tip").textContent = lesson.tip;
  element("challenge-description").textContent = lesson.challenge.description;
  element("challenge-hint").textContent = lesson.challenge.hint;
  element("challenge-hint").hidden = true;
  element("hint-button").textContent = "给我一点提示";
  element("expected-output").textContent = lesson.challenge.expected;
  element("next-button").disabled = index === lessons.length - 1 || busy;
  element("next-button").innerHTML = index === lessons.length - 1 ? "已是最后一课" : "下一课 <span>→</span>";
  document.title = `${lesson.title} · Go 语言工作台 · 码间`;
  renderCourseList();
  renderProgress();
  renderView();
  loadDraft();
}

function renderView() {
  document.body.classList.toggle("lab-view", state.view === "lab");
  document.querySelectorAll("[data-view]").forEach(button => {
    const active = button.dataset.view === state.view;
    button.classList.toggle("active", active);
    button.setAttribute("aria-pressed", String(active));
  });
  element("practice-title").textContent = state.view === "lab" ? currentLesson().title : "边学，边运行。";
}

function loadDraft() {
  const lesson = currentLesson();
  const saved = state.drafts[draftKey()];
  const valid = saved && typeof saved.code === "string" && typeof saved.input === "string" && ["run", "test"].includes(saved.mode);
  editor.value = valid ? saved.code : exercise === "example" ? lesson.example : lesson.challenge.code;
  element("stdin").value = valid ? saved.input : lesson.input || "";
  element("execution-mode").value = valid ? saved.mode : lesson.mode;
  editor.scrollTop = 0;
  editor.scrollLeft = 0;
  editor.setSelectionRange(0, 0);
  element("example-button").classList.toggle("active", exercise === "example");
  element("example-button").setAttribute("aria-pressed", String(exercise === "example"));
  element("challenge-button").classList.toggle("active", exercise === "challenge");
  element("challenge-button").setAttribute("aria-pressed", String(exercise === "challenge"));
  element("start-challenge").innerHTML = exercise === "challenge" ? "正在练习 <span>↗</span>" : "开始练习 <span>↗</span>";
  element("challenge-feedback").hidden = true;
  element("save-status").textContent = valid ? "已恢复本地草稿" : "草稿自动保存在本机";
  updateEditor();
  clearOutput();
}

function updateEditor() {
  element("highlighted-code").innerHTML = highlightGo(editor.value);
  element("line-numbers").textContent = Array.from({ length: editor.value.split("\n").length }, (_, index) => index + 1).join("\n");
  element("filename").textContent = element("execution-mode").value === "test" ? "main_test.go" : "main.go";
  syncScroll();
  updateCursor();
}

function syncScroll() {
  element("code-highlight").style.transform = `translate(${-editor.scrollLeft}px, ${-editor.scrollTop}px)`;
  element("line-numbers").style.transform = `translateY(${-editor.scrollTop}px)`;
}

function updateCursor() {
  const prefix = editor.value.slice(0, editor.selectionStart);
  const lines = prefix.split("\n");
  element("cursor-status").textContent = `Ln ${lines.length}, Col ${lines[lines.length - 1].length + 1}`;
}

function navigate(view, lessonID, push = true) {
  if (busy) return;
  saveDraft(true);
  const next = lessons.find(lesson => lesson.id === lessonID) || lessons[0];
  const lessonChanged = state.lesson !== next.id;
  state.lesson = next.id;
  state.view = view === "lab" ? "lab" : "learn";
  if (lessonChanged) {
    exercise = "example";
    renderLesson();
    window.scrollTo({ top: 0, behavior: "instant" });
  } else renderView();
  if (push) history.pushState(null, "", `#${state.view}/${state.lesson}`);
  persist();
  closeSidebar();
}

function selectExercise(next) {
  if (busy || exercise === next) return;
  saveDraft(true);
  exercise = next;
  loadDraft();
  editor.focus({ preventScroll: true });
}

function setConsole(panel) {
  for (const name of ["output", "input", "history"]) element(`${name}-panel`).hidden = name !== panel;
  document.querySelectorAll("[data-console]").forEach(button => {
    const active = button.dataset.console === panel;
    button.classList.toggle("active", active);
    button.setAttribute("aria-pressed", String(active));
  });
  if (panel === "history") renderHistory();
}

function clearOutput() {
  lastResult = null;
  element("output-status").className = "output-status";
  element("output-label").textContent = "准备就绪";
  element("output-meta").textContent = "";
  element("output").textContent = `${element("execution-mode").value === "test" ? "$ go test -v" : "$ go run main.go"}\n\n点击「运行代码」，让想法发生。`;
  element("output").classList.add("muted-output");
  element("challenge-feedback").hidden = true;
}

function setBusy(value, kind = "run") {
  busy = value;
  editor.readOnly = value;
  element("stdin").readOnly = value;
  document.querySelectorAll("[data-lesson], [data-view], #execution-mode, #example-button, #challenge-button, #start-challenge, #reset-button, #clear-output, #complete-button").forEach(button => { button.disabled = value; });
  element("next-button").disabled = value || currentLesson() === lessons[lessons.length - 1];
  element("run-button").disabled = value || !config;
  element("format-button").disabled = value || !config;
  element("stop-button").hidden = !value || kind !== "run";
  element("run-button").querySelector("span").textContent = value && kind === "run" ? "运行中…" : "运行代码";
  editor.setAttribute("aria-busy", String(value));
}

async function connect() {
  element("connection-label").textContent = "检测本地环境…";
  try {
    const response = await fetch("/api/config", { signal: AbortSignal.timeout(5000) });
    if (!response.ok) throw new Error("本地服务不可用");
    config = await response.json();
    const version = config.goVersion.match(/go\d+\.\d+(?:\.\d+)?/)?.[0].replace("go", "Go ") || "Go";
    element("connection-label").textContent = `${version} · 已连接`;
    element("connection").classList.remove("offline");
    element("run-button").disabled = busy || !lessons.length;
    element("format-button").disabled = busy || !lessons.length;
  } catch (error) {
    config = null;
    element("connection-label").textContent = "连接断开 · 点击重试";
    element("connection").classList.add("offline");
    element("run-button").disabled = true;
    element("format-button").disabled = true;
    throw new Error(`无法连接本地 Go 服务：${error.message}`);
  }
}

async function post(path, payload, signal) {
  if (!config) throw new Error("请点击右上角重新连接本地服务。");
  const response = await fetch(path, {
    method: "POST", headers: { "Content-Type": "application/json", "X-Practice-Token": config.token },
    body: JSON.stringify(payload), signal,
  });
  const body = await response.json();
  if (!response.ok) throw new Error(body.error || `请求失败（${response.status}）`);
  return body;
}

async function runCode() {
  if (busy || !config || !lessons.length) return;
  if (!editor.value.trim()) { toast("先写一点代码，再试着运行。"); return; }
  saveDraft(true);
  const lesson = currentLesson();
  const mode = element("execution-mode").value;
  const source = editor.value;
  activeRequest = new AbortController();
  setBusy(true);
  setConsole("output");
  element("challenge-feedback").hidden = true;
  element("output-status").className = "output-status running";
  element("output-label").textContent = "编译并运行中";
  element("output-meta").textContent = "";
  element("output").classList.add("muted-output");
  element("output").textContent = "$ go build …\n\n首次编译可能需要一些时间。你可以随时停止。";
  try {
    const result = await post("/api/run", { code: source, input: element("stdin").value, mode }, activeRequest.signal);
    lastResult = result;
    displayResult(result);
    state.history.unshift({ title: lesson.title, exercise, time: Date.now(), ...result });
    state.history = state.history.slice(0, 8);
    if (exercise === "challenge") {
      const matched = mode === lesson.mode && matchesExpected(result, lesson.challenge.expected, mode);
      const feedback = element("challenge-feedback");
      feedback.hidden = false;
      feedback.classList.toggle("mismatch", !matched);
      feedback.textContent = matched ? "✓ 输出核对通过，已记录本课进度。再想一想：你的实现处理好边界了吗？" : result.success ? "程序运行成功，输出还未符合题意。对照预期输出和运行模式，再调整一下。" : "这次还没通过。查看上面的错误信息，修改后再运行。";
      if (matched && !state.completed.includes(lesson.id)) state.completed.push(lesson.id);
      renderProgress();
      renderCourseList();
    }
    persist();
  } catch (error) {
    const stopped = error.name === "AbortError";
    element("output-status").className = "output-status error";
    element("output-label").textContent = stopped ? "已停止" : "执行请求失败";
    element("output").classList.remove("muted-output");
    element("output").textContent = stopped ? "已取消本次执行，可以继续修改代码。" : `${error.message}\n\n若服务已重启，请点击右上角重新连接。`;
  } finally {
    activeRequest = null;
    setBusy(false);
  }
}

function displayResult(result) {
  element("output-status").className = `output-status ${result.success ? "success" : "error"}`;
  element("output-label").textContent = result.timedOut ? `${result.phase === "build" ? "编译" : "运行"}超时` : result.success ? "运行成功" : result.phase === "build" ? "编译失败" : "运行失败";
  element("output-meta").textContent = `${(result.durationMs / 1000).toFixed(2)} s · exit ${result.exitCode}`;
  let output = result.output || (result.success ? "程序执行完毕，没有标准输出。" : "进程已退出，没有输出。请检查入口函数与执行模式。");
  if (result.timedOut) output += `\n\n[${result.phase === "build" ? "编译" : "运行"}已超过时间限制，进程已停止。]`;
  if (result.truncated) output += "\n\n[输出超过 64 KiB，已截断。]";
  element("output").textContent = output;
  element("output").classList.remove("muted-output");
}

function renderHistory() {
  element("history-panel").innerHTML = state.history.length ? state.history.map(record => `<details class="history-entry"><summary>${record.success ? "✓" : "×"} ${escapeHTML(record.title)} · ${record.exercise === "challenge" ? "练习" : "示例"} · ${new Date(record.time).toLocaleTimeString("zh-CN", { hour: "2-digit", minute: "2-digit" })}</summary><pre>${escapeHTML(record.output || "无输出")}</pre></details>`).join("") : '<p class="console-empty">运行过的代码，会在这里留下记录。</p>';
}

async function formatCode() {
  if (busy || !config || !lessons.length) return;
  setBusy(true, "format");
  try {
    const result = await post("/api/format", { code: editor.value }, AbortSignal.timeout(10000));
    editor.value = result.code;
    updateEditor();
    saveDraft(true);
    toast("已按 gofmt 规则整理代码。");
  } catch (error) { toast(error.message); }
  finally { setBusy(false); }
}

function closeSidebar() {
  element("sidebar").classList.remove("open");
  element("sidebar-backdrop").hidden = true;
  element("menu-button").setAttribute("aria-expanded", "false");
  if (window.innerWidth <= 700) element("sidebar").inert = true;
}

function handleCodeChange() {
  updateEditor();
  saveDraft();
  element("challenge-feedback").hidden = true;
  if (lastResult) element("output-label").textContent = "代码已修改 · 上次结果";
}

function bindEvents() {
  document.querySelectorAll("[data-view]").forEach(button => button.addEventListener("click", () => navigate(button.dataset.view, state.lesson)));
  element("course-list").addEventListener("click", event => {
    const button = event.target.closest("[data-lesson]");
    if (button) navigate(state.view, button.dataset.lesson);
  });
  element("course-search").addEventListener("input", renderCourseList);
  element("example-button").addEventListener("click", () => selectExercise("example"));
  element("challenge-button").addEventListener("click", () => selectExercise("challenge"));
  element("start-challenge").addEventListener("click", () => {
    selectExercise("challenge");
    editor.focus({ preventScroll: true });
    element("practice-panel").scrollIntoView({ behavior: "instant", block: "start" });
  });
  element("hint-button").addEventListener("click", () => {
    element("challenge-hint").hidden = !element("challenge-hint").hidden;
    element("hint-button").textContent = element("challenge-hint").hidden ? "给我一点提示" : "收起提示";
  });
  element("complete-button").addEventListener("click", () => {
    const known = state.completed.includes(state.lesson);
    state.completed = known ? state.completed.filter(id => id !== state.lesson) : [...state.completed, state.lesson];
    renderProgress(); renderCourseList(); persist();
    toast(known ? "已撤销本课的掌握标记。" : "已记录学习进度，继续把想法写成代码。");
  });
  element("next-button").addEventListener("click", () => {
    const next = lessons[lessons.indexOf(currentLesson()) + 1];
    if (next) navigate(state.view, next.id);
  });
  editor.addEventListener("input", handleCodeChange);
  editor.addEventListener("scroll", syncScroll);
  editor.addEventListener("click", updateCursor);
  editor.addEventListener("keyup", updateCursor);
  editor.addEventListener("keydown", event => {
    if (event.key === "Escape") { escapeTab = true; return; }
    if (busy || event.isComposing || event.ctrlKey || event.metaKey || event.altKey) return;
    if (event.key === "Tab" && escapeTab) { escapeTab = false; return; }
    escapeTab = false;
    if (event.key !== "Tab" && event.key !== "Enter") return;
    event.preventDefault();
    const updated = editSelection(editor.value, editor.selectionStart, editor.selectionEnd, event.key, event.shiftKey);
    const scrollTop = editor.scrollTop;
    editor.value = updated.source;
    editor.setSelectionRange(updated.start, updated.end);
    editor.scrollTop = scrollTop;
    handleCodeChange();
  });
  element("stdin").addEventListener("input", () => saveDraft());
  element("execution-mode").addEventListener("change", () => { updateEditor(); saveDraft(true); clearOutput(); });
  element("run-button").addEventListener("click", runCode);
  element("format-button").addEventListener("click", formatCode);
  element("stop-button").addEventListener("click", () => activeRequest?.abort());
  element("clear-output").addEventListener("click", clearOutput);
  document.querySelectorAll("[data-console]").forEach(button => button.addEventListener("click", () => setConsole(button.dataset.console)));
  element("connection").addEventListener("click", () => connect().catch(error => toast(error.message)));
  element("reset-button").addEventListener("click", () => element("confirm-dialog").showModal());
  element("confirm-dialog").addEventListener("close", () => {
    if (element("confirm-dialog").returnValue !== "reset") return;
    delete state.drafts[draftKey()];
    loadDraft(); persist(); editor.focus({ preventScroll: true });
    element("confirm-dialog").returnValue = "cancel";
  });
  element("help-button").addEventListener("click", () => element("help-dialog").showModal());
  element("menu-button").addEventListener("click", () => {
    const open = element("sidebar").classList.contains("open");
    if (open) closeSidebar();
    else {
      element("sidebar").inert = false;
      element("sidebar").classList.add("open");
      element("sidebar-backdrop").hidden = false;
      element("menu-button").setAttribute("aria-expanded", "true");
      element("course-search").focus();
    }
  });
  element("sidebar-backdrop").addEventListener("click", closeSidebar);
  window.addEventListener("resize", () => { element("sidebar").inert = window.innerWidth <= 700 && !element("sidebar").classList.contains("open"); });
  document.addEventListener("keydown", event => {
    if (event.key === "Escape" && element("sidebar").classList.contains("open")) { closeSidebar(); element("menu-button").focus(); }
    if (document.querySelector("dialog[open]") || !(event.ctrlKey || event.metaKey)) return;
    if (event.key === "Enter") { event.preventDefault(); runCode(); }
    if (event.key.toLowerCase() === "f" && event.shiftKey) { event.preventDefault(); formatCode(); }
    if (event.key.toLowerCase() === "k") {
      event.preventDefault();
      if (window.innerWidth <= 700 && !element("sidebar").classList.contains("open")) element("menu-button").click();
      element("course-search").focus();
    }
  });
  window.addEventListener("pagehide", () => { saveDraft(true); activeRequest?.abort(); });
  window.addEventListener("popstate", () => {
    const [view, lesson] = location.hash.slice(1).split("/");
    navigate(view, lesson, false);
  });
}

async function initialize() {
  try {
    const results = await Promise.allSettled([
      fetch("/lessons.json").then(response => { if (!response.ok) throw new Error("课程文件加载失败"); return response.json(); }),
      connect(),
    ]);
    if (results[0].status === "rejected") throw results[0].reason;
    lessons = results[0].value;
    if (!Array.isArray(lessons) || !lessons.length) throw new Error("课程数据为空");
    const [routeView, routeLesson] = location.hash.slice(1).split("/");
    if (lessons.some(lesson => lesson.id === routeLesson)) {
      state.lesson = routeLesson;
      state.view = routeView === "lab" ? "lab" : "learn";
    }
    if (!lessons.some(lesson => lesson.id === state.lesson)) state.lesson = lessons[0].id;
    element("lesson-count").textContent = `${lessons.length} LESSONS`;
    const mac = /Mac|iPhone|iPad/.test(navigator.platform);
    element("run-shortcut").textContent = `${mac ? "⌘" : "Ctrl"} ↵ 运行`;
    document.querySelector(".search-box kbd").textContent = mac ? "⌘ K" : "⌃ K";
    bindEvents();
    renderLesson();
    setBusy(false);
    closeSidebar();
    history.replaceState(null, "", `#${state.view}/${state.lesson}`);
    if (results[1].status === "rejected") toast(results[1].reason.message);
    else if (storageWarning) toast(storageWarning);
  } catch (error) {
    element("load-error").hidden = false;
    element("load-error").textContent = `工作台未能加载：${error.message}。请确认本地服务正在运行，然后刷新页面。`;
    element("course-list").textContent = "课程暂不可用";
  }
}

initialize();
