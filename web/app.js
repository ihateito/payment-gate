// PaymentGate Frontend Logic

const API_BASE = window.location.origin.includes(':3000') 
  ? 'http://localhost:8080' 
  : window.location.origin;

// UUID generator helper
function generateUUID() {
  if (crypto.randomUUID) {
    return crypto.randomUUID();
  }
  return 'xxxxxxxx-xxxx-4xxx-yxxx-xxxxxxxxxxxx'.replace(/[xy]/g, function(c) {
    const r = Math.random() * 16 | 0, v = c == 'x' ? r : (r & 0x3 | 0x8);
    return v.toString(16);
  });
}

// Generate random dummy payment data
function randomizeForm() {
  const sampleMerchants = [
    '00000000-0000-0000-0000-000000000001',
    'b2c1d3e4-f5a6-7890-abcd-ef1234567890',
    '7f8e9d0a-1b2c-3d4e-5f6a-7b8c9d0e1f2a'
  ];
  const sampleCurrencies = ['USD', 'EUR', 'GBP', 'KZT', 'RUB'];
  
  const randMerchant = sampleMerchants[Math.floor(Math.random() * sampleMerchants.length)];
  const randCustNum = Math.floor(100 + Math.random() * 900);
  const randAmount = (Math.floor(500 + Math.random() * 9500) / 100).toFixed(2);
  const randCurr = sampleCurrencies[Math.floor(Math.random() * sampleCurrencies.length)];

  document.getElementById('merchant-id').value = randMerchant;
  document.getElementById('customer-id').value = `cust_${randCustNum}`;
  document.getElementById('amount').value = randAmount;
  document.getElementById('currency').value = randCurr;
  document.getElementById('idempotency-key').value = generateUUID();
}

// Health check status poller
async function checkHealth() {
  const gwChip = document.getElementById('gateway-status');
  const dbChip = document.getElementById('db-status');
  
  try {
    const res = await fetch(`${API_BASE}/healthz`);
    if (res.ok) {
      const data = await res.json();
      gwChip.className = 'status-chip online';
      gwChip.querySelector('.status-label').textContent = 'Gateway: Online';

      if (data.database === 'healthy') {
        dbChip.className = 'status-chip online';
        dbChip.querySelector('.status-label').textContent = 'PostgreSQL: Healthy';
      } else {
        dbChip.className = 'status-chip offline';
        dbChip.querySelector('.status-label').textContent = 'PostgreSQL: Down';
      }
    } else {
      gwChip.className = 'status-chip offline';
      gwChip.querySelector('.status-label').textContent = 'Gateway: Error';
      dbChip.className = 'status-chip offline';
    }
  } catch (err) {
    gwChip.className = 'status-chip offline';
    gwChip.querySelector('.status-label').textContent = 'Gateway: Offline';
    dbChip.className = 'status-chip offline';
    dbChip.querySelector('.status-label').textContent = 'PostgreSQL: Unknown';
  }
}

// Fetch and render recent transactions
let cachedPayments = [];
async function fetchTransactions() {
  const tbody = document.getElementById('tx-tbody');
  const emptyState = document.getElementById('empty-state');
  const txCount = document.getElementById('tx-count');

  try {
    const res = await fetch(`${API_BASE}/v1/payments`);
    if (!res.ok) return;

    const payments = await res.json() || [];
    cachedPayments = payments;
    renderTable(payments);
  } catch (err) {
    console.error('Failed to load transactions:', err);
  }
}

function renderTable(payments) {
  const tbody = document.getElementById('tx-tbody');
  const emptyState = document.getElementById('empty-state');
  const txCount = document.getElementById('tx-count');
  const filter = (document.getElementById('search-input').value || '').toLowerCase();

  const filtered = payments.filter(p => {
    return p.id.toLowerCase().includes(filter) ||
           p.customer_id.toLowerCase().includes(filter) ||
           p.idempotency_key.toLowerCase().includes(filter);
  });

  txCount.textContent = filtered.length;

  if (filtered.length === 0) {
    tbody.innerHTML = '';
    emptyState.classList.remove('hidden');
    return;
  }

  emptyState.classList.add('hidden');
  tbody.innerHTML = filtered.map(p => {
    const formattedAmount = p.amount ? p.amount.formatted : `${p.amount} ${p.currency}`;
    const dateStr = new Date(p.created_at).toLocaleTimeString();
    
    // Status badges
    let statusClass = 'badge-warning';
    if (p.status === 'AUTHORIZED' || p.status === 'CAPTURED') statusClass = 'badge-success';
    if (p.status === 'FAILED') statusClass = 'badge-danger';

    let outboxBadge = '<span class="badge badge-warning">PENDING</span>';
    if (p.outbox_status === 'PUBLISHED') {
      outboxBadge = '<span class="badge badge-success">✓ PUBLISHED</span>';
    } else if (p.outbox_status === 'FAILED') {
      outboxBadge = '<span class="badge badge-danger">FAILED</span>';
    }

    return `
      <tr onclick="highlightPipeline('${p.status}', '${p.outbox_status}')" style="cursor: pointer;">
        <td class="mono" title="${p.id}">${p.id.slice(0, 8)}...${p.id.slice(-4)}</td>
        <td class="amount-cell">${formattedAmount}</td>
        <td>${p.customer_id}</td>
        <td><span class="badge ${statusClass}">${p.status}</span></td>
        <td>${outboxBadge}</td>
        <td class="mono" style="font-size: 0.78rem;">${dateStr}</td>
      </tr>
    `;
  }).join('');
}

// Pipeline visualizer update
function highlightPipeline(status, outboxStatus, isReplayed = false) {
  const steps = {
    gw: document.getElementById('step-gw'),
    redis: document.getElementById('step-redis'),
    db: document.getElementById('step-db'),
    outbox: document.getElementById('step-outbox'),
    kafka: document.getElementById('step-kafka'),
  };

  // Reset steps
  Object.values(steps).forEach(s => {
    s.className = 'step';
  });

  if (isReplayed) {
    steps.gw.className = 'step completed';
    steps.redis.className = 'step completed';
    steps.redis.querySelector('span').textContent = 'Replayed from Cache';
    return;
  }

  steps.gw.className = 'step completed';
  steps.redis.className = 'step completed';
  steps.redis.querySelector('span').textContent = 'Lock Acquired';
  steps.db.className = 'step completed';

  if (outboxStatus === 'PUBLISHED') {
    steps.outbox.className = 'step completed';
    steps.kafka.className = 'step completed';
  } else {
    steps.outbox.className = 'step active';
    steps.kafka.className = 'step';
  }
}

// Submit payment
async function handlePaymentSubmit(e) {
  e.preventDefault();
  const submitBtn = document.getElementById('submit-btn');
  const responseBox = document.getElementById('response-box');
  const responseStatusBadge = document.getElementById('response-status-badge');
  const idempotencyBadge = document.getElementById('idempotency-badge');
  const responseTime = document.getElementById('response-time');
  const responseJson = document.getElementById('response-json');

  const amountVal = parseFloat(document.getElementById('amount').value);
  const minorAmount = Math.round(amountVal * 100);
  const currencyVal = document.getElementById('currency').value;
  const merchantId = document.getElementById('merchant-id').value;
  const customerId = document.getElementById('customer-id').value;
  const idempKey = document.getElementById('idempotency-key').value;

  const payload = {
    merchant_id: merchantId,
    customer_id: customerId,
    amount: minorAmount,
    currency: currencyVal,
    method: 'card'
  };

  submitBtn.disabled = true;
  submitBtn.querySelector('.spinner').classList.remove('hidden');

  const startTime = performance.now();
  try {
    const res = await fetch(`${API_BASE}/v1/payments`, {
      method: 'POST',
      headers: {
        'Content-Type': 'application/json',
        'Idempotency-Key': idempKey
      },
      body: JSON.stringify(payload)
    });

    const elapsed = Math.round(performance.now() - startTime);
    const isReplayed = res.headers.get('Idempotent-Replayed') === 'true';
    const body = await res.json();

    responseBox.classList.remove('hidden');
    responseTime.textContent = `${elapsed}ms`;
    responseStatusBadge.textContent = `${res.status} ${res.statusText || 'OK'}`;
    responseStatusBadge.className = res.ok ? 'badge badge-success' : 'badge badge-danger';

    if (isReplayed) {
      idempotencyBadge.classList.remove('hidden');
    } else {
      idempotencyBadge.classList.add('hidden');
    }

    responseJson.textContent = JSON.stringify(body, null, 2);

    highlightPipeline(body.status, 'PENDING', isReplayed);
    await fetchTransactions();
  } catch (err) {
    responseBox.classList.remove('hidden');
    responseStatusBadge.textContent = 'Network Error';
    responseStatusBadge.className = 'badge badge-danger';
    idempotencyBadge.classList.add('hidden');
    responseJson.textContent = String(err);
  } finally {
    submitBtn.disabled = false;
    submitBtn.querySelector('.spinner').classList.add('hidden');
  }
}

// Initial setup
document.addEventListener('DOMContentLoaded', () => {
  randomizeForm();
  checkHealth();
  fetchTransactions();

  // Polling every 3 seconds
  setInterval(() => {
    checkHealth();
    fetchTransactions();
  }, 3000);

  document.getElementById('randomize-btn').addEventListener('click', randomizeForm);
  document.getElementById('new-key-btn').addEventListener('click', () => {
    document.getElementById('idempotency-key').value = generateUUID();
  });

  document.getElementById('resend-btn').addEventListener('click', (e) => {
    handlePaymentSubmit(e);
  });

  document.getElementById('payment-form').addEventListener('submit', handlePaymentSubmit);

  document.getElementById('refresh-all-btn').addEventListener('click', () => {
    checkHealth();
    fetchTransactions();
  });

  document.getElementById('refresh-tx-btn').addEventListener('click', fetchTransactions);

  document.getElementById('search-input').addEventListener('input', () => {
    renderTable(cachedPayments);
  });
});
