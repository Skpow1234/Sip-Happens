package http

import (
	"html"
	"net/http"
	"strings"
)

// Minimal read-only ops console (products, inventory, orders) using admin JSON APIs.
const adminConsoleHTML = `<!DOCTYPE html>
<html lang="en">
<head>
  <meta charset="utf-8"/>
  <meta name="viewport" content="width=device-width, initial-scale=1"/>
  <title>Sip-Happens Ops</title>
  <style>
    :root { --bg:#0f1419; --panel:#1a2332; --text:#e7ecf3; --muted:#8b9cb3; --accent:#f59e0b; --border:#2d3a4d; }
    * { box-sizing: border-box; }
    body { font-family: ui-sans-serif, system-ui, sans-serif; background: var(--bg); color: var(--text); margin: 0; padding: 1rem 1.25rem 2rem; line-height: 1.45; }
    h1 { font-size: 1.25rem; margin: 0 0 0.25rem; }
    .sub { color: var(--muted); font-size: 0.875rem; margin-bottom: 1rem; }
    .bar { display: flex; flex-wrap: wrap; gap: 0.5rem; align-items: center; margin-bottom: 1rem; padding: 0.75rem; background: var(--panel); border: 1px solid var(--border); border-radius: 8px; }
    .bar label { font-size: 0.8rem; color: var(--muted); }
    .bar input[type=password], .bar input[type=text], .bar select { background: var(--bg); border: 1px solid var(--border); color: var(--text); padding: 0.35rem 0.5rem; border-radius: 4px; min-width: 12rem; }
    button, .tab { cursor: pointer; background: var(--panel); border: 1px solid var(--border); color: var(--text); padding: 0.4rem 0.75rem; border-radius: 6px; font-size: 0.875rem; }
    button.primary { background: var(--accent); color: #111; border-color: var(--accent); font-weight: 600; }
    .tabs { display: flex; gap: 0.35rem; margin-bottom: 0.75rem; }
    .tab.active { border-color: var(--accent); color: var(--accent); }
    .panel { display: none; }
    .panel.active { display: block; }
    table { width: 100%; border-collapse: collapse; font-size: 0.85rem; }
    th, td { text-align: left; padding: 0.45rem 0.5rem; border-bottom: 1px solid var(--border); }
    th { color: var(--muted); font-weight: 500; }
    tr:hover td { background: rgba(255,255,255,0.03); }
    pre { background: var(--panel); border: 1px solid var(--border); padding: 0.75rem; border-radius: 8px; overflow: auto; font-size: 0.75rem; max-height: 24rem; }
    .err { color: #f87171; margin: 0.5rem 0; }
    .ok { color: #86efac; font-size: 0.8rem; }
    a { color: var(--accent); }
  </style>
</head>
<body>
  <h1>Sip-Happens Ops</h1>
  <p class="sub">Read-only console for orders, products, and inventory. Writes use <a href="/docs">/docs</a> or curl.</p>
  <div class="bar">
    <label>Admin token <input type="password" id="token" placeholder="Bearer value" autocomplete="off"/></label>
    <button type="button" class="primary" id="saveToken">Save</button>
    <button type="button" id="reload">Reload</button>
    <span class="ok" id="status"></span>
  </div>
  <div class="tabs">
    <button type="button" class="tab active" data-tab="orders">Orders</button>
    <button type="button" class="tab" data-tab="products">Products</button>
    <button type="button" class="tab" data-tab="inventory">Inventory</button>
    <button type="button" class="tab" data-tab="reservations">Reservations</button>
    <button type="button" class="tab" data-tab="prices">Price history</button>
  </div>
  <section id="panel-orders" class="panel active">
    <div class="bar">
      <label>Status <select id="orderStatus"><option value="">(all)</option>
        <option>PENDING_PAYMENT</option><option>PAID</option><option>FULFILLING</option>
        <option>SHIPPED</option><option>CANCELLED</option></select></label>
      <label>Limit <input type="text" id="orderLimit" value="50" size="4"/></label>
    </div>
    <div id="ordersErr" class="err"></div>
    <table><thead><tr><th>Ref</th><th>Status</th><th>Total</th><th>Customer</th><th>Created</th><th></th></tr></thead>
    <tbody id="ordersBody"></tbody></table>
    <h3>Detail</h3>
    <pre id="orderDetail">Select an order…</pre>
  </section>
  <section id="panel-products" class="panel">
    <div id="productsErr" class="err"></div>
    <table><thead><tr><th>SKU</th><th>Name</th><th>Active</th><th>ID</th></tr></thead>
    <tbody id="productsBody"></tbody></table>
  </section>
  <section id="panel-inventory" class="panel">
    <div class="bar"><label>SKU filter <input type="text" id="invSku" placeholder="optional"/></label></div>
    <div id="inventoryErr" class="err"></div>
    <table><thead><tr><th>SKU</th><th>On hand</th><th>Reserved</th><th>Available</th><th>Safety</th></tr></thead>
    <tbody id="inventoryBody"></tbody></table>
  </section>
  <section id="panel-reservations" class="panel">
    <div class="bar"><label>SKU filter <input type="text" id="resSku" placeholder="optional"/></label></div>
    <div id="resErr" class="err"></div>
    <table><thead><tr><th>Order</th><th>SKU</th><th>Qty</th><th>Order status</th><th>Expires</th></tr></thead>
    <tbody id="resBody"></tbody></table>
  </section>
  <section id="panel-prices" class="panel">
    <div class="bar">
      <label>Product ID <input type="text" id="priceProductId" placeholder="prd_… or internal UUID" size="36"/></label>
      <button type="button" id="loadPriceHistory">Load</button>
      <span class="sub" style="margin:0">Requires finance+ role. Source: <code>GET /admin/products/{id}/price-history</code> (bi-temporal ledger joined with admin audit rows).</span>
    </div>
    <div id="pricesErr" class="err"></div>
    <table>
      <thead>
        <tr>
          <th>Currency</th><th>Amount (minor)</th><th>Valid from</th><th>Valid through</th>
          <th>Set by</th><th>Set at</th><th>Ended by</th><th>Ended reason</th>
        </tr>
      </thead>
      <tbody id="pricesBody"></tbody>
    </table>
  </section>
  <script>
    const LS_KEY = 'sip_happens_admin_token';
    const $ = (id) => document.getElementById(id);
    const tokenEl = $('token');
    tokenEl.value = localStorage.getItem(LS_KEY) || '';
    $('saveToken').onclick = () => { localStorage.setItem(LS_KEY, tokenEl.value.trim()); $('status').textContent = 'saved'; };
    async function api(path) {
      const t = tokenEl.value.trim();
      if (!t) throw new Error('Set admin token first');
      const r = await fetch(path, { headers: { Authorization: 'Bearer ' + t } });
      const text = await r.text();
      let body; try { body = JSON.parse(text); } catch { body = text; }
      if (!r.ok) throw new Error((body && body.error && body.error.message) || r.status + ' ' + text);
      return body;
    }
    function esc(s) { return String(s ?? '').replace(/&/g,'&amp;').replace(/</g,'&lt;'); }
    document.querySelectorAll('.tab').forEach(btn => {
      btn.onclick = () => {
        document.querySelectorAll('.tab').forEach(b => b.classList.remove('active'));
        document.querySelectorAll('.panel').forEach(p => p.classList.remove('active'));
        btn.classList.add('active');
        $('panel-' + btn.dataset.tab).classList.add('active');
      };
    });
    async function loadOrders() {
      $('ordersErr').textContent = '';
      const st = $('orderStatus').value;
      const lim = $('orderLimit').value || '50';
      let q = '/admin/orders?limit=' + encodeURIComponent(lim);
      if (st) q += '&status=' + encodeURIComponent(st);
      const data = await api(q);
      const tb = $('ordersBody'); tb.innerHTML = '';
      (data.orders || []).forEach(o => {
        const tr = document.createElement('tr');
        const ref = o.DisplayRef || o.ID;
        tr.innerHTML = '<td>' + esc(ref) + '</td><td>' + esc(o.Status) + '</td><td>' +
          esc(o.TotalMinorUnits) + ' ' + esc(o.Currency) + '</td><td>' + esc(o.CustomerID) + '</td><td>' +
          esc(o.CreatedAt) + '</td><td><a href="#" data-id="' + esc(o.ID) + '">view</a></td>';
        tr.querySelector('a').onclick = (e) => { e.preventDefault(); loadOrderDetail(o.ID); };
        tb.appendChild(tr);
      });
    }
    async function loadOrderDetail(id) {
      $('orderDetail').textContent = 'Loading…';
      const data = await api('/admin/orders/' + encodeURIComponent(id));
      $('orderDetail').textContent = JSON.stringify(data, null, 2);
    }
    async function loadProducts() {
      $('productsErr').textContent = '';
      const data = await api('/admin/products?limit=100');
      const tb = $('productsBody'); tb.innerHTML = '';
      (data.items || []).forEach(row => {
        const p = row.Product || row.product || row;
        const tr = document.createElement('tr');
        tr.innerHTML = '<td>' + esc(p.SKU || p.sku) + '</td><td>' + esc(p.Name || p.name) + '</td><td>' +
          esc(p.IsActive ?? p.is_active) + '</td><td>' + esc(p.ID || p.id) + '</td>';
        tb.appendChild(tr);
      });
    }
    async function loadInventory() {
      $('inventoryErr').textContent = '';
      let q = '/admin/inventory?limit=200';
      const sku = $('invSku').value.trim();
      if (sku) q += '&sku=' + encodeURIComponent(sku);
      const data = await api(q);
      const tb = $('inventoryBody'); tb.innerHTML = '';
      (data.items || []).forEach(row => {
        const tr = document.createElement('tr');
        tr.innerHTML = '<td>' + esc(row.sku) + '</td><td>' + esc(row.on_hand) + '</td><td>' +
          esc(row.reserved) + '</td><td>' + esc(row.available_qty) + '</td><td>' + esc(row.safety_stock) + '</td>';
        tb.appendChild(tr);
      });
    }
    async function loadPriceHistory() {
      $('pricesErr').textContent = '';
      const id = $('priceProductId').value.trim();
      const tb = $('pricesBody'); tb.innerHTML = '';
      if (!id) { $('pricesErr').textContent = 'enter a product id'; return; }
      try {
        const data = await api('/admin/products/' + encodeURIComponent(id) + '/price-history');
        (data.entries || []).forEach(row => {
          const tr = document.createElement('tr');
          tr.innerHTML = '<td>' + esc(row.currency) + '</td><td>' + esc(row.amount_minor_units) +
            '</td><td>' + esc(row.valid_from || '') + '</td><td>' + esc(row.valid_through || '(open)') +
            '</td><td>' + esc(row.set_by || '') + '</td><td>' + esc(row.set_at || '') +
            '</td><td>' + esc(row.ended_by || '') + '</td><td>' + esc(row.ended_reason || '') + '</td>';
          tb.appendChild(tr);
        });
        if (!(data.entries || []).length) {
          $('pricesErr').textContent = 'no history';
        }
      } catch (e) {
        $('pricesErr').textContent = e.message || String(e);
      }
    }
    async function loadReservations() {
      $('resErr').textContent = '';
      let q = '/admin/inventory/reservations?limit=200';
      const sku = $('resSku').value.trim();
      if (sku) q += '&sku=' + encodeURIComponent(sku);
      const data = await api(q);
      const tb = $('resBody'); tb.innerHTML = '';
      (data.reservations || []).forEach(row => {
        const tr = document.createElement('tr');
        const ord = row.display_ref || row.order_id;
        tr.innerHTML = '<td>' + esc(ord) + '</td><td>' + esc(row.sku) + '</td><td>' + esc(row.qty) +
          '</td><td>' + esc(row.order_status) + '</td><td>' + esc(row.expires_at || '') + '</td>';
        tb.appendChild(tr);
      });
    }
    async function reloadAll() {
      $('status').textContent = 'loading…';
      try {
        await Promise.all([loadOrders(), loadProducts(), loadInventory(), loadReservations()]);
        $('status').textContent = 'ok ' + new Date().toLocaleTimeString();
      } catch (e) {
        $('status').textContent = '';
        $('ordersErr').textContent = e.message || String(e);
      }
    }
    $('reload').onclick = reloadAll;
    $('orderStatus').onchange = loadOrders;
    $('invSku').onchange = loadInventory;
    $('resSku').onchange = loadReservations;
    $('loadPriceHistory').onclick = loadPriceHistory;
    if (tokenEl.value) reloadAll();
  </script>
  <footer style="margin-top:2rem;padding-top:1rem;border-top:1px solid var(--border);font-size:0.8rem;color:var(--muted);">
    Legal: <a href="__PRIVACY_URL__">Privacy</a> · <a href="__DATA_PROCESSING_URL__">Data processing (Ley 1581)</a> · <a href="__CONSUMER_RIGHTS_URL__">Consumer rights (SIC)</a>
  </footer>
</body>
</html>
`

func renderAdminConsoleHTML(privacyURL, consumerRightsURL, dataProcessingURL string) string {
	out := adminConsoleHTML
	out = strings.Replace(out, "__PRIVACY_URL__", html.EscapeString(privacyURL), 1)
	out = strings.Replace(out, "__DATA_PROCESSING_URL__", html.EscapeString(dataProcessingURL), 1)
	out = strings.Replace(out, "__CONSUMER_RIGHTS_URL__", html.EscapeString(consumerRightsURL), 1)
	return out
}

func (s *Server) handleAdminConsole(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	if !s.requireAdminRole(w, r, AdminRoleFinance) {
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	_, _ = w.Write([]byte(renderAdminConsoleHTML(s.PrivacyPolicyURL, s.ConsumerRightsURL, s.DataProcessingPolicyURL)))
}
