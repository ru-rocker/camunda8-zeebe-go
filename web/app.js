// ==========================================================================
// Camunda 8 Zeebe Orchestration Dashboard - App Logic
// ==========================================================================

const API_BASE = '/api';

// Initialize on DOM ready
document.addEventListener('DOMContentLoaded', () => {
  checkHealth();
  fetchTasks();
  setInterval(checkHealth, 15000); // Check cluster health every 15s

  document.getElementById('form-start-process').addEventListener('submit', handleStartProcess);
  document.getElementById('form-task-query').addEventListener('submit', (e) => {
    e.preventDefault();
    fetchTasks();
  });
  document.getElementById('btn-deploy').addEventListener('click', handleDeployModels);
});

// Toast notification helper
function showToast(message, type = 'info') {
  const container = document.getElementById('toast-container');
  const toast = document.createElement('div');
  toast.className = 'toast';

  const icon = type === 'success' ? '✅' : type === 'error' ? '❌' : 'ℹ️';
  toast.innerHTML = `<span>${icon}</span> <span>${message}</span>`;
  container.appendChild(toast);

  setTimeout(() => {
    toast.style.opacity = '0';
    toast.style.transform = 'translateX(100%)';
    setTimeout(() => toast.remove(), 300);
  }, 4000);
}

// Check cluster health
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

// Deploy BPMN & DMN models
async function handleDeployModels() {
  const btn = document.getElementById('btn-deploy');
  btn.disabled = true;
  btn.textContent = 'Deploying...';

  try {
    const res = await fetch(`${API_BASE}/deploy`, { method: 'POST' });
    const data = await res.json();
    if (res.ok) {
      showToast('Successfully deployed BPMN & DMN models to Zeebe!', 'success');
    } else {
      showToast(`Deploy failed: ${data.error || 'Unknown error'}`, 'error');
    }
  } catch (err) {
    showToast(`Deployment error: ${err.message}`, 'error');
  } finally {
    btn.disabled = false;
    btn.textContent = 'Deploy Models (BPMN/DMN)';
  }
}

// Presets loader
function setPreset(scenario) {
  const processSelect = document.getElementById('input-process-id');
  const custInput = document.getElementById('input-customer-id');
  const tierSelect = document.getElementById('input-tier');
  const amountInput = document.getElementById('input-amount');
  const fraudInput = document.getElementById('input-fraud-score');

  switch (scenario) {
    case 'risk-platinum':
      processSelect.value = 'order-risk-fulfillment-process';
      custInput.value = 'cust_platinum_alice';
      tierSelect.value = 'PLATINUM';
      amountInput.value = '1500';
      fraudInput.value = '5';
      break;

    case 'risk-high':
      processSelect.value = 'order-risk-fulfillment-process';
      custInput.value = 'cust_high_risk_bob';
      tierSelect.value = 'GOLD';
      amountInput.value = '8000';
      fraudInput.value = '55';
      break;

    case 'risk-fraud':
      processSelect.value = 'order-risk-fulfillment-process';
      custInput.value = 'cust_fraud_spammer';
      tierSelect.value = 'STANDARD';
      amountInput.value = '15000';
      fraudInput.value = '95';
      break;

    case 'review':
      processSelect.value = 'order-fulfillment-process';
      custInput.value = 'cust_vip_review_777';
      tierSelect.value = 'STANDARD';
      amountInput.value = '2500';
      fraudInput.value = '5';
      break;
  }

  showToast(`Loaded preset: ${scenario}`);
}

// Start Process Instance
async function handleStartProcess(e) {
  e.preventDefault();
  const btn = document.getElementById('btn-submit-process');
  btn.disabled = true;
  btn.textContent = 'Launching Instance...';

  const payload = {
    processId: document.getElementById('input-process-id').value,
    customerId: document.getElementById('input-customer-id').value,
    customerTier: document.getElementById('input-tier').value,
    totalAmount: parseFloat(document.getElementById('input-amount').value),
    fraudScore: parseFloat(document.getElementById('input-fraud-score').value),
  };

  try {
    const res = await fetch(`${API_BASE}/instances`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify(payload)
    });

    const data = await res.json();
    if (res.ok) {
      showToast(`Process instance ${data.processInstanceKey} created successfully!`, 'success');
      setTimeout(fetchTasks, 1500); // Reload tasks after Zeebe exports to Tasklist
    } else {
      showToast(`Launch failed: ${data.error}`, 'error');
    }
  } catch (err) {
    showToast(`Error: ${err.message}`, 'error');
  } finally {
    btn.disabled = false;
    btn.textContent = 'Launch Workflow Instance';
  }
}

// Fetch & Query Tasks (TaskQuery)
async function fetchTasks() {
  const container = document.getElementById('tasks-container');
  container.innerHTML = '<div class="empty-state"><p>Querying Tasklist Read Model...</p></div>';

  const queryPayload = {
    assignee: document.getElementById('filter-assignee').value.trim() || undefined,
    candidateGroup: document.getElementById('filter-candidate-group').value.trim() || undefined,
    candidateUser: document.getElementById('filter-candidate-user').value.trim() || undefined,
    state: document.getElementById('filter-state').value,
    taskVariables: []
  };

  // Variable Filter 1
  const var1Name = document.getElementById('var1-name').value.trim();
  const var1Val = document.getElementById('var1-value').value.trim();
  const var1Op = document.getElementById('var1-op').value;
  if (var1Name && var1Val) {
    queryPayload.taskVariables.push({
      name: var1Name,
      value: isNaN(var1Val) ? `"${var1Val}"` : var1Val,
      operator: var1Op
    });
  }

  // Variable Filter 2
  const var2Name = document.getElementById('var2-name').value.trim();
  const var2Val = document.getElementById('var2-value').value.trim();
  const var2Op = document.getElementById('var2-op').value;
  if (var2Name && var2Val) {
    queryPayload.taskVariables.push({
      name: var2Name,
      value: isNaN(var2Val) ? `"${var2Val}"` : var2Val,
      operator: var2Op
    });
  }

  try {
    const res = await fetch(`${API_BASE}/tasks/search`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify(queryPayload)
    });

    const tasks = await res.json();
    if (!res.ok) {
      container.innerHTML = `<div class="empty-state"><p style="color: var(--accent-rose);">Error: ${tasks.error || 'Failed to query tasks'}</p></div>`;
      return;
    }

    if (!tasks || tasks.length === 0) {
      container.innerHTML = `
        <div class="empty-state">
          <p>No tasks found matching the filter criteria.</p>
          <span style="font-size: 0.8rem; color: var(--text-muted);">Launch a high-risk or manual review workflow on the left to see tasks.</span>
        </div>`;
      return;
    }

    renderTasks(tasks);
  } catch (err) {
    container.innerHTML = `<div class="empty-state"><p style="color: var(--accent-rose);">Network error: ${err.message}</p></div>`;
  }
}

// Render task list
function renderTasks(tasks) {
  const container = document.getElementById('tasks-container');
  container.innerHTML = '';

  tasks.forEach(task => {
    const card = document.createElement('div');
    card.className = 'task-card';

    const creationFormatted = task.creationDate ? new Date(task.creationDate).toLocaleString() : '-';
    const groups = task.candidateGroups && task.candidateGroups.length ? task.candidateGroups.join(', ') : '<none>';
    const isCompleted = task.taskState === 'COMPLETED';

    card.innerHTML = `
      <div class="task-header">
        <div class="task-title-group">
          <h3>${task.name || 'User Task'}</h3>
          <div class="task-meta">Task ID: ${task.id} &bull; ProcessInstance: ${task.processInstanceKey}</div>
        </div>
        <span class="badge ${task.taskState === 'CREATED' ? 'badge-amber' : 'badge-emerald'}">
          ${task.taskState}
        </span>
      </div>

      <div class="task-badges">
        <span class="badge badge-blue">Assignee: ${task.assignee || '<unassigned>'}</span>
        <span class="badge badge-purple">Group: ${groups}</span>
        <span class="badge badge-emerald">Created: ${creationFormatted}</span>
      </div>

      ${!isCompleted ? `
        <div class="task-actions">
          <button class="btn btn-danger" style="padding: 0.35rem 0.85rem; font-size: 0.78rem;" onclick="completeTask('${task.id}', false)">
            ✕ Reject
          </button>
          <button class="btn btn-success" style="padding: 0.35rem 0.85rem; font-size: 0.78rem;" onclick="completeTask('${task.id}', true)">
            ✓ Approve
          </button>
        </div>
      ` : ''}
    `;

    container.appendChild(card);
  });
}

// Complete / Approve / Reject Task
async function completeTask(taskId, approved) {
  try {
    const res = await fetch(`${API_BASE}/tasks/${taskId}/complete`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ approved })
    });

    const data = await res.json();
    if (res.ok) {
      showToast(`Task ${taskId} marked as ${approved ? 'APPROVED' : 'REJECTED'}!`, 'success');
      setTimeout(fetchTasks, 1000);
    } else {
      showToast(`Action failed: ${data.error}`, 'error');
    }
  } catch (err) {
    showToast(`Error: ${err.message}`, 'error');
  }
}

// Clear all search filters
function clearFilters() {
  document.getElementById('filter-assignee').value = '';
  document.getElementById('filter-candidate-group').value = '';
  document.getElementById('filter-candidate-user').value = '';
  document.getElementById('filter-state').value = 'CREATED';
  document.getElementById('var1-name').value = '';
  document.getElementById('var1-value').value = '';
  document.getElementById('var2-name').value = '';
  document.getElementById('var2-value').value = '';
  fetchTasks();
}
