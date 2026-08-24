const states = {
  draft: "草稿", pending_review: "待领取", in_review: "判读中", awaiting_author: "待作者回应",
  awaiting_recheck: "待复核", ready_decision: "待终审", archived: "已归档"
};

const eventNames = {
  case_created: "创建审查案", integrity_checks_completed: "完成完整性核验", case_claimed: "审查员领取案件",
  finding_reviewed: "记录风险判读", author_response_requested: "发起作者回应", author_response_submitted: "作者提交说明",
  author_response_completed: "作者完成回应", author_response_reviewed: "复核作者回应", recheck_completed: "完成全部复核",
  case_returned: "退回补充", case_approved: "终审通过", archive_digest_recorded: "记录归档摘要",
  draft_figures_revised: "修订草稿图像清单", findings_batch_reviewed: "批量记录风险判读"
};

const escapeHTML = (value = "") => String(value).replace(/[&<>'"]/g, char => ({"&":"&amp;","<":"&lt;",">":"&gt;","'":"&#39;",'"':"&quot;"}[char]));
const requestID = () => crypto.randomUUID();
const caseID = () => location.pathname.split("/").filter(Boolean).at(-1);
const actor = type => ({type, id: type === "reviewer" ? "reviewer-1" : type === "editor" ? "editor-1" : "author"});

async function api(path, options = {}) {
  const response = await fetch(path, {headers: {"Content-Type": "application/json", ...(options.headers || {})}, ...options});
  const payload = await response.json().catch(() => ({}));
  if (!response.ok) throw new Error(payload.error?.message || `请求失败 (${response.status})`);
  return payload;
}

function notice(message, error = false) {
  const node = document.querySelector("#notice");
  if (!node) return;
  node.textContent = message;
  node.className = `notice${error ? " error" : ""}`;
  node.hidden = false;
  window.clearTimeout(notice.timer);
  notice.timer = window.setTimeout(() => node.hidden = true, 5000);
}

function statusTag(state) { return `<span class="status ${state}">${escapeHTML(states[state] || state)}</span>`; }
function shortDigest(value) { return value ? `${value.slice(0, 12)}…` : "未提供"; }
function dateTime(value) { return new Intl.DateTimeFormat("zh-CN", {dateStyle:"medium", timeStyle:"short"}).format(new Date(value)); }

function fillQueueForm(params) {
  const form = document.querySelector("#queue-filters");
  [...form.elements].forEach(field => {
    if (!field.name) return;
    if (field.type === "checkbox") field.checked = params.get(field.name) === "true";
    else field.value = params.get(field.name) || (field.name === "page_size" ? "20" : "");
  });
}

async function loadQueue() {
  const params = new URLSearchParams(location.search);
  fillQueueForm(params);
  try {
    const result = await api(`/api/cases${params.size ? `?${params}` : ""}`);
    const {cases, summary, pagination} = result;
    const emptyMessage = pagination.total > 0 && pagination.page > pagination.total_pages ? "当前页已超出末页，请返回上一页" : "当前筛选条件下没有案件";
    document.querySelector("#case-list").innerHTML = cases.length ? cases.map(item => `<tr><td><a href="/cases/${item.id}"><strong>${escapeHTML(item.manuscript_code)}</strong></a></td><td>${escapeHTML(item.title)}</td><td>${escapeHTML(item.journal_section)}</td><td>${statusTag(item.state)}</td><td>${item.findings.length}</td><td>${item.revision}</td><td>${dateTime(item.updated_at)}</td></tr>`).join("") : `<tr><td colspan="7" class="empty">${emptyMessage}</td></tr>`;
    document.querySelector("#queue-summary").innerHTML = `<span><strong>${summary.total}</strong>筛选命中</span><span><strong>${summary.active}</strong>处理中</span><span><strong>${summary.open_risks}</strong>未闭环风险</span>`;
    const previous = new URLSearchParams(params); previous.set("page", Math.max(1, pagination.page - 1));
    const next = new URLSearchParams(params); next.set("page", pagination.page + 1);
    document.querySelector("#queue-pagination").innerHTML = `<span>第 ${pagination.page} / ${pagination.total_pages || 0} 页，共 ${pagination.total} 项</span><button type="button" class="secondary" data-page-url="?${previous}" ${pagination.page <= 1 ? "disabled" : ""}>上一页</button><button type="button" class="secondary" data-page-url="?${next}" ${pagination.total_pages === 0 || pagination.page >= pagination.total_pages ? "disabled" : ""}>下一页</button>`;
    document.querySelectorAll("[data-page-url]").forEach(button => button.addEventListener("click", () => { history.pushState({}, "", button.dataset.pageUrl); loadQueue(); }));
  } catch (error) {
    document.querySelector("#case-list").innerHTML = `<tr><td colspan="7" class="empty">案件读取失败：${escapeHTML(error.message)}</td></tr>`;
    document.querySelector("#queue-pagination").innerHTML = "";
    notice(error.message, true);
  }
}

function initQueue() {
  document.querySelector("#queue-filters").addEventListener("submit", event => {
    event.preventDefault();
    const data = new FormData(event.currentTarget);
    const params = new URLSearchParams();
    for (const [key, value] of data) if (String(value).trim() && !(key === "page_size" && value === "20")) params.set(key, value);
    params.set("page", "1");
    history.pushState({}, "", params.size ? `?${params}` : "/");
    loadQueue();
  });
  document.querySelector("#clear-filters").addEventListener("click", () => { history.pushState({}, "", "/"); loadQueue(); });
  window.addEventListener("popstate", loadQueue);
  loadQueue();
}

function addFigure(defaults = {}) {
  const template = document.querySelector("#figure-template");
  const fragment = template.content.cloneNode(true);
  Object.entries(defaults).forEach(([key, value]) => { const input = fragment.querySelector(`[data-field="${key}"]`); if (input) input.value = value; });
  fragment.querySelector(".remove-figure").addEventListener("click", event => { event.target.closest(".figure-editor").remove(); numberFigures(); });
  document.querySelector("#figure-rows").append(fragment);
  numberFigures();
}

function numberFigures() { document.querySelectorAll(".figure-editor .figure-number").forEach((node, index) => node.textContent = index + 1); }

function initNewCase() {
  addFigure({pixel_width: 1200, pixel_height: 900});
  document.querySelector("#add-figure").addEventListener("click", () => addFigure({pixel_width: 1200, pixel_height: 900}));
  document.querySelector("#new-case-form").addEventListener("submit", async event => {
    event.preventDefault();
    const form = event.currentTarget;
    const figures = [...document.querySelectorAll(".figure-editor")].map(row => Object.fromEntries([...row.querySelectorAll("[data-field]")].map(input => [input.dataset.field, input.type === "number" ? Number(input.value) : input.value])));
    const payload = {actor: actor("editor"), request_id: requestID(), expected_revision: 0, manuscript_code: form.manuscript_code.value, title: form.title.value, journal_section: form.journal_section.value, figures};
    try { const result = await api("/api/cases", {method:"POST", body:JSON.stringify(payload)}); location.href = `/cases/${result.case.id}`; } catch (error) { notice(error.message, true); }
  });
}

let currentCase;
let currentPreflight = [];
async function loadCase() {
  try {
    const result = await api(`/api/cases/${caseID()}`);
    currentCase = result.case;
    currentPreflight = result.preflight || [];
    renderCase();
  } catch (error) { notice(error.message, true); }
}

function renderCase() {
  const item = currentCase;
  document.title = `${item.manuscript_code} · 图像完整性审查`;
  document.querySelector("#case-heading").innerHTML = `<div><p class="eyebrow">${escapeHTML(item.manuscript_code)}</p><h1>${escapeHTML(item.title)}</h1><p class="muted">${escapeHTML(item.journal_section)} · revision ${item.revision}${item.assignee_id ? ` · 领取人 ${escapeHTML(item.assignee_id)}` : ""}</p></div>${statusTag(item.state)}`;
  document.querySelector("#figure-count").textContent = `${item.figures.length} 幅图像`;
  document.querySelector("#figure-list").innerHTML = item.figures.map(figure => `<tr><td><strong>${escapeHTML(figure.figure_label || "未编号")}${figure.panel_label ? ` · ${escapeHTML(figure.panel_label)}` : ""}</strong><br><span class="muted">${escapeHTML(figure.caption)}</span></td><td class="mono" title="${escapeHTML(figure.content_digest)}">${shortDigest(figure.content_digest)}</td><td>${figure.pixel_width} × ${figure.pixel_height}</td><td>${escapeHTML(figure.experiment_source || "未声明")}</td><td>${escapeHTML(figure.raw_data_reference || "未声明")}</td></tr>`).join("");
  document.querySelector("#edit-figures").hidden = item.state !== "draft";
  document.querySelector("#preflight").innerHTML = item.state === "draft" ? (currentPreflight.length ? `<strong>核验预检：${currentPreflight.length} 项风险</strong> · ${currentPreflight.map(risk => `${escapeHTML(risk.rule_code)} (${escapeHTML(risk.severity)})`).join("、")}` : "核验预检：当前固定规则未发现风险") : "";
  document.querySelector("#finding-count").textContent = `${item.findings.length} 项风险`;
  document.querySelector("#finding-list").innerHTML = item.findings.length ? item.findings.map(renderFinding).join("") : `<div class="empty">自动核验未发现风险项</div>`;
  document.querySelector("#batch-verdicts").hidden = item.state !== "in_review" || !item.findings.some(finding => !finding.review_verdict);
  renderRoundHistory(item, "#round-section", "#round-count", "#round-history", false);
  renderActions(item);
  bindCaseActions();
}

function renderFinding(finding) {
  let control = "";
  if (currentCase.state === "in_review") control = `<div class="finding-control"><label>判读结论<select data-verdict><option value="needs_explanation" ${finding.review_verdict === "needs_explanation" ? "selected" : ""}>需说明</option><option value="established" ${finding.review_verdict === "established" ? "selected" : ""}>成立</option><option value="excluded" ${finding.review_verdict === "excluded" ? "selected" : ""}>排除</option></select></label><label>判读记录<input data-note value="${escapeHTML(finding.review_note)}"></label><button class="secondary verdict-button" data-finding="${finding.id}">保存判读</button></div>`;
  const activeIDs = new Set((currentCase.response_rounds || []).find(round => round.number === currentCase.current_round)?.findings.map(item => item.finding_id) || []);
  if (currentCase.state === "awaiting_recheck" && finding.response_status === "submitted" && activeIDs.has(finding.id)) control = `<div class="finding-control"><label>复核结论<select data-resolution><option value="accepted" ${finding.resolution === "accepted" ? "selected" : ""}>接受回应</option><option value="rejected" ${finding.resolution === "rejected" ? "selected" : ""}>退回补充</option></select></label><label>复核记录<input data-note value="${escapeHTML(finding.resolution_note)}"></label><button class="secondary resolution-button" data-finding="${finding.id}">保存复核</button></div>`;
  const selector = currentCase.state === "in_review" && !finding.review_verdict ? `<label class="finding-select"><input type="checkbox" data-batch-select value="${finding.id}">选择</label>` : "";
  return `<article class="finding" data-finding-row="${finding.id}"><div class="finding-head"><div>${selector}<h3>${escapeHTML(finding.rule_code)}</h3><p>${escapeHTML(finding.evidence)}</p></div><span class="severity ${finding.severity}">${escapeHTML(finding.severity)}</span></div><p class="finding-meta">规则 ${escapeHTML(finding.rule_version)} · 图像 ${finding.figure_ids.map(escapeHTML).join("、")}</p>${finding.review_verdict ? `<p><strong>判读：</strong>${escapeHTML(finding.review_verdict)}${finding.review_note ? ` · ${escapeHTML(finding.review_note)}` : ""}</p>` : ""}${finding.author_explanation ? `<p><strong>作者说明：</strong>${escapeHTML(finding.author_explanation)}</p><p class="finding-meta">替换摘要 ${escapeHTML(finding.replacement_digest || "未提交")} · 原始数据 ${escapeHTML(finding.raw_data_reference || "未提交")}</p>` : ""}${finding.resolution ? `<p><strong>复核：</strong>${escapeHTML(finding.resolution)}${finding.resolution_note ? ` · ${escapeHTML(finding.resolution_note)}` : ""}</p>` : ""}${control}</article>`;
}

function renderRoundHistory(item, sectionSelector, countSelector, historySelector, closedOnly) {
  const rounds = (item.response_rounds || []).filter(round => !closedOnly || round.number !== item.current_round || round.status !== "active");
  const section = document.querySelector(sectionSelector);
  section.hidden = rounds.length === 0;
  if (!rounds.length) return;
  document.querySelector(countSelector).textContent = `${rounds.length} 轮`;
  document.querySelector(historySelector).innerHTML = [...rounds].reverse().map(round => `<section class="round-block"><h3>第 ${round.number} 轮 · ${escapeHTML(round.status)}</h3>${round.return_reason ? `<p><strong>退回原因：</strong>${escapeHTML(round.return_reason)}</p>` : ""}${round.findings.map(evidence => `<p><span class="mono">${escapeHTML(evidence.finding_id)}</span> · 作者说明 ${escapeHTML(evidence.author_explanation || "未提交")} · 复核 ${escapeHTML(evidence.resolution || "未复核")}${evidence.resolution_note ? `（${escapeHTML(evidence.resolution_note)}）` : ""}</p>`).join("")}</section>`).join("");
}

function writeContext(type) { return {actor: actor(type), request_id: requestID(), expected_revision: currentCase.revision}; }
function renderActions(item) {
  let html = `<p>${statusActionText(item.state)}</p>`;
  if (item.state === "draft") html += `<button class="primary" data-action="submit">提交并执行核验</button>`;
  if (item.state === "pending_review") html += `<button class="primary" data-action="claim">领取案件</button>`;
  if (item.state === "in_review") html += `<button class="primary" data-action="author-request" ${item.findings.some(x => !x.review_verdict) ? "disabled" : ""}>发起作者回应</button>`;
  if (item.state === "awaiting_recheck") html += `<button class="primary" data-action="recheck" ${item.findings.some(x => x.response_status === "submitted" && !x.resolution) ? "disabled" : ""}>完成复核并提交终审</button>`;
  if (item.state === "ready_decision") html += `<div class="decision-form"><label>终审记录<input id="decision-note" placeholder="记录终审依据"></label><button class="secondary" data-action="return" ${item.findings.some(finding => finding.resolution === "rejected") ? "" : "disabled"}>退回补充</button><button class="primary" data-action="approve">通过并归档</button></div>`;
  if (item.state === "archived") html += `<a class="primary-link" href="/api/cases/${item.id}/archive">下载 JSON 审查档案</a><button class="secondary" data-action="verify">校验归档摘要</button>`;
  document.querySelector("#case-actions").innerHTML = html;
}

function statusActionText(state) {
  return {draft:"草稿已保存，提交后将执行固定版本完整性规则。",pending_review:"自动核验完成，等待审查员领取。",in_review:"逐项保存判读结论，全部完成后可发起作者回应。",awaiting_author:"已生成作者访问入口，等待作者提交整改材料。",awaiting_recheck:"作者材料齐备，审查员需逐项完成复核。",ready_decision:"复核已完成，责任编辑可作出终审决定。",archived:"案件业务内容已冻结，可下载并校验审查档案。"}[state] || "";
}

async function postAction(path, body) {
  try { const result = await api(path, {method:"POST", body:JSON.stringify(body)}); if (result.case) currentCase = result.case; if (result.preflight) currentPreflight = result.preflight; renderCase(); return result; } catch (error) { notice(error.message, true); throw error; }
}

function addDraftFigure(defaults = {}) {
  const fragment = document.querySelector("#draft-figure-template").content.cloneNode(true);
  Object.entries(defaults).forEach(([key, value]) => { const input = fragment.querySelector(`[data-field="${key}"]`); if (input) input.value = value ?? ""; });
  fragment.querySelector(".remove-figure").addEventListener("click", event => { event.target.closest(".figure-editor").remove(); numberFigures(); });
  document.querySelector("#draft-figure-rows").append(fragment);
  numberFigures();
}

function openDraftEditor() {
  const rows = document.querySelector("#draft-figure-rows");
  rows.innerHTML = "";
  currentCase.figures.forEach(addDraftFigure);
  document.querySelector("#figure-table").hidden = true;
  document.querySelector("#draft-figure-form").hidden = false;
}

function closeDraftEditor() {
  document.querySelector("#figure-table").hidden = false;
  document.querySelector("#draft-figure-form").hidden = true;
}

async function saveDraftFigures(event) {
  event.preventDefault();
  const figures = [...document.querySelectorAll("#draft-figure-rows .figure-editor")].map(row => Object.fromEntries([...row.querySelectorAll("[data-field]")].map(input => [input.dataset.field, input.type === "number" ? Number(input.value) : input.value])));
  try {
    const result = await api(`/api/cases/${caseID()}/figures`, {method:"PUT", body:JSON.stringify({...writeContext("editor"), figures})});
    currentCase = result.case;
    currentPreflight = result.preflight || [];
    closeDraftEditor();
    renderCase();
    loadTimeline();
    notice("草稿图像清单已保存并完成预检");
  } catch (error) { notice(error.message, true); }
}

function bindCaseActions() {
  document.querySelectorAll(".verdict-button").forEach(button => button.addEventListener("click", () => { const control = button.closest(".finding-control"); postAction(`/api/cases/${caseID()}/verdicts`, {...writeContext("reviewer"), finding_id:button.dataset.finding, verdict:control.querySelector("[data-verdict]").value, note:control.querySelector("[data-note]").value}); }));
  document.querySelectorAll(".resolution-button").forEach(button => button.addEventListener("click", () => { const control = button.closest(".finding-control"); postAction(`/api/cases/${caseID()}/resolutions`, {...writeContext("reviewer"), round_number:currentCase.current_round, finding_id:button.dataset.finding, resolution:control.querySelector("[data-resolution]").value, note:control.querySelector("[data-note]").value}); }));
  document.querySelector("#apply-batch-verdict")?.addEventListener("click", () => { document.querySelectorAll("[data-batch-select]:checked").forEach(box => { document.querySelector(`[data-finding-row="${box.value}"] [data-verdict]`).value = document.querySelector("#batch-verdict").value; }); });
  document.querySelector("#save-batch-verdicts")?.addEventListener("click", () => {
    const items = [...document.querySelectorAll("[data-batch-select]:checked")].map(box => { const row = document.querySelector(`[data-finding-row="${box.value}"]`); return {finding_id:box.value, verdict:row.querySelector("[data-verdict]").value, note:row.querySelector("[data-note]").value}; });
    postAction(`/api/cases/${caseID()}/verdicts/batch`, {...writeContext("reviewer"), items}).then(() => notice("批量判读已保存"));
  });
  document.querySelector("[data-action='submit']")?.addEventListener("click", () => postAction(`/api/cases/${caseID()}/submit`, writeContext("editor")));
  document.querySelector("[data-action='claim']")?.addEventListener("click", () => postAction(`/api/cases/${caseID()}/claim`, writeContext("reviewer")));
  document.querySelector("[data-action='recheck']")?.addEventListener("click", () => postAction(`/api/cases/${caseID()}/recheck-complete`, writeContext("reviewer")));
  document.querySelector("[data-action='author-request']")?.addEventListener("click", async () => { const result = await postAction(`/api/cases/${caseID()}/author-request`, writeContext("reviewer")); const url = `${location.origin}/author/${caseID()}?access_token=${encodeURIComponent(result.access_token)}`; const node = document.querySelector("#credential"); node.innerHTML = `<strong>作者访问入口</strong><a href="${escapeHTML(url)}">${escapeHTML(url)}</a>`; node.hidden = false; });
  document.querySelector("[data-action='return']")?.addEventListener("click", () => postAction(`/api/cases/${caseID()}/decision`, {...writeContext("editor"), decision:"returned", note:document.querySelector("#decision-note").value}));
  document.querySelector("[data-action='approve']")?.addEventListener("click", () => postAction(`/api/cases/${caseID()}/decision`, {...writeContext("editor"), decision:"approved", note:document.querySelector("#decision-note").value}));
  document.querySelector("[data-action='verify']")?.addEventListener("click", async () => { try { const result = await api("/api/archives/verify", {method:"POST", body:JSON.stringify({case_reference:currentCase.manuscript_code, digest:currentCase.archive_digest})}); notice(result.valid ? "归档摘要校验一致" : "归档摘要不一致", !result.valid); } catch (error) { notice(error.message, true); } });
}

async function loadTimeline() {
  try { const {timeline} = await api(`/api/cases/${caseID()}/timeline`); document.querySelector("#timeline").innerHTML = timeline.map(event => `<li><strong>${escapeHTML(eventNames[event.event_type] || event.event_type)}</strong> · ${escapeHTML(event.actor_id)}<time>${dateTime(event.occurred_at)} · revision ${event.result_revision} · request ${escapeHTML(event.request_id)}</time></li>`).join(""); } catch (error) { notice(error.message, true); }
}

function initCase() {
  loadCase();
  document.querySelector("#load-timeline").addEventListener("click", loadTimeline);
  document.querySelector("#edit-figures").addEventListener("click", openDraftEditor);
  document.querySelector("#add-draft-figure").addEventListener("click", () => addDraftFigure({pixel_width:1200, pixel_height:900}));
  document.querySelector("#cancel-draft-figures").addEventListener("click", closeDraftEditor);
  document.querySelector("#draft-figure-form").addEventListener("submit", saveDraftFigures);
  loadTimeline();
}

let authorCase;
const authorToken = () => new URLSearchParams(location.search).get("access_token") || "";
async function loadAuthorCase() {
  try { const result = await api(`/api/author/cases/${caseID()}?access_token=${encodeURIComponent(authorToken())}`); authorCase = result.case; renderAuthorCase(); } catch (error) { document.querySelector("#author-heading h1").textContent = "访问凭据无效"; notice(error.message, true); }
}

function renderAuthorCase() {
  document.querySelector("#author-heading").innerHTML = `<div><p class="eyebrow">${escapeHTML(authorCase.manuscript_code)} · 第 ${authorCase.current_round || 0} 轮</p><h1>${escapeHTML(authorCase.title)}</h1><p class="muted">请按当前轮次的风险项提交结构化说明与可用证据。</p></div>${statusTag(authorCase.state)}`;
  const currentRound = (authorCase.response_rounds || []).find(round => round.number === authorCase.current_round);
  const currentIDs = new Set((currentRound?.findings || []).map(item => item.finding_id));
  const pending = authorCase.findings.filter(item => currentIDs.has(item.id));
  const editable = authorCase.state === "awaiting_author" && currentRound?.status === "active";
  document.querySelector("#author-findings").innerHTML = pending.map(item => `<article class="finding"><div class="finding-head"><div><h3>${escapeHTML(item.rule_code)}</h3><p>${escapeHTML(item.evidence)}</p></div><span class="severity ${item.severity}">${escapeHTML(item.severity)}</span></div><p><strong>审查意见：</strong>${escapeHTML(item.review_note || item.review_verdict)}</p><div class="finding-control"><label>结构化说明<textarea data-explanation ${editable ? "" : "disabled"}>${escapeHTML(item.author_explanation)}</textarea></label><label>替换图摘要<input data-replacement value="${escapeHTML(item.replacement_digest)}" ${editable ? "" : "disabled"}><br>原始数据引用<input data-raw value="${escapeHTML(item.raw_data_reference)}" ${editable ? "" : "disabled"}></label>${editable ? `<button class="secondary author-response" data-finding="${item.id}">保存回应</button>` : ""}</div></article>`).join("");
  document.querySelector("#author-actions").innerHTML = editable ? `<p>全部待回应项目保存后，提交进入审查员复核。</p><button class="primary" id="finish-author" ${pending.some(x => x.response_status === "pending") ? "disabled" : ""}>完成并提交复核</button>` : `<p>当前轮次不可修改，请等待审查流程推进。</p>`;
  renderRoundHistory(authorCase, "#author-round-section", "#author-round-count", "#author-round-history", true);
  document.querySelectorAll(".author-response").forEach(button => button.addEventListener("click", async () => { const control = button.closest(".finding-control"); try { const result = await api(`/api/author/cases/${caseID()}/responses`, {method:"POST", body:JSON.stringify({actor:actor("author"), request_id:requestID(), expected_revision:authorCase.revision, access_token:authorToken(), round_number:authorCase.current_round, finding_id:button.dataset.finding, explanation:control.querySelector("[data-explanation]").value, replacement_digest:control.querySelector("[data-replacement]").value, raw_data_reference:control.querySelector("[data-raw]").value})}); authorCase = result.case; renderAuthorCase(); notice("作者回应已保存"); } catch (error) { notice(error.message, true); } }));
  document.querySelector("#finish-author")?.addEventListener("click", async () => { try { const result = await api(`/api/author/cases/${caseID()}/complete`, {method:"POST", body:JSON.stringify({actor:actor("author"), request_id:requestID(), expected_revision:authorCase.revision, access_token:authorToken(), round_number:authorCase.current_round})}); authorCase = result.case; renderAuthorCase(); notice("已提交审查员复核"); } catch (error) { notice(error.message, true); } });
}

const page = document.body.dataset.page;
if (page === "queue") initQueue();
if (page === "new") initNewCase();
if (page === "case") initCase();
if (page === "author") loadAuthorCase();
