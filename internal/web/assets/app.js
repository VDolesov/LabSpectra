"use strict";

const state = {
  meta: { root: "", statuses: [], products: [], sources: [], origin_acripol: "АКРИПОЛ", can_open_local: false },
  items: [],
  selectedId: null,
  query: "",
  status: "",
  dates: { aFrom: "", aTo: "", sFrom: "", sTo: "" },
  sort: { key: "", dir: 1 },
  admin: "",
  adminTab: "requests",
  previousView: "table",
  view: "table",
};

let editManufacturerRead = null;

const $ = (sel) => document.querySelector(sel);
const el = (tag, props = {}, ...kids) => {
  const n = document.createElement(tag);
  Object.entries(props).forEach(([k, v]) => {
    if (k === "class") n.className = v;
    else if (k === "html") n.innerHTML = v;
    else if (k.startsWith("on")) n.addEventListener(k.slice(2), v);
    else if (v !== null && v !== undefined) n.setAttribute(k, v);
  });
  kids.flat().forEach((c) => n.append(c?.nodeType ? c : document.createTextNode(c ?? "")));
  return n;
};
const baseName = (p) => p.split("/").pop();

function toast(msg, kind = "") {
  const t = el("div", { class: `toast ${kind}` }, msg);
  $("#toasts").append(t);
  setTimeout(() => t.remove(), 3500);
}

async function api(path, opts = {}) {
  const res = await fetch(path, opts);
  const ct = res.headers.get("content-type") || "";
  const body = ct.includes("json") ? await res.json() : await res.text();
  if (!res.ok) throw new Error((body && body.error) || `Ошибка ${res.status}`);
  return body;
}

const fileURL = (id, rel) => `/files/${encodeURIComponent(id)}/${rel.split("/").map(encodeURIComponent).join("/")}`;

function fmtDay(s) {
  if (!s) return "—";
  const p = s.split("-");
  return p.length === 3 ? `${p[2]}.${p[1]}.${p[0]}` : s;
}
function fmtDateTime(s) {
  if (!s) return "—";
  const d = new Date(s);
  if (isNaN(d)) return s;
  return d.toLocaleString("ru-RU", { day: "2-digit", month: "2-digit", year: "numeric", hour: "2-digit", minute: "2-digit" });
}
function plural(n) {
  const m10 = n % 10, m100 = n % 100;
  if (m10 === 1 && m100 !== 11) return "анализ";
  if (m10 >= 2 && m10 <= 4 && (m100 < 10 || m100 >= 20)) return "анализа";
  return "анализов";
}

async function loadMeta() {
  state.meta = await api("/api/meta");
  renderStatusFilters();
  renderStatusSelect();
  renderCreateProduct();
  renderCreateSource();
  setupCreateOrigin();
  applyMetaVisibility();
}

function applyMetaVisibility() {
  const canOpenLocal = !!state.meta.can_open_local;
  $("#btn-open-xlsx").classList.toggle("hidden", state.view === "admin" || !canOpenLocal);
}

async function loadList() {
  const params = new URLSearchParams();
  if (state.query) params.set("q", state.query);
  if (state.status) params.set("status", state.status);
  if (state.dates.aFrom) params.set("a_from", state.dates.aFrom);
  if (state.dates.aTo) params.set("a_to", state.dates.aTo);
  if (state.dates.sFrom) params.set("s_from", state.dates.sFrom);
  if (state.dates.sTo) params.set("s_to", state.dates.sTo);
  const data = await api("/api/analyses?" + params.toString());
  state.items = data.items || [];
  render();
}

function setView(v) {
  if (state.view !== "admin" && v === "admin") state.previousView = state.view;
  state.view = v;
  $("#view-table").classList.toggle("hidden", v !== "table");
  $("#view-cards").classList.toggle("hidden", v !== "cards");
  $("#view-admin").classList.toggle("hidden", v !== "admin");
  $(".toolbar").classList.toggle("hidden", v === "admin");
  $(".datefilters").classList.toggle("hidden", v === "admin");
  $("#btn-new").classList.toggle("hidden", v === "admin");
  $("#btn-backup").classList.toggle("hidden", v === "admin");
  $("#btn-rebuild").classList.toggle("hidden", v === "admin");
  $("#btn-open-xlsx").classList.toggle("hidden", v === "admin" || !state.meta.can_open_local);
  $("#btn-admin").textContent = v === "admin" ? "Выйти" : "🛡 Управление";
  document.querySelectorAll(".seg").forEach((b) => b.classList.toggle("active", b.dataset.view === v));
}

function render() {
  $("#list-count").textContent = `${state.items.length} ${plural(state.items.length)}`;
  renderTable();
  renderSidebar();
}

function renderStatusFilters() {
  const wrap = $("#status-filters");
  wrap.innerHTML = "";
  const mk = (label, value) =>
    el("span", {
      class: "chip" + (state.status === value ? " active" : ""),
      onclick: () => { state.status = value; renderStatusFilters(); loadList(); },
    }, label);
  wrap.append(mk("Все", ""));
  state.meta.statuses.forEach((s) => wrap.append(mk(s, s)));
}

function fileIcons(id, list, icon) {
  if (!list || list.length === 0) return el("span", { class: "muted" }, "—");
  return el("div", { class: "cell-files" }, ...list.map((rel) =>
    el("a", { class: "file-link", href: fileURL(id, rel), target: "_blank", title: baseName(rel), onclick: (e) => e.stopPropagation() }, icon)));
}
function photoCell(id, list) {
  if (!list || list.length === 0) return el("span", { class: "muted" }, "—");
  return el("div", { class: "cell-files" }, ...list.map((rel) =>
    el("img", { class: "mini-thumb", src: fileURL(id, rel), title: baseName(rel), onclick: (e) => { e.stopPropagation(); openLightbox(fileURL(id, rel)); } })));
}

function openLightbox(url) {
  $("#lightbox-img").src = url;
  $("#lightbox").classList.remove("hidden");
}
function closeLightbox() {
  $("#lightbox").classList.add("hidden");
  $("#lightbox-img").src = "";
}

function editableCell(item, field, type, displayText, extraClass = "") {
  const td = el("td", { class: ("editable " + extraClass).trim(), title: "Двойной клик — изменить" }, displayText);
  td.addEventListener("dblclick", (e) => { e.stopPropagation(); beginEdit(td, item, field, type); });
  return td;
}

function statusCell(item) {
  const td = el("td", { class: "editable", title: "Двойной клик — изменить статус" },
    el("span", { class: "badge", "data-status": item.status }, item.status));
  td.addEventListener("dblclick", (e) => { e.stopPropagation(); beginEdit(td, item, "status", "status"); });
  return td;
}

function beginEdit(td, item, field, type) {
  if (td.querySelector("input, select")) return;
  const current = item[field] || "";
  let input;
  if (type === "status") {
    input = el("select", { class: "cell-edit" },
      ...state.meta.statuses.map((s) => el("option", s === current ? { value: s, selected: "" } : { value: s }, s)));
  } else if (type === "product" || type === "source") {
    const options = type === "product" ? state.meta.products : state.meta.sources;
    input = el("select", { class: "cell-edit" },
      el("option", current === "" ? { value: "", selected: "" } : { value: "" }, "—"),
      ...options.map((p) => el("option", p === current ? { value: p, selected: "" } : { value: p }, p)));
  } else {
    input = el("input", { class: "cell-edit", type: type === "date" ? "date" : "text", value: current });
  }
  td.textContent = "";
  td.append(input);
  input.focus();
  if (input.select) input.select();

  let done = false;
  const commit = async () => {
    if (done) return;
    done = true;
    const val = input.value;
    if (val === current) { renderTable(); return; }
    await saveField(item, field, val);
  };
  input.addEventListener("blur", commit);
  input.addEventListener("keydown", (ev) => {
    if (ev.key === "Enter") { ev.preventDefault(); input.blur(); }
    else if (ev.key === "Escape") { done = true; renderTable(); }
  });
  if (type === "status" || type === "product" || type === "source") input.addEventListener("change", () => input.blur());
}

async function saveField(item, field, val) {
  const u = { ...item, [field]: val };
  const payload = {
    analysis_date: u.analysis_date || "",
    synthesis_date: u.synthesis_date || "",
    product: u.product || "",
    origin: u.origin || "",
    source: u.source || "",
    batch: u.batch || "",
    operator: u.operator || "",
    sample_name: u.sample_name || "",
    description: u.description || "",
    short_result: u.short_result || "",
    status: u.status || "",
    comment: u.comment || "",
  };
  try {
    await api("/api/analyses/" + encodeURIComponent(item.id), {
      method: "PUT", headers: { "Content-Type": "application/json" }, body: JSON.stringify(payload),
    });
    toast(`${item.id}: сохранено`, "ok");
    await loadList();
  } catch (e) { toast(e.message, "err"); renderTable(); }
}

function sortedItems() {
  const items = state.items.slice();
  const k = state.sort.key;
  if (k) {
    const dir = state.sort.dir;
    items.sort((a, b) => {
      const av = (a[k] || "").toString().toLowerCase();
      const bv = (b[k] || "").toString().toLowerCase();
      if (av < bv) return -dir;
      if (av > bv) return dir;
      return 0;
    });
  }
  return items;
}

function renderHeaderArrows() {
  document.querySelectorAll("#reg-table th[data-key]").forEach((th) => {
    const arrow = th.querySelector(".sort-arrow");
    if (!arrow) return;
    arrow.textContent = th.dataset.key === state.sort.key ? (state.sort.dir === 1 ? " ▲" : " ▼") : "";
  });
}

function renderTable() {
  const tbody = $("#reg-tbody");
  tbody.innerHTML = "";
  if (state.items.length === 0) {
    tbody.append(el("tr", { class: "empty-row" },
      el("td", { colspan: "13" }, "Пока нет анализов. Нажмите «＋ Новый анализ», чтобы создать первый.")));
    return;
  }
  sortedItems().forEach((a) => {
    const idCell = el("td", { class: "c-id" },
      el("a", { class: "id-link", href: "#", title: "Открыть карточку", onclick: (e) => { e.preventDefault(); openCard(a.id); } }, a.id));
    tbody.append(el("tr", { class: "reg-row" },
      idCell,
      editableCell(a, "analysis_date", "date", fmtDay(a.analysis_date)),
      editableCell(a, "synthesis_date", "date", fmtDay(a.synthesis_date)),
      editableCell(a, "product", "product", a.product || "—"),
      editableCell(a, "origin", "text", a.origin || "—"),
      editableCell(a, "source", "source", a.source || "—"),
      editableCell(a, "batch", "text", a.batch || "—"),
      editableCell(a, "sample_name", "text", a.sample_name || "—"),
      editableCell(a, "short_result", "text", a.short_result || "—"),
      statusCell(a),
      editableCell(a, "operator", "text", a.operator || "—"),
      el("td", {}, photoCell(a.id, a.attachments.photos)),
      editableCell(a, "comment", "text", a.comment || "—", "c-comment")));
  });
}

function renderSidebar() {
  const list = $("#list");
  list.innerHTML = "";
  if (state.items.length === 0) {
    list.append(el("li", { class: "list-item muted" }, "Ничего не найдено"));
    return;
  }
  state.items.forEach((a) => {
    list.append(el("li", {
      class: "list-item" + (a.id === state.selectedId ? " active" : ""),
      onclick: () => selectAnalysis(a.id),
    },
      el("div", { class: "li-top" },
        el("span", { class: "li-id" }, a.id),
        el("span", { class: "badge", "data-status": a.status }, a.status)),
      el("div", { class: "li-title" }, a.sample_name || a.product || "— без названия —"),
      el("div", { class: "li-meta" }, `${a.product || "—"} · партия ${a.batch || "—"} · ${a.analysis_date || ""}`)));
  });
}

function openCard(id) {
  setView("cards");
  selectAnalysis(id);
}

async function selectAnalysis(id) {
  state.selectedId = id;
  renderSidebar();
  const a = await api("/api/analyses/" + encodeURIComponent(id));
  renderDetail(a);
}

function field(label, name, value, type = "text") {
  const input = type === "textarea"
    ? el("textarea", { name, rows: "2" }, value || "")
    : el("input", { name, type, value: value || "" });
  return el("label", {}, label, input);
}

function renderDetail(a) {
  const detail = $("#detail");
  detail.innerHTML = "";

  const statusSel = el("select", { name: "status" },
    ...state.meta.statuses.map((s) => el("option", s === a.status ? { value: s, selected: "" } : { value: s }, s)));
  const productSel = el("select", { name: "product" },
    el("option", (a.product || "") === "" ? { value: "", selected: "" } : { value: "" }, "—"),
    ...state.meta.products.map((p) => el("option", p === a.product ? { value: p, selected: "" } : { value: p }, p)));
  const sourceSel = el("select", { name: "source" },
    el("option", (a.source || "") === "" ? { value: "", selected: "" } : { value: "" }, "—"),
    ...state.meta.sources.map((s) => el("option", s === a.source ? { value: s, selected: "" } : { value: s }, s)));
  const manufacturer = manufacturerControls(a.origin || "");
  editManufacturerRead = manufacturer.read;

  const actions = [
    el("button", { class: "btn del small", onclick: () => deleteAnalysis(a.id) }, "🗑 Удалить"),
  ];
  if (state.meta.can_open_local) {
    actions.push(el("button", { class: "btn ghost small", onclick: () => openFolder(a.id) }, "📂 Открыть папку"));
  }
  actions.push(el("button", { class: "btn primary small", onclick: () => saveCard(a.id) }, "💾 Сохранить"));

  const head = el("div", { class: "card-head" },
    el("div", {},
      el("div", { class: "card-id" }, a.id),
      el("div", { class: "card-sub" }, `создан ${fmtDateTime(a.created_at)} · изменён ${fmtDateTime(a.updated_at)}`)),
    el("div", { class: "card-head-actions" }, ...actions));

  const fields = el("div", { class: "section" },
    el("h3", {}, "Данные анализа"),
    el("form", { id: "edit-form", class: "form", style: "padding:0" },
      el("div", { class: "grid2" },
        field("Дата анализа", "analysis_date", a.analysis_date, "date"),
        field("Дата синтеза", "synthesis_date", a.synthesis_date, "date"),
        el("label", {}, "Продукт", productSel),
        el("label", {}, "Статус", statusSel),
        manufacturer.selLabel,
        manufacturer.srcLabel,
        el("label", {}, "Происхождение", sourceSel),
        field("Партия", "batch", a.batch),
        field("Оператор", "operator", a.operator)),
      field("Дополнительно", "sample_name", a.sample_name),
      field("Описание", "description", a.description, "textarea"),
      field("Краткий результат", "short_result", a.short_result),
      field("Комментарий", "comment", a.comment, "textarea")));

  const attach = el("div", { class: "section" },
    el("h3", {}, "Вложения"),
    attachGroup(a, "Фотографии", "photo", a.attachments.photos, "image/*", true));

  detail.append(el("div", { class: "card-wrap" }, head, fields, attach));
}

function attachGroup(a, label, kind, list, accept, isImage) {
  const head = el("div", { class: "attach-head" },
    el("span", { class: "label" }, `${label} (${(list || []).length})`),
    el("label", { class: "btn ghost small upload-btn" }, "＋ Добавить",
      el("input", { type: "file", accept, multiple: "", onchange: (e) => uploadFiles(a.id, kind, e.target.files) })));

  let body;
  if (!list || list.length === 0) {
    body = el("div", { class: "muted" }, "пока нет файлов");
  } else if (isImage) {
    body = el("div", { class: "thumbs" }, ...list.map((rel) =>
      el("div", { class: "thumb" },
        el("img", { src: fileURL(a.id, rel), title: baseName(rel), onclick: () => openLightbox(fileURL(a.id, rel)) }),
        el("button", { class: "rm", title: "Удалить", onclick: () => removeAttachment(a.id, kind, rel) }, "✕"))));
  } else {
    body = el("div", { class: "files" }, ...list.map((rel) =>
      el("div", { class: "file-chip" },
        el("span", { class: "fi-icon" }, rel.endsWith(".pdf") ? "📕" : "📄"),
        el("a", { href: fileURL(a.id, rel), target: "_blank" }, baseName(rel)),
        el("button", { class: "rm", title: "Удалить", onclick: () => removeAttachment(a.id, kind, rel) }, "✕"))));
  }
  const group = el("div", { class: "attach-group" }, head, body);
  group.addEventListener("dragover", (e) => { e.preventDefault(); group.classList.add("dragover"); });
  group.addEventListener("dragleave", () => group.classList.remove("dragover"));
  group.addEventListener("drop", (e) => {
    e.preventDefault();
    group.classList.remove("dragover");
    if (e.dataTransfer.files && e.dataTransfer.files.length) uploadFiles(a.id, kind, e.dataTransfer.files);
  });
  return group;
}

async function saveCard(id) {
  const fd = new FormData($("#edit-form"));
  const payload = Object.fromEntries(fd.entries());
  if (editManufacturerRead) payload.origin = editManufacturerRead();
  try {
    await api("/api/analyses/" + encodeURIComponent(id), {
      method: "PUT", headers: { "Content-Type": "application/json" }, body: JSON.stringify(payload),
    });
    toast("Сохранено", "ok");
    await loadList();
    await selectAnalysis(id);
  } catch (e) { toast(e.message, "err"); }
}

async function uploadFiles(id, kind, files) {
  if (!files || files.length === 0) return;
  const fd = new FormData();
  for (const f of files) fd.append("files", f);
  try {
    await api(`/api/analyses/${encodeURIComponent(id)}/attachments?kind=${kind}`, { method: "POST", body: fd });
    toast("Файлы добавлены", "ok");
    await loadList();
    await selectAnalysis(id);
  } catch (e) { toast(e.message, "err"); }
}

async function removeAttachment(id, kind, rel) {
  if (!confirm("Удалить файл? Он переедет в .trash.")) return;
  try {
    await api(`/api/analyses/${encodeURIComponent(id)}/attachments?kind=${kind}&name=${encodeURIComponent(rel)}`, { method: "DELETE" });
    toast("Файл удалён", "ok");
    await loadList();
    await selectAnalysis(id);
  } catch (e) { toast(e.message, "err"); }
}

async function openFolder(id) {
  try { await api(`/api/analyses/${encodeURIComponent(id)}/open-folder`, { method: "POST" }); }
  catch (e) { toast(e.message, "err"); }
}

async function deleteAnalysis(id) {
  if (!confirm(`Удалить анализ ${id}?\nОн скроется из списка и попадёт администратору на подтверждение.`)) return;
  try {
    await api("/api/analyses/" + encodeURIComponent(id), { method: "DELETE" });
    toast(`${id} отправлен на удаление`, "ok");
    state.selectedId = null;
    setView("table");
    await loadList();
  } catch (e) { toast(e.message, "err"); }
}

const adminHdr = () => ({ "X-Admin-Password": state.admin });

async function ensureAdmin() {
  if (!state.admin) {
    const pw = prompt("Пароль администратора:");
    if (!pw) return false;
    try {
      await api("/api/admin/verify", { method: "POST", headers: { "X-Admin-Password": pw } });
      state.admin = pw;
      toast("Режим администратора включён", "ok");
    } catch (e) { toast("Неверный пароль", "err"); return false; }
  }
  return true;
}

async function enterAdmin() {
  if (state.view === "admin") {
    exitAdmin();
    return;
  }
  if (!(await ensureAdmin())) return;
  setView("admin");
  await renderAdminPanel();
}

function exitAdmin() {
  setView(state.previousView || "table");
}

async function renderAdminPanel() {
  document.querySelectorAll(".admin-tab").forEach((b) => b.classList.toggle("active", b.dataset.adminTab === state.adminTab));
  if (state.adminTab === "products") await renderAdminProducts();
  else if (state.adminTab === "maintenance") renderAdminMaintenance();
  else await renderAdminDeleted();
}

async function renderAdminDeleted() {
  const body = $("#admin-content");
  body.innerHTML = "";
  let data;
  try { data = await api("/api/admin/deleted", { headers: adminHdr() }); }
  catch (e) { toast(e.message, "err"); return; }
  const items = data.items || [];
  body.append(el("div", { class: "admin-panel-head" },
    el("div", {},
      el("h2", {}, "Заявки на удаление"),
      el("p", {}, "Обычный пользователь отправляет анализ на удаление, а здесь администратор подтверждает или возвращает его.")),
    el("span", { class: "count" }, `${items.length} ${plural(items.length)}`)));
  if (items.length === 0) {
    body.append(el("div", { class: "admin-empty" }, "Нет анализов, ожидающих подтверждения удаления."));
    return;
  }
  items.forEach((a) => {
    body.append(el("div", { class: "admin-row" },
      el("div", {},
        el("div", { class: "li-id" }, a.id),
        el("div", { class: "li-meta" }, `${a.product || "—"} · партия ${a.batch || "—"} · ${a.analysis_date || ""}`)),
      el("div", { class: "admin-row-actions" },
        el("button", { class: "btn ghost small", onclick: () => adminRestore(a.id) }, "↩ Восстановить"),
        el("button", { class: "btn del small", onclick: () => adminPurge(a.id) }, "🗑 Удалить навсегда"))));
  });
}

async function adminRestore(id) {
  try {
    await api(`/api/admin/analyses/${encodeURIComponent(id)}/restore`, { method: "POST", headers: adminHdr() });
    toast(`${id} восстановлен`, "ok");
    await renderAdminPanel();
    await loadList();
  } catch (e) { toast(e.message, "err"); }
}

async function adminPurge(id) {
  if (!confirm(`Удалить ${id} НАВСЕГДА? Папка уедет в .trash.`)) return;
  try {
    await api(`/api/admin/analyses/${encodeURIComponent(id)}`, { method: "DELETE", headers: adminHdr() });
    toast(`${id} удалён навсегда`, "ok");
    await renderAdminPanel();
    await loadList();
  } catch (e) { toast(e.message, "err"); }
}

async function renderAdminProducts() {
  const body = $("#admin-content");
  body.innerHTML = "";
  let data;
  try { data = await api("/api/admin/products", { headers: adminHdr() }); }
  catch (e) { toast(e.message, "err"); return; }
  const products = data.items || [];
  const input = el("input", { type: "text", id: "admin-product-input", placeholder: "например NEW-01" });
  body.append(el("div", { class: "admin-panel-head" },
    el("div", {},
      el("h2", {}, "Препараты"),
      el("p", {}, "Этот список используется в выпадающем поле «Продукт» при создании и редактировании анализа.")),
    el("span", { class: "count" }, `${products.length} позиций`)));
  body.append(el("form", {
    class: "admin-inline-form",
    onsubmit: async (e) => {
      e.preventDefault();
      await adminAddProduct(input.value);
      input.value = "";
    },
  }, input, el("button", { class: "btn primary", type: "submit" }, "Добавить")));
  body.append(el("div", { class: "product-list" }, ...products.map((p) =>
    el("div", { class: "product-row" },
      el("span", {}, p),
      el("button", { class: "btn del small", onclick: () => adminDeleteProduct(p) }, "Удалить")))));
}

async function adminAddProduct(product) {
  product = product.trim();
  if (!product) return;
  try {
    await api("/api/admin/products", {
      method: "POST",
      headers: { "Content-Type": "application/json", ...adminHdr() },
      body: JSON.stringify({ product }),
    });
    toast("Препарат добавлен", "ok");
    await loadMeta();
    await renderAdminPanel();
  } catch (e) { toast(e.message, "err"); }
}

async function adminDeleteProduct(product) {
  if (!confirm(`Удалить препарат ${product}?`)) return;
  try {
    await api(`/api/admin/products/${encodeURIComponent(product)}`, { method: "DELETE", headers: adminHdr() });
    toast("Препарат удалён", "ok");
    await loadMeta();
    await renderAdminPanel();
  } catch (e) { toast(e.message, "err"); }
}

function renderAdminMaintenance() {
  const body = $("#admin-content");
  body.innerHTML = "";
  body.append(el("div", { class: "admin-panel-head" },
    el("div", {},
      el("h2", {}, "Обслуживание"),
      el("p", {}, "Действия для резервных копий и реестра."))));
  body.append(el("div", { class: "admin-actions-grid" },
    el("button", { class: "btn ghost", onclick: backup }, "💾 Создать бэкап"),
    el("button", { class: "btn ghost", onclick: rebuildRegistry }, "↻ Пересобрать реестр"),
    el("button", { class: "btn primary", onclick: openModal }, "＋ Новый анализ")));
}

function manufacturerControls(current) {
  const acripol = state.meta.origin_acripol;
  const isAcripol = current === acripol;
  const isExternal = current && !isAcripol;
  const sel = el("select", {},
    el("option", current === "" ? { value: "", selected: "" } : { value: "" }, "—"),
    el("option", isAcripol ? { value: acripol, selected: "" } : { value: acripol }, acripol),
    el("option", isExternal ? { value: "__ext__", selected: "" } : { value: "__ext__" }, "стороннее"));
  const src = el("input", { type: "text", value: isExternal ? current : "", placeholder: "название производителя" });
  const srcLabel = el("label", { class: isExternal ? "" : "hidden" }, "Сторонний производитель", src);
  sel.addEventListener("change", () => srcLabel.classList.toggle("hidden", sel.value !== "__ext__"));
  const read = () => (sel.value === "__ext__" ? src.value.trim() : sel.value);
  return { selLabel: el("label", {}, "Производитель", sel), srcLabel, read };
}

function renderStatusSelect() {
  const sel = $("#create-status");
  sel.innerHTML = "";
  state.meta.statuses.forEach((s) => sel.append(el("option", { value: s }, s)));
}

function renderCreateProduct() {
  const sel = $("#create-product");
  sel.innerHTML = "";
  sel.append(el("option", { value: "" }, "—"));
  state.meta.products.forEach((p) => sel.append(el("option", { value: p }, p)));
}

function renderCreateSource() {
  const sel = $("#create-source");
  sel.innerHTML = "";
  sel.append(el("option", { value: "" }, "—"));
  state.meta.sources.forEach((s) => sel.append(el("option", { value: s }, s)));
}

function setupCreateOrigin() {
  const sel = $("#create-origin");
  sel.innerHTML = "";
  sel.append(el("option", { value: "" }, "—"));
  sel.append(el("option", { value: state.meta.origin_acripol }, state.meta.origin_acripol));
  sel.append(el("option", { value: "__ext__" }, "стороннее"));
  sel.onchange = () => $("#create-origin-source-wrap").classList.toggle("hidden", sel.value !== "__ext__");
}

function createOriginValue() {
  const sel = $("#create-origin");
  return sel.value === "__ext__" ? $("#create-origin-source").value.trim() : sel.value;
}

function openModal() {
  $("#create-form").reset();
  $("#create-origin-source-wrap").classList.add("hidden");
  $("#modal").classList.remove("hidden");
}
function closeModal() { $("#modal").classList.add("hidden"); }

async function submitCreate(e) {
  e.preventDefault();
  const payload = Object.fromEntries(new FormData(e.target).entries());
  payload.origin = createOriginValue();
  try {
    const a = await api("/api/analyses", {
      method: "POST", headers: { "Content-Type": "application/json" }, body: JSON.stringify(payload),
    });
    toast(`Создан анализ ${a.id}`, "ok");
    closeModal();
    state.selectedId = a.id;
    await loadList();
    openCard(a.id);
  } catch (err) { toast(err.message, "err"); }
}

async function rebuildRegistry() {
  if (!confirm("Пересобрать registry.xlsx из карточек card.json?")) return;
  if (!(await ensureAdmin())) return;
  try {
    const r = await api("/api/registry/rebuild", { method: "POST", headers: adminHdr() });
    toast(`Реестр пересобран: ${r.rebuilt} строк`, "ok");
  } catch (e) { toast(e.message, "err"); }
}

async function openRegistry() {
  try {
    await api("/api/registry/open", { method: "POST" });
    toast("Открываю registry.xlsx…", "ok");
  } catch (e) { toast(e.message, "err"); }
}

async function backup() {
  if (!(await ensureAdmin())) return;
  try {
    await api("/api/backup", { method: "POST", headers: adminHdr() });
    toast("Резервная копия создана в backups/", "ok");
  } catch (e) { toast(e.message, "err"); }
}

function debounce(fn, ms) { let t; return (...a) => { clearTimeout(t); t = setTimeout(() => fn(...a), ms); }; }

function init() {
  document.querySelectorAll(".seg").forEach((b) => b.addEventListener("click", () => setView(b.dataset.view)));
  $("#btn-new").addEventListener("click", openModal);
  $("#modal-close").addEventListener("click", closeModal);
  $("#create-cancel").addEventListener("click", closeModal);
  $("#create-form").addEventListener("submit", submitCreate);
  $("#btn-rebuild").addEventListener("click", rebuildRegistry);
  $("#btn-open-xlsx").addEventListener("click", openRegistry);
  $("#btn-backup").addEventListener("click", backup);
  $("#btn-admin").addEventListener("click", enterAdmin);
  $("#admin-exit").addEventListener("click", exitAdmin);
  document.querySelectorAll(".admin-tab").forEach((b) => b.addEventListener("click", async () => {
    state.adminTab = b.dataset.adminTab;
    await renderAdminPanel();
  }));
  $("#modal").addEventListener("click", (e) => { if (e.target.id === "modal") closeModal(); });
  $("#lightbox").addEventListener("click", closeLightbox);
  document.addEventListener("keydown", (e) => { if (e.key === "Escape") closeLightbox(); });

  document.querySelectorAll("#reg-table th[data-key]").forEach((th) => {
    th.append(el("span", { class: "sort-arrow" }, ""));
    th.addEventListener("click", () => {
      if (state.sort.key === th.dataset.key) state.sort.dir = -state.sort.dir;
      else { state.sort.key = th.dataset.key; state.sort.dir = 1; }
      renderHeaderArrows();
      renderTable();
    });
  });
  $("#search").addEventListener("input", debounce((e) => { state.query = e.target.value.trim(); loadList(); }, 250));

  const bindDate = (sel, key) => $(sel).addEventListener("change", (e) => { state.dates[key] = e.target.value; loadList(); });
  bindDate("#f-a-from", "aFrom");
  bindDate("#f-a-to", "aTo");
  bindDate("#f-s-from", "sFrom");
  bindDate("#f-s-to", "sTo");
  $("#f-reset").addEventListener("click", () => {
    ["#f-a-from", "#f-a-to", "#f-s-from", "#f-s-to"].forEach((s) => ($(s).value = ""));
    state.dates = { aFrom: "", aTo: "", sFrom: "", sTo: "" };
    loadList();
  });

  loadMeta().then(loadList).catch((e) => toast(e.message, "err"));
}

document.addEventListener("DOMContentLoaded", init);
