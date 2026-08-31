const $ = (id) => document.getElementById(id);
const state = { cps: [], sel: null };

async function jget(url) {
  const r = await fetch(url);
  if (!r.ok) {
    const e = await r.json().catch(() => ({}));
    throw new Error(e.error || r.statusText);
  }
  return r.json();
}

function esc(s) {
  return String(s).replace(/[&<>"]/g, (c) => ({ "&": "&amp;", "<": "&lt;", ">": "&gt;", '"': "&quot;" }[c]));
}

/* ---------- project header ---------- */

async function loadProject() {
  try {
    const p = await jget("/api/project");
    const el = $("proj");
    el.textContent = p.root + " · " + fmtSize(p.size);
    el.title = p.root;
  } catch (e) {
    $("proj").textContent = e.message;
  }
}

function fmtSize(n) {
  if (!n) return "";
  if (n < 1024 * 1024) return (n / 1024).toFixed(0) + " KB store";
  return (n / (1024 * 1024)).toFixed(1) + " MB store";
}

/* ---------- timeline ---------- */

async function loadCheckpoints() {
  const errEl = $("side-err");
  errEl.classList.add("hidden");
  try {
    const d = await jget("/api/checkpoints");
    state.cps = d.items;
    renderTimeline();
    fillSelects();
  } catch (e) {
    errEl.textContent = "failed to load checkpoints: " + e.message;
    errEl.classList.remove("hidden");
  }
}

function renderTimeline() {
  const ul = $("cps");
  ul.innerHTML = "";
  $("side-empty").classList.toggle("hidden", state.cps.length > 0);

  for (const cp of [...state.cps].reverse()) {
    const li = document.createElement("li");
    li.className = "cp" + (cp.id === state.sel ? " sel" : "") + (cp.latest ? " latest" : "");
    li.dataset.id = cp.id;

    const badges =
      (cp.auto ? '<span class="badge auto">auto</span>' : "") +
      (cp.latest ? '<span class="badge latest">latest</span>' : "");

    const aLen = (cp.added && Array.isArray(cp.added)) ? cp.added.length : 0;
    const mLen = (cp.modified && Array.isArray(cp.modified)) ? cp.modified.length : 0;
    const dLen = (cp.deleted && Array.isArray(cp.deleted)) ? cp.deleted.length : 0;

    const counts =
      (aLen > 0 || mLen > 0 || dLen > 0)
        ? '<span class="counts">' +
          '<span class="cnt-a">+' + aLen + "</span>" +
          '<span class="cnt-m">~' + mLen + "</span>" +
          '<span class="cnt-d">-' + dLen + "</span></span>"
        : "";

    li.innerHTML =
      '<div class="cp-top"><span class="cp-id">#' + cp.id + "</span>" +
      '<span class="cp-msg">' + esc(cp.message) + "</span>" + counts + "</div>" +
      '<div class="cp-meta"><span>' + esc(cp.time) + "</span>" +
      (cp.branch ? '<span class="branch-chip">' + esc(cp.branch) + "</span>" : "") +
      badges + "<span>" + cp.files + " files</span></div>";

    li.onclick = () => selectCheckpoint(cp.id);
    ul.appendChild(li);
  }
}

function idsAsc() {
  return state.cps.map((c) => c.id).sort((a, b) => a - b);
}

function prevOf(id) {
  const ids = idsAsc();
  const i = ids.indexOf(id);
  return i > 0 ? ids[i - 1] : null;
}

function labelFor(id) {
  const cp = state.cps.find((c) => c.id === id);
  return cp ? "#" + id + " " + cp.message : "#" + id;
}

/* ---------- compare controls ---------- */

function fillSelects() {
  const a = $("selA"), b = $("selB");
  const keepA = a.value, keepB = b.value;
  a.innerHTML = "";
  b.innerHTML = "";

  let opt = document.createElement("option");
  opt.value = "none"; opt.textContent = "(empty)";
  a.appendChild(opt);
  for (const id of idsAsc()) {
    opt = document.createElement("option");
    opt.value = String(id); opt.textContent = labelFor(id);
    a.appendChild(opt.cloneNode(true));
    b.appendChild(opt.cloneNode(true));
  }
  opt = document.createElement("option");
  opt.value = "working"; opt.textContent = "working tree";
  b.appendChild(opt);

  if (keepA) a.value = keepA;
  if (keepB) b.value = keepB;
}

function currentRange() {
  return { a: $("selA").value, b: $("selB").value };
}

function setRange(a, b) {
  $("selA").value = String(a);
  $("selB").value = String(b);
}

function selectCheckpoint(id) {
  state.sel = id;
  renderTimeline();
  const prev = prevOf(id);
  setRange(prev ?? "none", id);
  loadDiff();
  refreshGotoButtons();
}

/* ---------- diff ---------- */

async function loadDiff() {
  hideStates();
  $("loading").classList.remove("hidden");
  const { a, b } = currentRange();
  try {
    const d = await jget("/api/diff?a=" + encodeURIComponent(a) + "&b=" + encodeURIComponent(b));
    $("loading").classList.add("hidden");
    renderStat(d.stat);
    renderFiles(d.files);
  } catch (e) {
    $("loading").classList.add("hidden");
    showError(e.message);
  }
}

function hideStates() {
  ["err", "files-empty", "patch"].forEach((id) => $(id).classList.add("hidden"));
  $("files").innerHTML = "";
  $("stat").textContent = "";
}

function showError(msg) {
  const el = $("err");
  el.textContent = msg;
  el.classList.remove("hidden");
}

function renderStat(stat) {
  if (!stat) return;
  const html = esc(stat)
    .replace(/(\d+) added/, '<span class="stat-a">$1 added</span>')
    .replace(/(\d+) modified/, '<span class="stat-m">$1 modified</span>')
    .replace(/(\d+) deleted/, '<span class="stat-d">$1 deleted</span>')
    .replace(/(\d+) binary/, '<span class="stat-b">$1 binary</span>');
  $("stat").innerHTML = html;
}

function splitPath(p) {
  const i = p.lastIndexOf("/");
  return i >= 0 ? [p.slice(0, i + 1), p.slice(i + 1)] : ["", p];
}

function renderFiles(files) {
  const box = $("files");
  box.innerHTML = "";
  $("files-empty").classList.toggle("hidden", files.length > 0);
  $("patch").classList.toggle("hidden", files.length > 0);

  for (const f of files) {
    const row = document.createElement("div");
    row.className = "file-row";
    row.dataset.path = f.path;

    const [dir, name] = splitPath(f.path);
    row.innerHTML =
      '<span class="st st-' + f.status + '">' + f.status + "</span>" +
      '<span class="file-path"><span class="file-dir">' + esc(dir) + "</span><span>" + esc(name) + "</span></span>" +
      (f.binary ? '<span class="file-bin">binary</span>' : "");

    row.onclick = () => {
      document.querySelectorAll(".file-row.active").forEach((el) => el.classList.remove("active"));
      row.classList.add("active");
      showPatch(f);
    };
    box.appendChild(row);
  }

  if (files.length > 0) {
    box.querySelector(".file-row").click();
  }
}

/* ---------- patch rendering with line numbers ---------- */

function parsePatches(patchText, oldStart, newStart) {
  const lines = patchText.split("\n");
  let o = oldStart, n = newStart;
  const out = [];
  for (const raw of lines) {
    const hunk = raw.match(/^@@ -(\d+)(?:,\d+)? \+(\d+)(?:,\d+)? @@/);
    let cls = "l-ctx", oNo = "", nNo = "";
    if (hunk) {
      cls = "l-hunk";
      o = parseInt(hunk[1], 10);
      n = parseInt(hunk[2], 10);
    } else if (raw.startsWith("---") || raw.startsWith("+++") || raw.startsWith("index ")) {
      cls = "l-meta";
    } else if (raw.startsWith("+")) {
      cls = "l-add"; nNo = n; n++;
    } else if (raw.startsWith("-")) {
      cls = "l-del"; oNo = o; o++;
    } else {
      oNo = o; nNo = n; o++; n++;
    }
    out.push({ cls, oNo, nNo, text: raw });
  }
  return out;
}

function showPatch(f) {
  const pre = $("patch");
  pre.classList.remove("hidden");

  const head = document.createElement("span");
  head.className = "dl-file";
  head.textContent = "[" + f.status + "] " + f.path + (f.binary ? "  (binary)" : "");
  pre.innerHTML = "";
  pre.appendChild(head);

  if (f.binary || !f.patch) {
    const note = document.createElement("span");
    note.className = "l-trunc";
    note.textContent = f.binary ? "binary file, contents not shown\n" : "no content change\n";
    pre.appendChild(note);
    return;
  }

  let oStart = 0, nStart = 0;
  const m = f.patch.match(/^@@ -(\d+)(?:,\d+)? \+(\d+)(?:,\d+)? @@/m);
  if (m) { oStart = parseInt(m[1], 10); nStart = parseInt(m[2], 10); }

  const rows = parsePatches(f.patch.replace(/^\n/, ""), oStart, nStart);
  const frag = document.createDocumentFragment();
  for (const r of rows) {
    const line = document.createElement("span");
    line.className = "dl-line " + r.cls;
    line.innerHTML =
      '<span class="dl-no">' + (r.oNo !== "" ? r.oNo : "") + (r.nNo !== "" ? (r.oNo !== "" ? " " : "") + r.nNo : "") + "</span>" +
      '<span class="dl-code">' + esc(r.text) + "\n</span>";
    frag.appendChild(line);
  }
  pre.appendChild(frag);
}

/* ---------- wiring ---------- */

$("selA").onchange = () => { syncSelFromRange(); loadDiff(); };
$("selB").onchange = () => { syncSelFromRange(); loadDiff(); };
$("swap").onclick = () => {
  const { a, b } = currentRange();
  if (b === "working") return;
  setRange(b, a);
  syncSelFromRange();
  loadDiff();
};
$("reload").onclick = () => { loadCheckpoints().then(loadDiff); };

function syncSelFromRange() {
  const { b } = currentRange();
  if (!isNaN(+b)) {
    state.sel = +b;
    renderTimeline();
  }
  refreshGotoButtons();
}

/* ---------- mutations ---------- */

async function mutate(url, confirmMsg) {
  if (confirmMsg && !confirm(confirmMsg)) return null;
  const r = await fetch(url, { method: "POST" });
  const d = await r.json();
  if (!r.ok) {
    showError(d.error || "request failed");
    return null;
  }
  await loadCheckpoints();
  return d;
}

$("bSet").onclick = async () => {
  const msg = prompt("Checkpoint message:", "");
  if (msg === null) return;
  const d = await mutate("/api/set?message=" + encodeURIComponent(msg));
  if (d) toastMsg(`Checkpoint #${d.id} saved (+${d.added} ~${d.modified} -${d.deleted})`);
};

$("bStart").onclick = async () => {
  if (!confirm("Start a new checkpoint session? Baseline #0 will be captured from the current state.")) return;
  const d = await mutate("/api/start");
  if (d) toastMsg(`Baseline #${d.id} dibuat (${d.files} file)`);
};

$("bDrop").onclick = async () => {
  const latest = state.cps.find(c => c.latest);
  if (!latest) { showError("No checkpoints yet."); return; }
  const d = await mutate("/api/drop", `Drop latest checkpoint #${latest.id} (${latest.message})?\nThe working tree will not change.`);
  if (d) toastMsg(`Checkpoint #${d.dropped} dropped`);
};

$("bReset").onclick = async () => {
  if (!state.cps.length) { showError("No checkpoints yet."); return; }
  if (!confirm("End session and delete ALL checkpoints?\nRollback history is cleared. The working tree is NOT touched.")) return;
  const d = await mutate("/api/reset");
  if (d) toastMsg("Session ended. All checkpoints deleted");
};

function selectedId() {
  const { b } = currentRange();
  const n = +b;
  return isNaN(n) ? null : n;
}

function refreshGotoButtons() {
  const id = selectedId();
  const has = id !== null;
  $("bGoto").classList.toggle("hidden", !has);
  $("bGotoPurge").classList.toggle("hidden", !has);
  if (has) $("gotoId").textContent = id;
}

$("bGoto").onclick = () => {
  const id = selectedId();
  if (id === null) return;
  mutate(`/api/goto?id=${id}`).then(d => {
    if (d) {
      setRange("none", String(d.target));
      loadDiff();
      toastMsg(`Restored to #${d.target}. Safety snapshot #${d.safety} was saved first`);
      refreshGotoButtons();
    }
  });
};

$("bGotoPurge").onclick = () => {
  const id = selectedId();
  if (id === null) return;
  if (!confirm(`Goto #${id} and delete EVERY newer checkpoint?\nA safety snapshot of the current state is kept automatically.`)) return;
  mutate(`/api/goto?id=${id}&purge=true`).then(d => {
    if (d) {
      setRange(prevOf(d.target) ?? "none", String(d.target));
      loadDiff();
      toastMsg(`Restored to #${d.target} with purge. Safety #${d.safety} was saved`);
      refreshGotoButtons();
    }
  });
};

function toastMsg(msg) {
  $("stat").innerHTML = '<span class="toastline">' + esc(msg) + "</span>";
}

loadProject();
loadCheckpoints().then(() => {
  const ids = idsAsc();
  if (ids.length > 0) selectCheckpoint(ids[ids.length - 1]);
  refreshGotoButtons();
});
