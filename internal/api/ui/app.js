const translations = {
  en: {
    page_title: "LittleClaw Console",
    hero_eyebrow: "LittleClaw Runtime",
    hero_title: "Control Console",
    hero_lede: "Run tasks, trigger workflows, approve guarded steps, and inspect live runtime state from one screen.",
    language_label: "Language",
    refresh_button: "Refresh",
    auto_refresh: "Auto refresh",
    metric_task_runs: "Task Runs",
    metric_workflow_runs: "Workflow Runs",
    metric_pending_approvals: "Pending Approvals",
    metric_queue_jobs: "Queue Jobs",
    status_loading: "Loading runtime state...",
    status_refreshing: "Refreshing runtime state...",
    status_synced: "Last synced at {time}",
    quick_actions_title: "Quick Actions",
    quick_actions_desc: "Launch task runs, workflows, or queue jobs.",
    run_task_title: "Run Task",
    task_label: "Task",
    task_placeholder: "Summarize the largest files in this workspace",
    skill_label: "Skill",
    planner_label: "Planner",
    run_task_button: "Run Task",
    run_workflow_title: "Run Workflow",
    workflow_label: "Workflow",
    run_workflow_button: "Run Workflow",
    queue_job_title: "Queue Job",
    target_label: "Target",
    target_task: "task",
    target_workflow: "workflow",
    queue_task_placeholder: "Read a file and summarize it",
    max_attempts_label: "Max Attempts",
    submit_job_button: "Submit Job",
    approvals_title: "Approvals",
    approvals_desc: "Human confirmation gates from workflows.",
    workflow_runs_title: "Workflow Runs",
    workflow_runs_desc: "Recent orchestration executions and resumable runs.",
    task_runs_title: "Task Runs",
    task_runs_desc: "Recent direct runtime executions.",
    queue_title: "Queue",
    queue_desc: "Pending, running, and completed worker jobs.",
    inspector_title: "Inspector",
    inspector_desc: "Raw payloads for the selected object.",
    inspector_placeholder: "Select a run, approval, or job to inspect.",
    catalog_title: "Catalog",
    catalog_desc: "Available workflows and skills.",
    catalog_workflows: "Workflows",
    catalog_skills: "Skills",
    no_approvals: "No approval requests.",
    no_workflow_runs: "No workflow runs yet.",
    no_task_runs: "No task runs yet.",
    no_queue_jobs: "No queue jobs yet.",
    no_output: "No output yet.",
    no_tools: "no tools",
    inspect: "Inspect",
    approve: "Approve",
    reject: "Reject",
    resume: "Resume",
    attempts: "{count} attempts",
    steps: "{count} steps",
    approval_prompt: "{action} {id}. Optional comment:",
    approved: "approved",
    rejected: "rejected",
    pending: "pending",
    completed: "completed",
    failed: "failed",
    running: "running",
    skipped: "skipped",
    waiting_approval: "waiting approval",
    task: "task",
    workflow: "workflow",
  },
  "zh-CN": {
    page_title: "LittleClaw 控制台",
    hero_eyebrow: "LittleClaw 运行时",
    hero_title: "控制台",
    hero_lede: "在一个界面中运行任务、触发工作流、处理人工审批，并查看运行时状态。",
    language_label: "语言",
    refresh_button: "刷新",
    auto_refresh: "自动刷新",
    metric_task_runs: "任务运行",
    metric_workflow_runs: "工作流运行",
    metric_pending_approvals: "待审批",
    metric_queue_jobs: "队列任务",
    status_loading: "正在加载运行时状态...",
    status_refreshing: "正在刷新运行时状态...",
    status_synced: "最近同步时间 {time}",
    quick_actions_title: "快捷操作",
    quick_actions_desc: "启动任务、工作流或队列任务。",
    run_task_title: "运行任务",
    task_label: "任务",
    task_placeholder: "总结当前工作区里最大的文件",
    skill_label: "技能",
    planner_label: "规划器",
    run_task_button: "运行任务",
    run_workflow_title: "运行工作流",
    workflow_label: "工作流",
    run_workflow_button: "运行工作流",
    queue_job_title: "提交队列任务",
    target_label: "目标",
    target_task: "任务",
    target_workflow: "工作流",
    queue_task_placeholder: "读取一个文件并总结内容",
    max_attempts_label: "最大尝试次数",
    submit_job_button: "提交任务",
    approvals_title: "审批",
    approvals_desc: "来自工作流的人工确认节点。",
    workflow_runs_title: "工作流运行记录",
    workflow_runs_desc: "最近的编排执行和可恢复运行。",
    task_runs_title: "任务运行记录",
    task_runs_desc: "最近的直接运行记录。",
    queue_title: "队列",
    queue_desc: "待处理、运行中和已完成的 worker 任务。",
    inspector_title: "检查器",
    inspector_desc: "查看所选对象的原始 JSON。",
    inspector_placeholder: "选择一个运行、审批或队列任务进行查看。",
    catalog_title: "目录",
    catalog_desc: "当前可用的工作流和技能。",
    catalog_workflows: "工作流",
    catalog_skills: "技能",
    no_approvals: "暂无审批请求。",
    no_workflow_runs: "暂无工作流运行记录。",
    no_task_runs: "暂无任务运行记录。",
    no_queue_jobs: "暂无队列任务。",
    no_output: "暂无输出。",
    no_tools: "无工具",
    inspect: "查看",
    approve: "批准",
    reject: "拒绝",
    resume: "继续执行",
    attempts: "已尝试 {count} 次",
    steps: "{count} 步",
    approval_prompt: "{action} {id}。可填写备注：",
    approved: "已批准",
    rejected: "已拒绝",
    pending: "待处理",
    completed: "已完成",
    failed: "失败",
    running: "运行中",
    skipped: "已跳过",
    waiting_approval: "等待审批",
    task: "任务",
    workflow: "工作流",
  },
};

const state = {
  skills: [],
  workflows: [],
  runs: [],
  workflowRuns: [],
  approvals: [],
  jobs: [],
  refreshTimer: null,
  locale: "en",
};

const $ = (selector) => document.querySelector(selector);

function detectLocale() {
  const saved = window.localStorage.getItem("littleclaw.locale");
  if (saved && translations[saved]) return saved;
  return navigator.language && navigator.language.toLowerCase().startsWith("zh") ? "zh-CN" : "en";
}

function t(key, values = {}) {
  const bundle = translations[state.locale] || translations.en;
  const template = bundle[key] ?? translations.en[key] ?? key;
  return Object.entries(values).reduce((acc, [name, value]) => acc.replaceAll(`{${name}}`, String(value)), template);
}

function setLocale(locale) {
  state.locale = translations[locale] ? locale : "en";
  window.localStorage.setItem("littleclaw.locale", state.locale);
  document.documentElement.lang = state.locale === "zh-CN" ? "zh-CN" : "en";
  renderStaticText();
  renderCatalog();
  renderSummary();
  renderApprovals();
  renderWorkflowRuns();
  renderRuns();
  renderQueue();
}

function renderStaticText() {
  document.title = t("page_title");
  document.querySelectorAll("[data-i18n]").forEach((node) => {
    node.textContent = t(node.dataset.i18n);
  });
  document.querySelectorAll("[data-i18n-placeholder]").forEach((node) => {
    node.placeholder = t(node.dataset.i18nPlaceholder);
  });
  $("#language-select").value = state.locale;
}

function setStatus(message, tone = "info") {
  const node = $("#status-pill");
  node.textContent = message;
  node.dataset.tone = tone;
}

function formatTime(value) {
  if (!value) return "-";
  const date = new Date(value);
  if (Number.isNaN(date.getTime())) return value;
  return date.toLocaleString(state.locale === "zh-CN" ? "zh-CN" : "en-US");
}

function trimText(value, limit = 160) {
  if (!value) return "";
  return value.length > limit ? `${value.slice(0, limit)}...` : value;
}

function badge(status) {
  return `<span class="badge ${status}">${t(status)}</span>`;
}

async function request(path, options = {}) {
  const response = await fetch(path, {
    headers: { "Content-Type": "application/json" },
    ...options,
  });
  const isJSON = response.headers.get("content-type")?.includes("application/json");
  const body = isJSON ? await response.json() : await response.text();
  if (!response.ok) {
    const message = typeof body === "object" && body?.error ? body.error : response.statusText;
    throw new Error(message);
  }
  return body;
}

function renderCatalog() {
  const workflowCatalog = $("#workflow-catalog");
  const skillCatalog = $("#skill-catalog");
  workflowCatalog.innerHTML = state.workflows
    .map((item) => `<li><strong>${item.name}</strong><br><span class="hint">${item.description || t("steps", { count: item.step_count })}</span></li>`)
    .join("");
  skillCatalog.innerHTML = state.skills
    .map((item) => `<li><strong>${item.name}</strong><br><span class="hint">${(item.allowed_tools || []).join(", ") || t("no_tools")}</span></li>`)
    .join("");
}

function populateSelectors() {
  const skillOptions = state.skills.map((skill) => `<option value="${skill.name}">${skill.name}</option>`).join("");
  const workflowOptions = state.workflows.map((workflow) => `<option value="${workflow.name}">${workflow.name}</option>`).join("");
  $("#task-skill").innerHTML = skillOptions;
  $("#queue-skill").innerHTML = skillOptions;
  $("#workflow-name").innerHTML = workflowOptions;
  $("#queue-workflow").innerHTML = workflowOptions;
}

function renderSummary() {
  $("#metric-runs").textContent = state.runs.length;
  $("#metric-workflows").textContent = state.workflowRuns.length;
  $("#metric-approvals").textContent = state.approvals.filter((item) => item.status === "pending").length;
  $("#metric-jobs").textContent = state.jobs.length;
}

function showInspector(payload) {
  $("#inspector").textContent = JSON.stringify(payload, null, 2);
}

function attachViewAction(buttonSelector, items) {
  document.querySelectorAll(buttonSelector).forEach((button) => {
    button.addEventListener("click", () => {
      const idx = Number(button.dataset.index);
      showInspector(items[idx]);
    });
  });
}

function renderApprovals() {
  const root = $("#approvals-list");
  if (!state.approvals.length) {
    root.innerHTML = `<div class="empty">${t("no_approvals")}</div>`;
    return;
  }
  root.innerHTML = state.approvals
    .map((item, index) => `
      <article class="row">
        <div class="row-head">
          <p class="row-title">${item.workflow_name} / ${item.step_name}</p>
          ${badge(item.status)}
        </div>
        <div class="row-subtitle">${item.prompt}</div>
        <div class="row-meta">
          <span>${item.id}</span>
          <span>${formatTime(item.updated_at)}</span>
        </div>
        <div class="row-actions">
          <button class="button button-secondary approval-view" data-index="${index}">${t("inspect")}</button>
          ${item.status === "pending" ? `
            <button class="button button-warn approval-approve" data-id="${item.id}">${t("approve")}</button>
            <button class="button button-danger approval-reject" data-id="${item.id}">${t("reject")}</button>
          ` : ""}
        </div>
      </article>
    `)
    .join("");

  attachViewAction(".approval-view", state.approvals);
  document.querySelectorAll(".approval-approve").forEach((button) => {
    button.addEventListener("click", () => decideApproval(button.dataset.id, "approve"));
  });
  document.querySelectorAll(".approval-reject").forEach((button) => {
    button.addEventListener("click", () => decideApproval(button.dataset.id, "reject"));
  });
}

function renderWorkflowRuns() {
  const root = $("#workflow-runs-list");
  if (!state.workflowRuns.length) {
    root.innerHTML = `<div class="empty">${t("no_workflow_runs")}</div>`;
    return;
  }
  root.innerHTML = state.workflowRuns
    .map((item, index) => `
      <article class="row">
        <div class="row-head">
          <p class="row-title">${item.workflow_name}</p>
          ${badge(item.status)}
        </div>
        <div class="row-subtitle">${trimText(item.output || t("no_output"))}</div>
        <div class="row-meta">
          <span>${item.run_id}</span>
          <span>${formatTime(item.finished_at || item.started_at)}</span>
        </div>
        <div class="row-actions">
          <button class="button button-secondary workflow-view" data-index="${index}">${t("inspect")}</button>
          ${item.status === "waiting_approval" ? `<button class="button button-primary workflow-resume" data-id="${item.run_id}">${t("resume")}</button>` : ""}
        </div>
      </article>
    `)
    .join("");

  attachViewAction(".workflow-view", state.workflowRuns);
  document.querySelectorAll(".workflow-resume").forEach((button) => {
    button.addEventListener("click", () => resumeWorkflow(button.dataset.id));
  });
}

function renderRuns() {
  const root = $("#runs-list");
  if (!state.runs.length) {
    root.innerHTML = `<div class="empty">${t("no_task_runs")}</div>`;
    return;
  }
  root.innerHTML = state.runs
    .map((item, index) => `
      <article class="row">
        <div class="row-head">
          <p class="row-title">${trimText(item.output || item.task_id || item.run_id, 100)}</p>
          ${badge(item.status)}
        </div>
        <div class="row-meta">
          <span>${item.run_id}</span>
          <span>${formatTime(item.finished_at || item.started_at)}</span>
        </div>
        <div class="row-actions">
          <button class="button button-secondary run-view" data-index="${index}">${t("inspect")}</button>
        </div>
      </article>
    `)
    .join("");

  attachViewAction(".run-view", state.runs);
}

function renderQueue() {
  const root = $("#queue-list");
  if (!state.jobs.length) {
    root.innerHTML = `<div class="empty">${t("no_queue_jobs")}</div>`;
    return;
  }
  root.innerHTML = state.jobs
    .map((item, index) => `
      <article class="row">
        <div class="row-head">
          <p class="row-title">${item.target === "workflow" ? item.workflow : trimText(item.task, 80)}</p>
          ${badge(item.status)}
        </div>
        <div class="row-meta">
          <span>${item.id}</span>
          <span>${t("attempts", { count: `${item.attempts}/${item.max_attempts}` })}</span>
          <span>${formatTime(item.updated_at)}</span>
        </div>
        <div class="row-subtitle">${trimText(item.last_error || item.output || t("no_output"))}</div>
        <div class="row-actions">
          <button class="button button-secondary queue-view" data-index="${index}">${t("inspect")}</button>
        </div>
      </article>
    `)
    .join("");

  attachViewAction(".queue-view", state.jobs);
}

async function loadCatalogs() {
  const [skills, workflows] = await Promise.all([
    request("/v1/skills"),
    request("/v1/workflows"),
  ]);
  state.skills = skills;
  state.workflows = workflows;
  renderCatalog();
  populateSelectors();
}

async function refreshData() {
  setStatus(t("status_refreshing"));
  const [runs, workflowRuns, approvals, jobs] = await Promise.all([
    request("/v1/runs?limit=8"),
    request("/v1/workflows/runs?limit=8"),
    request("/v1/approvals?limit=8"),
    request("/v1/queue/jobs?limit=8"),
  ]);
  state.runs = runs;
  state.workflowRuns = workflowRuns;
  state.approvals = approvals;
  state.jobs = jobs;
  renderSummary();
  renderApprovals();
  renderWorkflowRuns();
  renderRuns();
  renderQueue();
  setStatus(t("status_synced", { time: new Date().toLocaleTimeString(state.locale === "zh-CN" ? "zh-CN" : "en-US") }));
}

async function decideApproval(id, action) {
  const label = action === "approve" ? t("approve") : t("reject");
  const comment = window.prompt(t("approval_prompt", { action: label, id })) || "";
  await request(`/v1/approvals/${id}/${action}`, {
    method: "POST",
    body: JSON.stringify({ comment }),
  });
  await refreshData();
}

async function resumeWorkflow(id) {
  await request(`/v1/workflows/runs/${id}/resume`, {
    method: "POST",
    body: JSON.stringify({ planner: "auto" }),
  });
  await refreshData();
}

function bindForms() {
  $("#task-form").addEventListener("submit", async (event) => {
    event.preventDefault();
    const formElement = event.currentTarget;
    const form = new FormData(formElement);
    const payload = {
      task: form.get("task"),
      skill: form.get("skill"),
      planner: form.get("planner"),
    };
    const result = await request("/v1/runs", {
      method: "POST",
      body: JSON.stringify(payload),
    });
    showInspector(result);
    formElement.reset();
    populateSelectors();
    await refreshData();
  });

  $("#workflow-form").addEventListener("submit", async (event) => {
    event.preventDefault();
    const formElement = event.currentTarget;
    const form = new FormData(formElement);
    const payload = {
      name: form.get("name"),
      planner: form.get("planner"),
    };
    const result = await request("/v1/workflows/runs", {
      method: "POST",
      body: JSON.stringify(payload),
    });
    showInspector(result);
    formElement.reset();
    populateSelectors();
    await refreshData();
  });

  $("#queue-form").addEventListener("submit", async (event) => {
    event.preventDefault();
    const formElement = event.currentTarget;
    const form = new FormData(formElement);
    const payload = {
      target: form.get("target"),
      task: form.get("task"),
      workflow: form.get("workflow"),
      skill: form.get("skill"),
      planner: "auto",
      max_attempts: Number(form.get("max_attempts") || 1),
    };
    const result = await request("/v1/queue/jobs", {
      method: "POST",
      body: JSON.stringify(payload),
    });
    showInspector(result);
    formElement.reset();
    populateSelectors();
    updateQueueTarget();
    await refreshData();
  });
}

function updateQueueTarget() {
  const target = $("#queue-target").value;
  document.querySelectorAll(".conditional").forEach((node) => {
    node.classList.toggle("hidden", node.dataset.target !== target);
  });
}

function installRefreshLoop() {
  const toggle = $("#auto-refresh");
  const start = () => {
    stop();
    if (toggle.checked) {
      state.refreshTimer = window.setInterval(() => {
        refreshData().catch((error) => setStatus(error.message, "error"));
      }, 5000);
    }
  };
  const stop = () => {
    if (state.refreshTimer) {
      window.clearInterval(state.refreshTimer);
      state.refreshTimer = null;
    }
  };
  toggle.addEventListener("change", start);
  start();
}

function bindLanguage() {
  $("#language-select").addEventListener("change", (event) => {
    setLocale(event.target.value);
    showInspector({ locale: state.locale });
  });
}

async function boot() {
  try {
    state.locale = detectLocale();
    renderStaticText();
    bindLanguage();
    $("#refresh-button").addEventListener("click", () => refreshData().catch((error) => setStatus(error.message, "error")));
    $("#queue-target").addEventListener("change", updateQueueTarget);
    bindForms();
    await loadCatalogs();
    updateQueueTarget();
    await refreshData();
    installRefreshLoop();
  } catch (error) {
    setStatus(error.message, "error");
    $("#inspector").textContent = String(error.stack || error);
  }
}

boot();
