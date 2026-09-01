// ==========================================================================
// Zeebe Enterprise Ops Console - Client Application Logic
// ==========================================================================

const API_BASE = '/api';

document.addEventListener('DOMContentLoaded', () => {
  // Initialize default variable row
  addVariableRow('priority', 'HIGH');
  addVariableRow('channel', 'MOBILE_APP');

  checkHealth();
  executeTaskQuery();
  setInterval(checkHealth, 15000);

  document.getElementById('form-start-instance').addEventListener('submit', handleStartProcess);
  document.getElementById('form-task-query').addEventListener('submit', (e) => {
    e.preventDefault();
    executeTaskQuery();
  });
  document.getElementById('btn-deploy').addEventListener('click', handleDeployModels);
});

// Toast notification
function notify(msg, type = 'info') {
  const rack = document.getElementById('toast-rack');
  const toast = document.createElement('div');
  toast.className = 'toast-msg';

  const symbol = type === 'success' ? '●' : type === 'error' ? '▲' : '■';
  const color = type === 'success' ? '#10b981' : type === 'error' ? '#f43f5e' : '#06b6d4';

  toast.innerHTML = `<span style="color: ${color};">${symbol}</span> <span>${msg}</span>`;
  rack.appendChild(toast);

  setTimeout(() => {
    toast.style.opacity = '0';
    toast.style.transform = 'translateY(10px)';
    toast.style.transition = 'all 0.2s ease';
    setTimeout(() => toast.remove(), 200);
  }, 4000);
}

// Health status checker
async function checkHealth() {
  try {
    const res = await fetch(`${API_BASE}/health`);
    const data = await res.json();

    const zDot = document.querySelector('#status-zeebe .dot');
    const tDot = document.querySelector('#status-tasklist .dot');

    if (zDot) zDot.style.background = data.zeebe ? '#10b981' : '#f43f5e';
    if (tDot) tDot.style.background = data.tasklist ? '#10b981' : '#f43f5e';
  } catch (err) {
    console.error('Health check failed', err);
  }
}

// Deploy models
async function handleDeployModels() {
  const btn = document.getElementById('btn-deploy');
  btn.disabled = true;
  btn.textContent = 'Deploying...';

  try {
    const res = await fetch(`${API_BASE}/deploy`, { method: 'POST' });
    const data = await res.json();
    if (res.ok) {
      notify('BPMN workflows & DMN rules successfully deployed to Zeebe!', 'success');
    } else {
      notify(`Deploy failed: ${data.error}`, 'error');
    }
  } catch (err) {
    notify(`Error: ${err.message}`, 'error');
  } finally {
    btn.disabled = false;
    btn.textContent = 'Deploy All (BPMN/DMN)';
  }
}

// Dynamic Variables Builder
function addVariableRow(defaultKey = '', defaultVal = '') {
  const container = document.getElementById('dynamic-variables-container');
  const row = document.createElement('div');
  row.className = 'variable-row';

  row.innerHTML = `
    <input type="text" class="input-text var-key" placeholder="Key" value="${defaultKey}" />
    <select class="input-select var-type">
      <option value="string" selected>String</option>
      <option value="number">Number</option>
      <option value="boolean">Boolean</option>
    </select>
    <input type="text" class="input-text var-val" placeholder="Value" value="${defaultVal}" />
    <button type="button" class="btn-remove-row" onclick="this.parentElement.remove()" title="Delete variable">✕</button>
  `;

  container.appendChild(row);
}

// Collect all dynamic variables
function collectCustomVariables() {
  const custom = {};
  const rows = document.querySelectorAll('.variable-row');

  rows.forEach(r => {
    const key = r.querySelector('.var-key').value.trim();
    const type = r.querySelector('.var-type').value;
    const rawVal = r.querySelector('.var-val').value.trim();

    if (!key) return;

    if (type === 'number') {
      custom[key] = parseFloat(rawVal) || 0;
    } else if (type === 'boolean') {
      custom[key] = rawVal.toLowerCase() === 'true';
    } else {
      custom[key] = rawVal;
    }
  });

  return custom;
}

// Preset Scenario Selector
function applyPreset(scenario) {
  const pSelect = document.getElementById('input-process-id');
  const cInput = document.getElementById('input-customer-id');
  const tSelect = document.getElementById('input-tier');
  const aInput = document.getElementById('input-amount');
  const fInput = document.getElementById('input-fraud-score');
  const varContainer = document.getElementById('dynamic-variables-container');
  varContainer.innerHTML = '';

  switch (scenario) {
    case 'risk-platinum':
      pSelect.value = 'order-risk-fulfillment-process';
      cInput.value = 'cust_platinum_alice';
      tSelect.value = 'PLATINUM';
      aInput.value = '1500';
      fInput.value = '5';
      addVariableRow('vipTag', 'BLACK_CARD');
      addVariableRow('rushDelivery', 'true');
      break;

    case 'risk-high':
      pSelect.value = 'order-risk-fulfillment-process';
      cInput.value = 'cust_high_risk_bob';
      tSelect.value = 'GOLD';
      aInput.value = '8000';
      fInput.value = '55';
      addVariableRow('riskAlert', 'ELEVATED');
      addVariableRow('assignedGroup', 'risk-managers');
      break;

    case 'risk-fraud':
      pSelect.value = 'order-risk-fulfillment-process';
      cInput.value = 'cust_fraud_spammer';
      tSelect.value = 'STANDARD';
      aInput.value = '15000';
      fInput.value = '95';
      addVariableRow('fraudFlag', 'CRITICAL_BLOCK');
      break;

    case 'review':
      pSelect.value = 'order-fulfillment-process';
      cInput.value = 'cust_vip_review_777';
      tSelect.value = 'STANDARD';
      aInput.value = '2500';
      fInput.value = '5';
      addVariableRow('manualReviewReason', 'AMOUNT_OVER_1000');
      break;
  }

  notify(`Applied preset: ${scenario}`);
}

// Start Process Instance
async function handleStartProcess(e) {
  e.preventDefault();
  const btn = document.getElementById('btn-submit-instance');
  btn.disabled = true;
  btn.textContent = 'Submitting to Zeebe...';

  const customVars = collectCustomVariables();
  const payload = {
    processId: document.getElementById('input-process-id').value,
    customerId: document.getElementById('input-customer-id').value,
    customerTier: document.getElementById('input-tier').value,
    totalAmount: parseFloat(document.getElementById('input-amount').value),
    fraudScore: parseFloat(document.getElementById('input-fraud-score').value),
    customVariables: customVars
  };

  try {
    const res = await fetch(`${API_BASE}/instances`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify(payload)
    });

    const data = await res.json();
    if (res.ok) {
      notify(`Instance created! Key: ${data.processInstanceKey}`, 'success');
      setTimeout(executeTaskQuery, 1200);
    } else {
      notify(`Failed to start: ${data.error}`, 'error');
    }
  } catch (err) {
    notify(`Error: ${err.message}`, 'error');
  } finally {
    btn.disabled = false;
    btn.textContent = 'Launch Workflow Instance';
  }
}

// Execute TaskQuery
async function executeTaskQuery() {
  const feed = document.getElementById('task-feed');
  const countBadge = document.getElementById('task-counter');
  feed.innerHTML = '<div class="empty-feed"><p>Querying read model...</p></div>';

  const query = {
    taskDefinitionId: document.getElementById('query-task-def').value.trim() || undefined,
    assignee: document.getElementById('query-assignee').value.trim() || undefined,
    candidateGroup: document.getElementById('query-group').value.trim() || undefined,
    candidateUser: document.getElementById('query-user').value.trim() || undefined,
    state: document.getElementById('query-state').value,
    taskVariables: []
  };

  // Var filter 1
  const v1Name = document.getElementById('var-filter1-name').value.trim();
  const v1Val = document.getElementById('var-filter1-val').value.trim();
  const v1Op = document.getElementById('var-filter1-op').value;
  if (v1Name && v1Val) {
    query.taskVariables.push({
      name: v1Name,
      value: isNaN(v1Val) ? `"${v1Val}"` : v1Val,
      operator: v1Op
    });
  }

  // Var filter 2
  const v2Name = document.getElementById('var-filter2-name').value.trim();
  const v2Val = document.getElementById('var-filter2-val').value.trim();
  const v2Op = document.getElementById('var-filter2-op').value;
  if (v2Name && v2Val) {
    query.taskVariables.push({
      name: v2Name,
      value: isNaN(v2Val) ? `"${v2Val}"` : v2Val,
      operator: v2Op
    });
  }

  try {
    const res = await fetch(`${API_BASE}/tasks/search`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify(query)
    });

    const tasks = await res.json();
    if (!res.ok) {
      feed.innerHTML = `<div class="empty-feed"><p style="color: var(--accent-rose);">Error: ${tasks.error || 'Query failed'}</p></div>`;
      countBadge.textContent = '0 tasks';
      return;
    }

    countBadge.textContent = `${tasks.length} task${tasks.length === 1 ? '' : 's'}`;

    if (!tasks || tasks.length === 0) {
      feed.innerHTML = `
        <div class="empty-feed">
          <p>No tasks found matching criteria.</p>
          <span style="font-size: 0.75rem; color: var(--text-dim);">Trigger a high-risk instance to create a user task.</span>
        </div>`;
      return;
    }

    renderTaskCards(tasks);
  } catch (err) {
    feed.innerHTML = `<div class="empty-feed"><p style="color: var(--accent-rose);">${err.message}</p></div>`;
  }
}

// Render Task Cards
function renderTaskCards(tasks) {
  const feed = document.getElementById('task-feed');
  feed.innerHTML = '';

  tasks.forEach(task => {
    const card = document.createElement('div');
    card.className = 'task-item';

    const creation = task.creationDate ? new Date(task.creationDate).toLocaleString() : '-';
    const groups = task.candidateGroups && task.candidateGroups.length ? task.candidateGroups.join(', ') : '<none>';
    const isCompleted = task.taskState === 'COMPLETED';

    // Format custom task variables
    let varChips = '';
    let varTableRows = '';
    if (task.variables && task.variables.length) {
      task.variables.forEach(v => {
        const valStr = typeof v.value === 'object' ? JSON.stringify(v.value) : String(v.value);
        varChips += `<span class="tag tag-cyan" title="Variable: ${v.name}">${v.name}: ${valStr}</span> `;
        varTableRows += `<tr><td style="color:var(--accent-cyan); font-family:var(--font-mono); padding: 2px 8px;">${v.name}</td><td style="font-family:var(--font-mono); padding: 2px 8px;">${valStr}</td></tr>`;
      });
    }

    const varInspectorId = `vars-${task.id}`;

    card.innerHTML = `
      <div class="task-item-top">
        <div>
          <div class="task-item-title">${task.name || 'User Task'}</div>
          <div class="task-item-sub">TaskID: ${task.id} &bull; ProcessInstance: ${task.processInstanceKey}</div>
        </div>
        <span class="tag ${task.taskState === 'CREATED' ? 'tag-amber' : 'tag-emerald'}">${task.taskState}</span>
      </div>

      <div class="task-tags">
        <span class="tag tag-indigo">Assignee: ${task.assignee || '<unassigned>'}</span>
        <span class="tag tag-purple">Group: ${groups}</span>
        <span class="tag tag-count">Created: ${creation}</span>
      </div>

      ${varChips ? `<div class="task-tags" style="margin-top: 0.25rem;">${varChips}</div>` : ''}

      ${varTableRows ? `
        <div id="${varInspectorId}" style="display: none; background: rgba(0,0,0,0.3); border: 1px solid var(--border-subtle); border-radius: 4px; margin: 0.5rem 0; padding: 0.5rem; font-size: 0.72rem;">
          <table style="width: 100%; border-collapse: collapse;">
            <thead><tr style="text-align: left; color: var(--text-dim); border-bottom: 1px solid var(--border-subtle);"><th>Variable Key</th><th>Value</th></tr></thead>
            <tbody>${varTableRows}</tbody>
          </table>
        </div>
      ` : ''}

      <div class="task-item-footer">
        <button type="button" class="btn-text" onclick="toggleVarInspector('${varInspectorId}')">
          ${varTableRows ? '🔍 Inspect All Variables' : ''}
        </button>

        ${!isCompleted ? `
          <div class="task-actions-group">
            <button class="btn-action btn-reject" onclick="submitDecision('${task.id}', false)">
              ✕ Reject
            </button>
            <button class="btn-action btn-approve" onclick="submitDecision('${task.id}', true)">
              ✓ Approve
            </button>
          </div>
        ` : '<span style="font-size: 0.75rem; color: var(--accent-emerald);">Completed</span>'}
      </div>
    `;

    feed.appendChild(card);
  });
}

function toggleVarInspector(elementId) {
  const el = document.getElementById(elementId);
  if (el) {
    el.style.display = el.style.display === 'none' ? 'block' : 'none';
  }
}

// Submit Decision (Approve / Reject)
async function submitDecision(taskId, approved) {
  const btnGroup = event?.target?.parentElement;
  if (btnGroup) {
    btnGroup.innerHTML = '<span style="font-size: 0.75rem; color: var(--accent-cyan);">Completing...</span>';
  }

  try {
    const res = await fetch(`${API_BASE}/tasks/${taskId}/complete`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ approved })
    });

    const data = await res.json();
    if (res.ok) {
      notify(`Task ${taskId} marked as ${approved ? 'APPROVED' : 'REJECTED'}!`, 'success');
      // Re-query immediately and again after ES indexing sync
      executeTaskQuery();
      setTimeout(executeTaskQuery, 1000);
    } else {
      notify(`Action failed: ${data.error || 'Server error'}`, 'error');
      executeTaskQuery();
    }
  } catch (err) {
    notify(`Error: ${err.message}`, 'error');
    executeTaskQuery();
  }
}

// Reset Query
function resetTaskQuery() {
  document.getElementById('query-task-def').value = '';
  document.getElementById('query-assignee').value = '';
  document.getElementById('query-group').value = '';
  document.getElementById('query-user').value = '';
  document.getElementById('query-state').value = 'CREATED';
  document.getElementById('var-filter1-name').value = '';
  document.getElementById('var-filter1-val').value = '';
  document.getElementById('var-filter2-name').value = '';
  document.getElementById('var-filter2-val').value = '';
  executeTaskQuery();
}
