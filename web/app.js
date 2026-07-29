const elements = {
  market: document.querySelector('#market-select'),
  connectionDot: document.querySelector('#connection-dot'),
  connectionText: document.querySelector('#connection-text'),
  updated: document.querySelector('#last-updated'),
  mid: document.querySelector('#mid-price'),
  quoteUnit: document.querySelector('#quote-unit'),
  severity: document.querySelector('#severity-badge'),
  bid: document.querySelector('#best-bid'),
  ask: document.querySelector('#best-ask'),
  weighted: document.querySelector('#weighted-mid'),
  spread: document.querySelector('#spread'),
  imbalance: document.querySelector('#imbalance'),
  volatility: document.querySelector('#volatility'),
  feedAge: document.querySelector('#feed-age'),
  reasons: document.querySelector('#risk-reasons'),
  eventCount: document.querySelector('#event-count'),
  spreadCard: document.querySelector('#spread-card'),
  imbalanceCard: document.querySelector('#imbalance-card'),
  volCard: document.querySelector('#vol-card'),
  shockCard: document.querySelector('#shock-card'),
  chartLine: document.querySelector('#chart-line'),
  chartArea: document.querySelector('#chart-area'),
  orderForm: document.querySelector('#paper-order-form'),
  orderResult: document.querySelector('#order-result'),
  orderButton: document.querySelector('#paper-order-form button[type="submit"]'),
};

let source;
let priceHistory = [];
let currentMarket = 'btctwd';

const fmtPrice = (value) => Number(value || 0).toLocaleString('zh-TW', { maximumFractionDigits: 2 });
const fmt = (value, digits = 2) => Number(value || 0).toFixed(digits);

const marketLabels = {
  btctwd: 'BTC / TWD',
  btcusd: 'BTC / USD',
  ethtwd: 'ETH / TWD',
};

const quoteCurrencies = ['USDT', 'USDC', 'TWD', 'USD', 'BTC', 'ETH'];

async function loadMarkets() {
  try {
    const response = await fetch('/api/v1/markets', { cache: 'no-store' });
    if (!response.ok) throw new Error(`market list request failed: ${response.status}`);
    const data = await response.json();
    const markets = data.markets?.length ? data.markets : ['btctwd'];
    const options = markets.map(market => {
      const option = document.createElement('option');
      option.value = market;
      option.textContent = formatMarket(market);
      return option;
    });
    elements.market.replaceChildren(...options);
    connect(markets[0]);
  } catch (error) {
    setConnection(false, 'API unavailable');
  }
}

function marketParts(market) {
  const upper = String(market || '').toUpperCase();
  const quote = quoteCurrencies.find(candidate => upper.endsWith(candidate) && upper.length > candidate.length);
  return quote ? { base: upper.slice(0, -quote.length), quote } : { base: upper, quote: '' };
}

function formatMarket(market) {
  const normalized = String(market || '').trim().toLowerCase();
  if (marketLabels[normalized]) return marketLabels[normalized];
  const { base, quote } = marketParts(normalized);
  return quote ? `${base} / ${quote}` : base;
}

function formatNotional(value, market) {
  const quote = marketParts(market).quote;
  const formatted = fmtPrice(value);
  if (quote === 'TWD') return `NT$${formatted}`;
  if (quote === 'USD') return `US$${formatted}`;
  return quote ? `${quote} ${formatted}` : formatted;
}

function isSyntheticReferenceMarket(market) {
  return String(market || '').toLowerCase() === 'btcusd';
}

function updateOrderAvailability(market) {
  const disabled = isSyntheticReferenceMarket(market);
  elements.orderForm.querySelectorAll('input, select, button').forEach(control => {
    control.disabled = disabled;
  });
  if (disabled) {
    elements.orderResult.className = 'order-result idle';
    elements.orderResult.innerHTML = '<strong>Reference market only</strong><p>BTC / USD is a synthetic demo reference. Paper orders remain available for MAX-native markets.</p>';
  } else {
    elements.orderResult.className = 'order-result idle';
    elements.orderResult.innerHTML = '<strong>Ready for simulation</strong><p>The gateway checks feed freshness, severity, notional size, price distance, and input validity.</p>';
  }
}

function connect(market) {
  source?.close();
  currentMarket = market;
  priceHistory = [];
  elements.quoteUnit.textContent = marketParts(market).quote || 'QUOTE';
  elements.market.setAttribute('aria-label', `Selected market: ${formatMarket(market)}`);
  updateOrderAvailability(market);
  setConnection(false, 'Connecting');
  source = new EventSource(`/api/v1/stream?market=${encodeURIComponent(market)}`);
  source.addEventListener('risk.snapshot', event => {
    const snapshot = JSON.parse(event.data);
    render(snapshot);
    setConnection(true, 'Simulated feed connected');
  });
  source.onerror = () => setConnection(false, 'Reconnecting');
}

function setConnection(live, text) {
  elements.connectionDot.className = live ? 'live' : 'offline';
  elements.connectionText.textContent = text;
}

function render(s) {
  currentMarket = s.market || currentMarket;
  elements.quoteUnit.textContent = marketParts(currentMarket).quote || 'QUOTE';
  elements.mid.textContent = fmtPrice(s.mid_price);
  elements.bid.textContent = fmtPrice(s.best_bid);
  elements.ask.textContent = fmtPrice(s.best_ask);
  elements.weighted.textContent = fmtPrice(s.weighted_mid_price);
  elements.spread.textContent = fmt(s.spread_bps);
  elements.imbalance.textContent = fmt(s.book_imbalance, 3);
  elements.volatility.textContent = fmt(s.realized_volatility, 4);
  elements.feedAge.textContent = fmt(s.feed_age_seconds, 1);
  elements.spreadCard.textContent = `${fmt(s.spread_bps)} bps`;
  elements.imbalanceCard.textContent = fmt(s.book_imbalance, 3);
  elements.volCard.textContent = fmt(s.realized_volatility, 4);
  elements.shockCard.textContent = `${fmt(s.price_shock_bps)} bps`;
  elements.eventCount.textContent = `${Number(s.event_count || 0).toLocaleString()} events`;
  elements.updated.textContent = `Updated ${new Date(s.observed_at).toLocaleTimeString('zh-TW')}`;
  elements.severity.textContent = s.severity;
  elements.severity.className = `badge ${String(s.severity).toLowerCase().replace('warning', 'warning')}`;
  elements.reasons.innerHTML = (s.reasons || []).map(reason => `<li>${escapeHTML(reason)}</li>`).join('');
  priceHistory.push(Number(s.mid_price));
  if (priceHistory.length > 60) priceHistory.shift();
  drawChart(priceHistory);
  if (!document.querySelector('#order-price').value) {
    document.querySelector('#order-price').placeholder = fmtPrice(s.mid_price);
  }
}

function drawChart(values) {
  if (values.length < 2) return;
  const width = 800, height = 250, pad = 8;
  const min = Math.min(...values), max = Math.max(...values);
  const span = Math.max(max - min, max * 0.0002, 1);
  const points = values.map((value, i) => {
    const x = pad + (i / (values.length - 1)) * (width - pad * 2);
    const y = pad + (1 - (value - min) / span) * (height - pad * 2);
    return [x, y];
  });
  const line = points.map(([x, y], i) => `${i ? 'L' : 'M'}${x.toFixed(2)},${y.toFixed(2)}`).join(' ');
  const area = `${line} L${points.at(-1)[0]},${height} L${points[0][0]},${height} Z`;
  elements.chartLine.setAttribute('d', line);
  elements.chartArea.setAttribute('d', area);
}

function escapeHTML(value) {
  const node = document.createElement('span');
  node.textContent = value;
  return node.innerHTML;
}

elements.market.addEventListener('change', event => connect(event.target.value));

elements.orderForm.addEventListener('submit', async event => {
  event.preventDefault();
  if (isSyntheticReferenceMarket(currentMarket)) return;
  const payload = {
    market: elements.market.value,
    side: document.querySelector('#order-side').value,
    quantity: document.querySelector('#order-quantity').value,
    limit_price: document.querySelector('#order-price').value,
  };
  elements.orderResult.className = 'order-result idle';
  elements.orderResult.innerHTML = '<strong>Evaluating…</strong><p>Applying deterministic pre-trade checks.</p>';
  try {
    const response = await fetch('/api/v1/paper/orders', {
      method: 'POST',
      headers: { 'content-type': 'application/json' },
      body: JSON.stringify(payload),
    });
    const result = await response.json();
    const accepted = Boolean(result.accepted);
    elements.orderResult.className = `order-result ${accepted ? 'accepted' : 'rejected'}`;
    elements.orderResult.innerHTML = `<strong>${accepted ? 'Accepted for paper simulation' : 'Rejected by guardrails'}</strong>
      <p>${(result.reasons || []).map(escapeHTML).join(' · ')}<br />Estimated notional: ${formatNotional(result.estimated_notional_twd, result.market || currentMarket)}</p>`;
  } catch (error) {
    elements.orderResult.className = 'order-result rejected';
    elements.orderResult.innerHTML = `<strong>Request failed</strong><p>${escapeHTML(error.message)}</p>`;
  }
});

loadMarkets();
