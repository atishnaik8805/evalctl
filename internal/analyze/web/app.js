'use strict';

const state = {
  traces: [],
  selectedId: null,
};

function escapeHTML(s) {
  const div = document.createElement('div');
  div.textContent = s == null ? '' : String(s);
  return div.innerHTML;
}

function formatJSON(value) {
  return JSON.stringify(value, null, 2);
}

async function fetchJSON(url, options) {
  const res = await fetch(url, options);
  let body = null;
  try {
    body = await res.json();
  } catch (e) {
    body = null;
  }
  if (!res.ok) {
    const message = (body && body.error) || res.statusText;
    const err = new Error(message);
    err.status = res.status;
    err.body = body;
    throw err;
  }
  return body;
}

// ---- View switching ----

function showView(name) {
  for (const btn of document.querySelectorAll('#view-tabs button')) {
    btn.classList.toggle('active', btn.dataset.view === name);
  }
  for (const el of document.querySelectorAll('#main .view')) {
    el.classList.toggle('active', el.id === 'view-' + name);
  }
  if (name === 'open-codes') loadOpenCodes();
  if (name === 'axial-codes') loadAxialCodes();
}

for (const btn of document.querySelectorAll('#view-tabs button')) {
  btn.addEventListener('click', () => showView(btn.dataset.view));
}

// ---- Trace list + detail (User Story 1) ----

async function loadTraceList() {
  state.traces = await fetchJSON('/api/traces');
  renderTraceList();
}

function renderTraceList() {
  const list = document.getElementById('trace-list');
  list.innerHTML = '';
  for (const t of state.traces) {
    const li = document.createElement('li');
    li.textContent = t.id;
    li.dataset.id = t.id;
    li.className = t.id === state.selectedId ? 'selected' : '';
    li.addEventListener('click', () => selectTrace(t.id));
    list.appendChild(li);
  }
}

async function selectTrace(id) {
  state.selectedId = id;
  showView('traces');
  renderTraceList();
  const detail = document.getElementById('trace-detail');
  detail.innerHTML = '<p class="empty-state">Loading…</p>';
  try {
    const trace = await fetchJSON('/api/traces/' + encodeURIComponent(id));
    renderTraceDetail(trace);
  } catch (err) {
    detail.innerHTML = '<p class="error">' + escapeHTML(err.message) + '</p>';
  }
}

function renderTraceDetail(trace) {
  const detail = document.getElementById('trace-detail');
  let html = '<h2>' + escapeHTML(trace.id) + '</h2>';
  html += '<section><h3>Input</h3><pre>' + escapeHTML(formatJSON(trace.input)) + '</pre></section>';
  html += '<section><h3>Output</h3><pre>' + escapeHTML(formatJSON(trace.output)) + '</pre></section>';
  if (trace.steps && trace.steps.length > 0) {
    html += '<section><h3>Steps</h3><ol class="steps">';
    for (const step of trace.steps) {
      html += '<li><div class="step-index">#' + escapeHTML(step.index) + '</div>';
      html += '<div><strong>state</strong><pre>' + escapeHTML(formatJSON(step.state)) + '</pre></div>';
      html += '<div><strong>action</strong><pre>' + escapeHTML(formatJSON(step.action)) + '</pre></div>';
      html += '<div><strong>result</strong><pre>' + escapeHTML(formatJSON(step.result)) + '</pre></div>';
      html += '</li>';
    }
    html += '</ol></section>';
  }
  html += renderAnnotationsSection(trace);
  detail.innerHTML = html;
  wireAnnotationHandlers(trace.id);
}

// ---- Open-code annotate form (User Story 2) ----

function renderAnnotationsSection(trace) {
  let html = '<section><h3>Open codes</h3><ul class="annotations">';
  for (const a of trace.annotations) {
    html += '<li data-id="' + escapeHTML(a.id) + '">';
    html += '<span class="label">' + escapeHTML(a.label) + '</span>';
    if (a.note) html += '<span class="note">' + escapeHTML(a.note) + '</span>';
    html += '<button type="button" class="edit-annotation" data-id="' + escapeHTML(a.id) +
      '" data-label="' + escapeHTML(a.label) + '" data-note="' + escapeHTML(a.note) + '">Edit</button>';
    html += '<button type="button" class="delete-annotation" data-id="' + escapeHTML(a.id) + '">Delete</button>';
    html += '</li>';
  }
  if (trace.annotations.length === 0) {
    html += '<li class="empty-state">No open codes yet.</li>';
  }
  html += '</ul>';
  html += '<form id="annotation-form">' +
    '<input type="text" id="annotation-label" placeholder="label" required>' +
    '<input type="text" id="annotation-note" placeholder="note (optional)">' +
    '<button type="submit">Add</button></form>';
  html += '<p id="annotation-error" class="error"></p>';
  html += '</section>';
  return html;
}

function wireAnnotationHandlers(traceId) {
  const form = document.getElementById('annotation-form');
  if (form) {
    form.addEventListener('submit', async (e) => {
      e.preventDefault();
      const label = document.getElementById('annotation-label').value;
      const note = document.getElementById('annotation-note').value;
      const errEl = document.getElementById('annotation-error');
      errEl.textContent = '';
      try {
        await fetchJSON('/api/traces/' + encodeURIComponent(traceId) + '/annotations', {
          method: 'POST',
          headers: { 'Content-Type': 'application/json' },
          body: JSON.stringify({ label, note }),
        });
        await selectTrace(traceId);
      } catch (err) {
        errEl.textContent = err.message;
      }
    });
  }

  for (const btn of document.querySelectorAll('.delete-annotation')) {
    btn.addEventListener('click', async () => {
      try {
        await fetchJSON('/api/annotations/' + encodeURIComponent(btn.dataset.id), { method: 'DELETE' });
        await selectTrace(traceId);
      } catch (err) {
        alert(err.message);
      }
    });
  }

  for (const btn of document.querySelectorAll('.edit-annotation')) {
    btn.addEventListener('click', async () => {
      const newLabel = prompt('Label', btn.dataset.label);
      if (newLabel === null) return;
      const newNote = prompt('Note', btn.dataset.note || '');
      try {
        await fetchJSON('/api/annotations/' + encodeURIComponent(btn.dataset.id), {
          method: 'PUT',
          headers: { 'Content-Type': 'application/json' },
          body: JSON.stringify({ label: newLabel, note: newNote || '' }),
        });
        await selectTrace(traceId);
      } catch (err) {
        alert(err.message);
      }
    });
  }
}

// ---- Aggregated open codes view (User Story 2, FR-008) ----

async function loadOpenCodes() {
  const tbody = document.querySelector('#open-codes-table tbody');
  tbody.innerHTML = '<tr><td colspan="3" class="empty-state">Loading…</td></tr>';
  try {
    const rows = await fetchJSON('/api/open-codes');
    if (rows.length === 0) {
      tbody.innerHTML = '<tr><td colspan="3" class="empty-state">No open codes yet.</td></tr>';
      return;
    }
    tbody.innerHTML = rows.map((r) => (
      '<tr><td>' + escapeHTML(r.display_label) + '</td>' +
      '<td>' + r.count + '</td>' +
      '<td>' + (r.axial_code_id ? escapeHTML(r.axial_code_id) : '<span class="empty-state">uncategorized</span>') + '</td></tr>'
    )).join('');
  } catch (err) {
    tbody.innerHTML = '<tr><td colspan="3" class="error">' + escapeHTML(err.message) + '</td></tr>';
  }
}

async function loadAxialCodes() {
  const board = document.getElementById('axial-codes-board');
  board.innerHTML = '<p class="empty-state">Loading…</p>';
  try {
    const [axialCodes, openCodes, config] = await Promise.all([
      fetchJSON('/api/axial-codes'),
      fetchJSON('/api/open-codes'),
      fetchJSON('/api/config'),
    ]);
    renderAxialBoard(axialCodes, openCodes, config);
  } catch (err) {
    board.innerHTML = '<p class="error">' + escapeHTML(err.message) + '</p>';
  }
}

function renderAxialBoard(axialCodes, openCodes, config) {
  const board = document.getElementById('axial-codes-board');
  const uncategorized = openCodes.filter((oc) => !oc.axial_code_id);

  let html = '<div id="suggest-section">';
  if (config.llm_suggest_available) {
    html += '<button type="button" id="suggest-btn">Suggest with LLM</button>';
  } else {
    html += '<button type="button" disabled title="No --llm-endpoint was configured for this session">Suggest with LLM</button>' +
      '<span class="empty-state"> (not configured)</span>';
  }
  html += '<div id="suggest-draft"></div></div>';

  html += '<form id="create-axial-form">' +
    '<input type="text" id="axial-name" placeholder="New axial code name" required>' +
    '<input type="text" id="axial-description" placeholder="description (optional)">' +
    '<button type="submit">Create</button></form>' +
    '<p id="axial-error" class="error"></p>';

  html += '<div class="axial-cards">';
  for (const ax of axialCodes) {
    html += '<div class="axial-card">';
    html += '<div class="axial-card-header">';
    html += '<strong>' + escapeHTML(ax.name) + '</strong>';
    html += ' <span class="trace-count">(' + ax.trace_count + ' trace' + (ax.trace_count === 1 ? '' : 's') + ')</span>';
    html += ' <button type="button" class="rename-axial" data-id="' + escapeHTML(ax.id) +
      '" data-name="' + escapeHTML(ax.name) + '" data-description="' + escapeHTML(ax.description) + '">Rename</button>';
    html += ' <button type="button" class="delete-axial" data-id="' + escapeHTML(ax.id) + '">Delete</button>';
    html += '</div>';
    if (ax.description) html += '<p class="axial-description">' + escapeHTML(ax.description) + '</p>';
    html += '<ul class="axial-members">';
    for (const label of ax.member_labels) {
      html += '<li>' + escapeHTML(label) +
        ' <button type="button" class="unassign-member" data-axial-id="' + escapeHTML(ax.id) +
        '" data-label="' + escapeHTML(label) + '">Unassign</button></li>';
    }
    if (ax.member_labels.length === 0) html += '<li class="empty-state">No members yet.</li>';
    html += '</ul></div>';
  }
  if (axialCodes.length === 0) html += '<p class="empty-state">No axial codes yet.</p>';
  html += '</div>';

  html += '<section class="uncategorized"><h3>Uncategorized</h3><ul>';
  for (const oc of uncategorized) {
    html += '<li>' + escapeHTML(oc.display_label) + ' (' + oc.count + ')';
    if (axialCodes.length > 0) {
      html += ' <select class="assign-select" data-label="' + escapeHTML(oc.normalized_label) + '">';
      html += '<option value="">Assign to…</option>';
      for (const ax of axialCodes) {
        html += '<option value="' + escapeHTML(ax.id) + '">' + escapeHTML(ax.name) + '</option>';
      }
      html += '</select>';
    }
    html += '</li>';
  }
  if (uncategorized.length === 0) html += '<li class="empty-state">None.</li>';
  html += '</ul></section>';

  board.innerHTML = html;
  wireAxialBoardHandlers();
  wireSuggestHandler();
}

// ---- LLM-suggested axial-code groupings (User Story 4) ----

function wireSuggestHandler() {
  const btn = document.getElementById('suggest-btn');
  if (!btn) return;
  btn.addEventListener('click', async () => {
    const draftEl = document.getElementById('suggest-draft');
    draftEl.innerHTML = '<p class="empty-state">Asking the LLM…</p>';
    try {
      const result = await fetchJSON('/api/axial-codes/suggest', { method: 'POST' });
      renderSuggestDraft(result.groupings || []);
    } catch (err) {
      draftEl.innerHTML = '<p class="error">Suggestion failed: ' + escapeHTML(err.message) + '</p>';
    }
  });
}

function renderSuggestDraft(groupings) {
  const draftEl = document.getElementById('suggest-draft');
  if (groupings.length === 0) {
    draftEl.innerHTML = '<p class="empty-state">No groupings suggested.</p>';
    return;
  }

  let html = '<div class="suggest-groupings">';
  groupings.forEach((g, i) => {
    html += '<div class="suggest-group" data-index="' + i + '">';
    html += '<input type="text" class="suggest-name" value="' + escapeHTML(g.name) + '">';
    html += '<ul class="suggest-labels">';
    for (const label of g.labels) {
      html += '<li><label><input type="checkbox" class="suggest-label-check" value="' + escapeHTML(label) + '" checked> ' + escapeHTML(label) + '</label></li>';
    }
    html += '</ul>';
    html += '<button type="button" class="apply-suggestion" data-index="' + i + '">Apply this group</button>';
    html += '</div>';
  });
  html += '</div><button type="button" id="dismiss-suggestions">Dismiss</button>';
  draftEl.innerHTML = html;

  document.getElementById('dismiss-suggestions').addEventListener('click', () => {
    draftEl.innerHTML = '';
  });

  for (const btn of draftEl.querySelectorAll('.apply-suggestion')) {
    btn.addEventListener('click', async () => {
      const groupEl = draftEl.querySelector('.suggest-group[data-index="' + btn.dataset.index + '"]');
      const name = groupEl.querySelector('.suggest-name').value;
      const labels = Array.from(groupEl.querySelectorAll('.suggest-label-check:checked')).map((cb) => cb.value);
      try {
        await fetchJSON('/api/axial-codes', {
          method: 'POST',
          headers: { 'Content-Type': 'application/json' },
          body: JSON.stringify({ name, description: '', member_labels: labels }),
        });
        await loadAxialCodes();
      } catch (err) {
        alert(err.message);
      }
    });
  }
}

function wireAxialBoardHandlers() {
  const form = document.getElementById('create-axial-form');
  form.addEventListener('submit', async (e) => {
    e.preventDefault();
    const name = document.getElementById('axial-name').value;
    const description = document.getElementById('axial-description').value;
    const errEl = document.getElementById('axial-error');
    errEl.textContent = '';
    try {
      await fetchJSON('/api/axial-codes', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ name, description, member_labels: [] }),
      });
      await loadAxialCodes();
    } catch (err) {
      errEl.textContent = err.message;
    }
  });

  for (const btn of document.querySelectorAll('.rename-axial')) {
    btn.addEventListener('click', async () => {
      const newName = prompt('Name', btn.dataset.name);
      if (newName === null) return;
      const newDesc = prompt('Description', btn.dataset.description || '');
      try {
        await fetchJSON('/api/axial-codes/' + encodeURIComponent(btn.dataset.id), {
          method: 'PUT',
          headers: { 'Content-Type': 'application/json' },
          body: JSON.stringify({ name: newName, description: newDesc || '' }),
        });
        await loadAxialCodes();
      } catch (err) {
        alert(err.message);
      }
    });
  }

  for (const btn of document.querySelectorAll('.delete-axial')) {
    btn.addEventListener('click', async () => {
      if (!confirm('Delete this axial code? Members revert to uncategorized.')) return;
      try {
        await fetchJSON('/api/axial-codes/' + encodeURIComponent(btn.dataset.id), { method: 'DELETE' });
        await loadAxialCodes();
      } catch (err) {
        alert(err.message);
      }
    });
  }

  for (const btn of document.querySelectorAll('.unassign-member')) {
    btn.addEventListener('click', async () => {
      try {
        await fetchJSON(
          '/api/axial-codes/' + encodeURIComponent(btn.dataset.axialId) + '/members/' + encodeURIComponent(btn.dataset.label),
          { method: 'DELETE' }
        );
        await loadAxialCodes();
      } catch (err) {
        alert(err.message);
      }
    });
  }

  for (const select of document.querySelectorAll('.assign-select')) {
    select.addEventListener('change', async () => {
      const axialId = select.value;
      if (!axialId) return;
      try {
        await fetchJSON('/api/axial-codes/' + encodeURIComponent(axialId) + '/members', {
          method: 'POST',
          headers: { 'Content-Type': 'application/json' },
          body: JSON.stringify({ normalized_label: select.dataset.label }),
        });
        await loadAxialCodes();
      } catch (err) {
        alert(err.message);
      }
    });
  }
}

loadTraceList().catch((err) => {
  document.getElementById('trace-list').innerHTML = '<li class="error">' + escapeHTML(err.message) + '</li>';
});
