const fmtMoney = new Intl.NumberFormat("zh-CN", {
  style: "currency",
  currency: "CNY",
  maximumFractionDigits: 2
});
const fmtNumber = new Intl.NumberFormat("zh-CN");
const curveColors = ["#ff5a72", "#38d996", "#6aa7ff", "#ffcf5a", "#b28cff", "#35d0ba"];

const state = {
  accountId: "",
  range: "20d",
  dashboard: null,
  activity: [],
  closed: [],
  curveHits: [],
  refreshTimer: 0
};

document.addEventListener("DOMContentLoaded", () => {
  setupPanelExpand();
  setupCurveTooltip();
  loadAccount("");
});
window.addEventListener("resize", () => drawEquityCurve(getEquitySeries()));

async function loadAccount(accountId) {
  try {
    const query = accountId ? `&accountId=${encodeURIComponent(accountId)}` : "";
    const dashboard = await fetchData(`/api/paper/dashboard?range=${state.range}${query}`);
    const selectedID = dashboard.selectedAccount?.id || "";
    const activityQuery = selectedID ? `&accountId=${encodeURIComponent(selectedID)}` : "";
    const activity = await fetchData(`/api/paper/activity?limit=80${activityQuery}`);

    state.accountId = selectedID;
    state.dashboard = dashboard;
    state.activity = activity.items || [];
    state.closed = dashboard.closedPositions || [];
    render();
  } catch (error) {
    renderError(error);
  }
}

async function fetchData(url) {
  const response = await fetch(url);
  const body = await response.json();
  if (!response.ok || body.code !== 0) {
    throw new Error(body.message || `请求失败: ${url}`);
  }
  return body.data || {};
}

function render() {
  const dashboard = state.dashboard || {};
  const selectedAccount = dashboard.selectedAccount;
  const accounts = dashboard.accounts || [];
  const accountNames = Object.fromEntries(accounts.map((item) => [item.id, item.name]));

  text("marketStatus", dashboard.marketSnapshot?.statusText || dashboard.marketStatus?.status || "未知");
  text("updatedAt", formatTime(dashboard.updatedAt));
  document.getElementById("emptyState").hidden = accounts.length > 0;

  renderMarketStrip(dashboard.marketSnapshot);
  renderAccounts(accounts, selectedAccount);
  renderMarketOverview(dashboard);
  renderPositions(dashboard.positions || [], accountNames, selectedAccount);
  renderOrdersTrades(dashboard.orders || [], dashboard.trades || [], accountNames, selectedAccount);
  renderClosedPositions(state.closed, accountNames, selectedAccount);
  renderActivity(state.activity, accounts);
  renderCurveRange();
  drawEquityCurve(getEquitySeries());
  scheduleRefresh(dashboard.marketSnapshot);
}

function scheduleRefresh(snapshot) {
  window.clearTimeout(state.refreshTimer);
  const next = snapshot?.nextRefresh ? new Date(snapshot.nextRefresh).getTime() : 0;
  const delay = Number.isFinite(next) && next > Date.now()
    ? Math.max(5000, next - Date.now())
    : 60000;
  state.refreshTimer = window.setTimeout(() => loadAccount(state.accountId), delay);
}

function setupPanelExpand() {
  document.querySelectorAll("[data-expand]").forEach((panel) => {
    const close = document.createElement("button");
    close.type = "button";
    close.className = "panel-close";
    close.textContent = "关闭";
    close.addEventListener("click", (event) => {
      event.stopPropagation();
      panel.classList.remove("expanded");
      document.body.classList.remove("has-expanded-panel");
      drawEquityCurve(getEquitySeries());
    });
    panel.appendChild(close);
    panel.addEventListener("click", (event) => {
      if (panel.classList.contains("expanded")) {
        return;
      }
      if (event.target.closest("button, select, input, textarea, a")) {
        return;
      }
      panel.classList.add("expanded");
      document.body.classList.add("has-expanded-panel");
      drawEquityCurve(getEquitySeries());
    });
  });
  document.addEventListener("keydown", (event) => {
    if (event.key !== "Escape") {
      return;
    }
    document.querySelectorAll(".panel.expanded").forEach((panel) => {
      panel.classList.remove("expanded");
    });
    document.body.classList.remove("has-expanded-panel");
    drawEquityCurve(getEquitySeries());
  });
}

function renderCurveRange() {
  const root = document.getElementById("curveRange");
  root.className = "range-switch";
  const ranges = [
    ["20d", "20日"],
    ["60d", "60日"],
    ["120d", "120日"],
    ["all", "全部"]
  ];
  root.innerHTML = ranges.map(([value, label]) => `
    <button class="${state.range === value ? "active" : ""}" data-range="${value}">
      ${label}
    </button>
  `).join("");
  root.querySelectorAll("button").forEach((button) => {
    button.addEventListener("click", () => {
      state.range = button.dataset.range;
      loadAccount(state.accountId);
    });
  });
}

function renderAccounts(accounts, selectedAccount) {
  const root = document.getElementById("accountCards");
  root.innerHTML = accounts.map((account) => {
    const isSelected = account.id === selectedAccount?.id;
    const totalCash = account.availableCash + account.frozenCash;
    return `
      <article class="account-card ${isSelected ? "selected" : ""}" data-account-id="${escapeHtml(account.id)}">
        <div class="name">
          <span>${escapeHtml(account.name)}</span>
          <span class="pill">${isSelected ? "当前" : "筛选"}</span>
        </div>
        <div class="cash">${fmtMoney.format(totalCash)}</div>
        <div class="subline">
          可用 ${fmtMoney.format(account.availableCash)}
          · 冻结 ${fmtMoney.format(account.frozenCash)}
        </div>
      </article>
    `;
  }).join("");
  root.querySelectorAll(".account-card").forEach((card) => {
    card.addEventListener("click", () => {
      const nextID = card.dataset.accountId === state.accountId ? "" : card.dataset.accountId;
      loadAccount(nextID);
    });
  });
}

function renderMarketStrip(snapshot) {
  const root = document.getElementById("marketStrip");
  const indexes = snapshot?.indexes || [];
  const breadth = snapshot?.breadth || {};
  const statusText = snapshot?.statusText || "未知";
  const note = snapshot?.note || "市场快照暂不可用";
  const breadthTotal = breadth.total || 0;
  const riseWidth = breadthTotal ? Math.round((breadth.rising || 0) / breadthTotal * 100) : 0;
  const fallWidth = breadthTotal ? Math.round((breadth.falling || 0) / breadthTotal * 100) : 0;

  root.innerHTML = `
    <div class="market-strip-head">
      <div>
        <p class="eyebrow">MARKET SNAPSHOT</p>
        <h2>主要市场行情</h2>
      </div>
      <div class="market-session">
        <strong>${escapeHtml(statusText)}</strong>
        <span>${escapeHtml(note)}</span>
      </div>
    </div>
    <div class="index-tape">
      ${indexes.length ? indexes.map(renderIndexTile).join("") : `
        <div class="index-tile muted">指数行情暂不可用</div>
      `}
    </div>
    <div class="breadth-card">
      <div class="breadth-main">
        <span>全市场涨跌</span>
        <strong>
          <b class="good">${fmtNumber.format(breadth.rising || 0)}</b>
          /
          <b class="bad">${fmtNumber.format(breadth.falling || 0)}</b>
          /
          <b>${fmtNumber.format(breadth.flat || 0)}</b>
        </strong>
      </div>
      <div class="breadth-bar" aria-label="上涨下跌比例">
        <i class="rise" style="width:${riseWidth}%"></i>
        <i class="fall" style="width:${fallWidth}%"></i>
      </div>
      <div class="breadth-sub">
        <span>涨停约 ${fmtNumber.format(breadth.limitUp || 0)}</span>
        <span>跌停约 ${fmtNumber.format(breadth.limitDown || 0)}</span>
        <span>上涨占比 ${formatPercent(breadth.risingPct || 0)}</span>
      </div>
    </div>
  `;
}

function renderIndexTile(item) {
  const tone = item.changePct >= 0 ? "good" : "bad";
  return `
    <div class="index-tile">
      <span>${escapeHtml(item.name)}</span>
      <strong>${formatFixed(item.close, 2)}</strong>
      <em class="${tone}">${formatPercent(item.changePct || 0)}</em>
    </div>
  `;
}

function renderMarketOverview(dashboard) {
  const account = dashboard.selectedAccount;
  const positions = dashboard.positions || [];
  const orders = dashboard.orders || [];
  const trades = dashboard.trades || [];
  const snapshot = dashboard.marketSnapshot || {};
  const values = [
    ["账户范围", account?.name || "全部账户", account ? "" : "warn"],
    ["市场状态", snapshot.statusText || "未知", "warn"],
    ["持仓标的", `${positions.length} 个`, ""],
    ["待处理委托", `${orders.filter((item) => item.status === "pending").length} 笔`, ""],
    ["成交记录", `${trades.length} 笔`, ""]
  ];

  document.getElementById("marketOverview").innerHTML = `
    ${values.map(([label, value, tone]) => `
      <div class="metric">
        <span class="muted">${label}</span>
        <strong class="${tone}">${escapeHtml(value)}</strong>
      </div>
    `).join("")}
  `;
}

function renderPositions(items, accountNames, selectedAccount) {
  text("positionCount", items.length);
  const body = document.getElementById("positionsBody");
  if (!items.length) {
    body.innerHTML = `<tr><td colspan="4" class="empty-line">暂无持仓</td></tr>`;
    return;
  }

  body.innerHTML = items.map((item) => `
    <tr>
      <td>${escapeHtml(item.name || item.code)}<div class="muted">${positionMeta(item, accountNames, selectedAccount)}</div></td>
      <td>${fmtNumber.format(item.quantity)}</td>
      <td>${fmtNumber.format(item.sellableQuantity)}</td>
      <td>${fmtMoney.format(item.avgCost)}</td>
    </tr>
  `).join("");
}

function renderOrdersTrades(orders, trades, accountNames, selectedAccount) {
  const latest = [
    ...orders.map((item) => ({ type: "委托", at: item.updatedAt, item })),
    ...trades.map((item) => ({ type: "成交", at: item.tradedAt, item }))
  ].sort((a, b) => String(b.at).localeCompare(String(a.at))).slice(0, 8);

  document.getElementById("ordersTrades").innerHTML = latest.length
    ? latest.map((entry) => renderOrderTradeRow(entry, accountNames, selectedAccount)).join("")
    : `<div class="empty-line">暂无委托或成交</div>`;
}

function renderOrderTradeRow(entry, accountNames, selectedAccount) {
  const item = entry.item;
  const side = item.side === "buy" ? "买入" : "卖出";
  const tone = item.side === "buy" ? "good" : "bad";
  const amount = item.amount || item.price * item.quantity;
  return `
    <div class="row">
      <div>
        <strong>${entry.type} · ${escapeHtml(item.name || item.code)}</strong>
        <div class="meta">${orderTradeMeta(entry, accountNames, selectedAccount)}</div>
      </div>
      <div class="${tone}">${side} ${fmtMoney.format(amount || 0)}</div>
    </div>
  `;
}

function renderClosedPositions(items, accountNames, selectedAccount) {
  document.getElementById("closedPositions").innerHTML = items.length
    ? items.slice(0, 8).map((item) => {
      const tone = item.realizedPnl >= 0 ? "good" : "bad";
      return `
        <div class="row">
          <div>
            <strong>${escapeHtml(item.name || item.code)}</strong>
            <div class="meta">${closedPositionMeta(item, accountNames, selectedAccount)}</div>
          </div>
          <div class="${tone}">${fmtMoney.format(item.realizedPnl)}</div>
        </div>
      `;
    }).join("")
    : `<div class="empty-line">暂无清仓记录</div>`;
}

function positionMeta(item, accountNames, selectedAccount) {
  const account = selectedAccount ? "" : ` · ${accountNames[item.accountId] || item.accountId || "--"}`;
  return `${item.code}${account}`;
}

function orderTradeMeta(entry, accountNames, selectedAccount) {
  const item = entry.item;
  const account = selectedAccount ? "" : ` · ${accountNames[item.accountId] || item.accountId || "--"}`;
  return `${formatTime(entry.at)} · ${item.code} · ${item.status || "filled"}${account}`;
}

function closedPositionMeta(item, accountNames, selectedAccount) {
  const account = selectedAccount ? "" : ` · ${accountNames[item.accountId] || item.accountId || "--"}`;
  return `${item.openedAt || "--"} → ${item.closedAt}${account}`;
}

function renderActivity(items, accounts) {
  const accountNames = Object.fromEntries(accounts.map((item) => [item.id, item.name]));
  text("activityCount", items.length);
  document.getElementById("activityTimeline").innerHTML = items.length
    ? items.slice(0, 12).map((item) => `
      <li>
        <strong>${escapeHtml(describeActivity(item))}</strong>
        <span class="muted">${formatTime(item.createdAt)} · ${escapeHtml(accountNames[item.accountId] || item.accountId || "--")}</span>
      </li>
    `).join("")
    : `<li class="empty-line">暂无 Agent 行为</li>`;
}

function getEquitySeries() {
  const dashboard = state.dashboard || {};
  if (Array.isArray(dashboard.equityCurves) && dashboard.equityCurves.length) {
    return dashboard.equityCurves.filter((item) => item.points?.length);
  }
  if (dashboard.equityCurve?.length) {
    return [{
      accountId: dashboard.selectedAccount?.id || "selected",
      accountName: dashboard.selectedAccount?.name || "当前账户",
      points: dashboard.equityCurve
    }];
  }
  return [];
}

function drawEquityCurve(series) {
  const canvas = document.getElementById("equityCanvas");
  const days = getCurveDays(series);
  const containerWidth = canvas.parentElement.clientWidth || canvas.getBoundingClientRect().width;
  const logicalWidth = days.length > 60
    ? Math.max(containerWidth, days.length * 14 + 48)
    : containerWidth;
  canvas.style.width = `${logicalWidth}px`;
  const scale = window.devicePixelRatio || 1;
  canvas.width = Math.max(1, Math.floor(logicalWidth * scale));
  canvas.height = Math.floor(300 * scale);

  const ctx = canvas.getContext("2d");
  ctx.scale(scale, scale);
  ctx.clearRect(0, 0, logicalWidth, 300);
  ctx.strokeStyle = "#243140";
  ctx.lineWidth = 1;

  const plot = {
    left: 18,
    right: 14,
    top: 28,
    bottom: 238,
    axisY: 258,
    labelY: 284
  };
  for (let i = 0; i < 5; i++) {
    const y = plot.top + i * ((plot.bottom - plot.top) / 4);
    ctx.beginPath();
    ctx.moveTo(0, y);
    ctx.lineTo(logicalWidth, y);
    ctx.stroke();
  }

  renderCurveLegend(series);
  if (!series.length) {
    ctx.fillStyle = "#8795a5";
    ctx.font = "13px Microsoft YaHei, sans-serif";
    ctx.textAlign = "center";
    ctx.fillText("暂无资产曲线", logicalWidth / 2, 150);
    return;
  }

  const values = series.flatMap((item) => dailyCurvePoints(item.points).map(
    (point) => point.totalAssets
  ));
  const min = Math.min(...values);
  const max = Math.max(...values);
  const span = max - min || 1;
  const width = logicalWidth - plot.left - plot.right;
  const height = plot.bottom - plot.top;

  drawCurveXAxis(ctx, days, plot, width);
  state.curveHits = [];

  series.forEach((item, seriesIndex) => {
    const pointsByDay = Object.fromEntries(dailyCurvePoints(item.points).map((point) => [
      point.tradingDay,
      point
    ]));
    ctx.strokeStyle = curveColors[seriesIndex % curveColors.length];
    ctx.lineWidth = 2;
    ctx.beginPath();
    let started = false;
    days.forEach((day, index) => {
      const point = pointsByDay[day];
      if (!point) {
        return;
      }
      const x = plot.left + (width * index) / Math.max(1, days.length - 1);
      const y = plot.bottom - ((point.totalAssets - min) / span) * height;
      state.curveHits.push({
        x,
        y,
        color: curveColors[seriesIndex % curveColors.length],
        accountName: item.accountName || item.accountId,
        point
      });
      if (!started) {
        ctx.moveTo(x, y);
        started = true;
      } else {
        ctx.lineTo(x, y);
      }
    });
    ctx.stroke();
    for (const hit of state.curveHits.filter((hit) => hit.accountName === (item.accountName || item.accountId))) {
      ctx.fillStyle = hit.color;
      ctx.beginPath();
      ctx.arc(hit.x, hit.y, 2.5, 0, Math.PI * 2);
      ctx.fill();
    }
  });

  ctx.fillStyle = "#e7edf4";
  ctx.font = "12px Microsoft YaHei, sans-serif";
  ctx.textAlign = "left";
  ctx.fillText(`最高 ${fmtMoney.format(max)}`, 14, 20);
  ctx.fillText(`最低 ${fmtMoney.format(min)}`, 14, 250);
}

function getCurveDays(series) {
  return Array.from(new Set(
    series.flatMap((item) => dailyCurvePoints(item.points).map((point) => point.tradingDay))
  )).sort();
}

function dailyCurvePoints(points) {
  const byDay = new Map();
  for (const point of points || []) {
    if (point.tradingDay) {
      byDay.set(point.tradingDay, point);
    }
  }
  return Array.from(byDay.values()).sort((a, b) => a.tradingDay.localeCompare(b.tradingDay));
}

function drawCurveXAxis(ctx, days, plot, width) {
  ctx.strokeStyle = "#2b3a4a";
  ctx.fillStyle = "#8795a5";
  ctx.font = "11px Microsoft YaHei, sans-serif";
  ctx.textAlign = "center";
  const labelStep = Math.max(1, Math.ceil(days.length / Math.max(1, Math.floor(width / 72))));
  days.forEach((day, index) => {
    const x = plot.left + (width * index) / Math.max(1, days.length - 1);
    ctx.beginPath();
    ctx.moveTo(x, plot.axisY - 4);
    ctx.lineTo(x, plot.axisY + 4);
    ctx.stroke();
    if (index % labelStep === 0 || index === days.length - 1) {
      ctx.fillText(formatDayLabel(day), x, plot.labelY);
    }
  });
  ctx.beginPath();
  ctx.moveTo(plot.left, plot.axisY);
  ctx.lineTo(plot.left + width, plot.axisY);
  ctx.stroke();
}

function formatDayLabel(day) {
  const [, month, date] = String(day).split("-");
  return month && date ? `${month}/${date}` : day;
}

function renderCurveLegend(series) {
  const canvas = document.getElementById("equityCanvas");
  let legend = document.getElementById("equityLegend");
  if (!legend) {
    legend = document.createElement("div");
    legend.id = "equityLegend";
    legend.className = "curve-legend";
    canvas.insertAdjacentElement("afterend", legend);
  }
  legend.hidden = !series.length;
  legend.innerHTML = series.map((item, index) => `
    <span>
      <i style="background:${curveColors[index % curveColors.length]}"></i>
      ${escapeHtml(item.accountName || item.accountId)}
    </span>
  `).join("");
}

function setupCurveTooltip() {
  const canvas = document.getElementById("equityCanvas");
  const tooltip = document.getElementById("curveTooltip");
  if (!canvas || !tooltip) {
    return;
  }
  canvas.addEventListener("mousemove", (event) => {
    const rect = canvas.getBoundingClientRect();
    const x = event.clientX - rect.left;
    const y = event.clientY - rect.top;
    const hit = nearestCurveHit(x, y);
    if (!hit) {
      tooltip.hidden = true;
      drawEquityCurve(getEquitySeries());
      return;
    }
    drawEquityCurve(getEquitySeries());
    drawCurveHoverDot(hit);
    tooltip.hidden = false;
    tooltip.style.left = `${hit.x + 14}px`;
    tooltip.style.top = `${Math.max(8, hit.y - 46)}px`;
    tooltip.innerHTML = `
      <strong>${escapeHtml(hit.accountName)} · ${formatDayLabel(hit.point.tradingDay)}</strong>
      <span>总资产 ${fmtMoney.format(hit.point.totalAssets)}</span>
      <span>${escapeHtml(formatTradeSummary(hit.point))}</span>
    `;
  });
  canvas.addEventListener("mouseleave", () => {
    tooltip.hidden = true;
    drawEquityCurve(getEquitySeries());
  });
}

function nearestCurveHit(x, y) {
  let best = null;
  let bestDistance = Infinity;
  for (const hit of state.curveHits) {
    const distance = Math.hypot(hit.x - x, hit.y - y);
    if (distance < bestDistance) {
      best = hit;
      bestDistance = distance;
    }
  }
  return bestDistance <= 10 ? best : null;
}

function drawCurveHoverDot(hit) {
  const canvas = document.getElementById("equityCanvas");
  const ctx = canvas.getContext("2d");
  ctx.strokeStyle = "#ffffff";
  ctx.fillStyle = hit.color;
  ctx.lineWidth = 2;
  ctx.beginPath();
  ctx.arc(hit.x, hit.y, 6, 0, Math.PI * 2);
  ctx.fill();
  ctx.stroke();
}

function formatTradeSummary(point) {
  const parts = [];
  if (point.buyQuantity > 0) {
    parts.push(`买入 ${fmtNumber.format(point.buyQuantity)} 股 / ${fmtMoney.format(point.buyAmount)}`);
  }
  if (point.sellQuantity > 0) {
    parts.push(`卖出 ${fmtNumber.format(point.sellQuantity)} 股 / ${fmtMoney.format(point.sellAmount)}`);
  }
  return parts.length ? parts.join("；") : "当日无成交";
}

function formatPercent(value) {
  const number = Number(value || 0);
  const sign = number > 0 ? "+" : "";
  return `${sign}${number.toFixed(2)}%`;
}

function formatFixed(value, digits) {
  const number = Number(value || 0);
  return number.toFixed(digits);
}

function describeActivity(item) {
  const request = parseJSON(item.request);
  const response = parseJSON(item.response);
  if (item.actionType === "create_account") {
    const positions = request.initialPositions || [];
    return `创建账户 ${request.name || ""}，初始资金 ${fmtMoney.format(request.initialCash || 0)}，初始持仓 ${positions.length} 只`;
  }
  if (item.actionType === "place_order") {
    const side = request.side === "sell" ? "卖出" : "买入";
    const name = request.name || request.code || response.name || response.code || "";
    const price = request.price ? `，委托价 ${fmtMoney.format(request.price)}` : "";
    const reason = request.reason ? `，理由：${request.reason}` : "";
    return `提交委托：${side} ${name} ${fmtNumber.format(request.quantity || 0)} 股${price}${reason}`;
  }
  if (item.actionType === "fill_order") {
    const side = response.side === "sell" ? "卖出成交" : "买入成交";
    const name = response.name || response.code || request.name || request.code || "";
    return `${side}：${name} ${fmtNumber.format(response.quantity || 0)} 股，成交额 ${fmtMoney.format(response.amount || 0)}`;
  }
  if (item.actionType === "cancel_order") {
    return `撤销委托：${request.orderId || ""}`;
  }
  return item.actionType || "账户行为";
}

function parseJSON(value) {
  if (!value) {
    return {};
  }
  try {
    return JSON.parse(value);
  } catch {
    return {};
  }
}

function renderError(error) {
  document.getElementById("emptyState").hidden = false;
  document.getElementById("emptyState").innerHTML = `
    <p class="eyebrow">LOAD FAILED</p>
    <h2>看板数据加载失败</h2>
    <p>${escapeHtml(error.message)}</p>
  `;
  drawEquityCurve([]);
}

function text(id, value) {
  document.getElementById(id).textContent = value;
}

function formatTime(value) {
  if (!value) {
    return "--";
  }
  const date = new Date(value);
  if (Number.isNaN(date.getTime())) {
    return value;
  }
  return date.toLocaleString("zh-CN", { hour12: false });
}

function escapeHtml(value) {
  return String(value ?? "").replace(/[&<>"']/g, (char) => ({
    "&": "&amp;",
    "<": "&lt;",
    ">": "&gt;",
    '"': "&quot;",
    "'": "&#39;"
  }[char]));
}
