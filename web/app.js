const form = document.getElementById("model");
let seq = 0;

function trim(n, prec) {
  return String(Number(Number(n).toFixed(prec)));
}

function bw(gbps) {
  const sign = gbps < 0 ? "-" : "";
  const v = Math.abs(gbps);
  if (v >= 1000) return sign + trim(v / 1000, 2) + " TB/s";
  if (v >= 10) return sign + trim(v, 1) + " GB/s";
  if (v >= 1) return sign + trim(v, 2) + " GB/s";
  return sign + trim(v * 1000, 1) + " MB/s";
}

function capTB(tb) {
  if (tb >= 1e6) return (tb / 1e6).toFixed(2) + " EB";
  if (tb >= 1000) return (tb / 1000).toFixed(2) + " PB";
  return tb.toFixed(1) + " TB";
}

function pb(v) {
  const sign = v < 0 ? "-" : "";
  const a = Math.abs(v);
  if (a >= 1000) return sign + trim(a / 1000, 2) + " EB";
  if (a >= 10) return sign + trim(a, 1) + " PB";
  return sign + trim(a, 2) + " PB";
}

function pct(frac) {
  return (frac * 100).toFixed(1).replace(/\.0$/, "") + "%";
}

function factor(v) {
  if (!Number.isFinite(v)) return "—";
  const shown = Math.abs(v - Math.round(v)) < 1e-9 ? String(Math.round(v)) : trim(v, 2);
  return shown + "×";
}

function intish(n) {
  return Math.round(n).toLocaleString("en-US");
}

function iops(n) {
  if (!Number.isFinite(n) || n <= 0) return "0";
  return intish(n);
}

function filesRate(n) {
  const v = Number(n);
  if (!Number.isFinite(v) || v <= 0) return "0 /s";
  if (v >= 100) return intish(v) + " /s";
  if (v >= 10) return trim(v, 1) + " /s";
  return trim(v, 2) + " /s";
}

function iopsCompact(n) {
  const v = Number(n);
  if (!Number.isFinite(v) || v <= 0) return "0";
  if (v >= 1e9) return trim(v / 1e9, 2) + " billion";
  if (v >= 1e6) return trim(v / 1e6, 2) + " million";
  return intish(v);
}

function chf(v) {
  const n = Number(v);
  if (!Number.isFinite(n)) return "CHF 0";
  const sign = n < 0 ? "-" : "";
  const a = Math.abs(n);
  if (a >= 1e9) return sign + "CHF " + trim(a / 1e9, 2) + " billion";
  if (a >= 1e6) return sign + "CHF " + trim(a / 1e6, 2) + " million";
  return sign + "CHF " + intish(Math.round(a));
}

function watts(w) {
  const n = Number(w);
  if (!Number.isFinite(n)) return "0 W";
  const sign = n < 0 ? "-" : "";
  const a = Math.abs(n);
  if (a >= 1e6) return sign + trim(a / 1e6, 2) + " MW";
  if (a >= 1000) return sign + trim(a / 1000, 1) + " kW";
  return sign + trim(a, 0) + " W";
}

function hours(h) {
  if (!Number.isFinite(h) || h <= 0) return "—";
  if (h >= 48) return trim(h / 24, 1) + " days";
  return trim(h, 1) + " hours";
}

function mbps(v) {
  if (v >= 1000) return trim(v / 1000, 2) + " GB/s";
  return trim(v, 1) + " MB/s";
}

function sentence(s) {
  if (!s) return "";
  return s.charAt(0).toUpperCase() + s.slice(1);
}

function el(tag, className, text) {
  const node = document.createElement(tag);
  if (className) node.className = className;
  if (text != null) node.textContent = text;
  return node;
}

function setNum(id, value, digits) {
  const p = 10 ** (digits ?? 6);
  const n = Math.round(Number(value) * p) / p;
  document.getElementById(id).value = String(n);
}

function setRadio(name, value) {
  const input = document.querySelector(`input[name="${name}"][value="${value}"]`);
  if (input) input.checked = true;
}

function num(id) {
  const raw = document.getElementById(id).value.trim();
  if (raw === "") return null;
  const n = Number(raw);
  return Number.isFinite(n) ? n : null;
}

function radio(name) {
  const input = document.querySelector(`input[name="${name}"]:checked`);
  if (!input) return null;
  const n = Number(input.value);
  return Number.isFinite(n) ? n : null;
}

function choice(name) {
  const input = document.querySelector(`input[name="${name}"]:checked`);
  return input ? input.value : null;
}

function fill(cfg) {
  setNum("nvme-nodes", cfg.nvme.nodes, 0);
  setRadio("nvme-net", cfg.nvme.networkGbps);
  setNum("nvme-drives", cfg.nvme.drivesPerNode, 0);
  setNum("nvme-size", cfg.nvme.driveSizeTB, 4);
  setNum("nvme-bw", cfg.nvme.driveBWGBps, 4);
  setNum("nvme-iops", cfg.nvme.driveIOPS, 0);
  setRadio("hdd-layout", cfg.layout || "ec10p2");
  setNum("hdd-nodes", cfg.hdd.nodes, 0);
  setRadio("hdd-net", cfg.hdd.networkGbps);
  setNum("hdd-drives", cfg.hdd.drivesPerNode, 0);
  setNum("hdd-size", cfg.hdd.driveSizeTB, 4);
  setNum("hdd-bw-mb", cfg.hdd.driveBWGBps * 1000, 4);
  setNum("hdd-iops", cfg.hdd.driveIOPS, 0);
  setNum("read-streams", cfg.stream.readStreams, 0);
  setNum("write-streams", cfg.stream.writeStreams, 0);
  const bootDrives = cfg.hdd.nodes * cfg.hdd.drivesPerNode;
  const bootPer = bootDrives > 0 ? (cfg.stream.readStreams + cfg.stream.writeStreams) / bootDrives : 1;
  document.getElementById("streams-slider").value = String(bootPer);
  setStreamReadout(bootPer);
  setNum("segment-mb", cfg.stream.segmentMB, 4);
  setNum("seek-ms", cfg.stream.seekMs, 4);
  setNum("tape-drives", cfg.tape.drives, 0);
  setNum("tape-bw-mb", cfg.tape.driveBWGBps * 1000, 4);
  setNum("tape-capacity", cfg.tape.capacityEB, 4);
  setNum("file-size", cfg.workload.fileSizeGB, 4);
  setNum("compute-read", cfg.workload.computeReadGBps, 4);
  setNum("compute-write", cfg.workload.computeWriteGBps, 4);
  setNum("nvme-hit", cfg.workload.nvmeHitRate * 100, 4);
  setNum("nvme-through", (cfg.workload.nvmeThroughFraction || 0) * 100, 4);
  setNum("nvme-reread", cfg.workload.nvmeRereadFactor || 0, 4);
  setNum("working-set", cfg.workload.workingSetPB, 4);
  setNum("reserve-pct", cfg.workload.reserveFraction * 100, 4);
  setNum("archive-eb", cfg.workload.archiveEBPerYear, 4);
  setNum("recall", cfg.workload.recallGBps, 4);
  setNum("baseline-eb", cfg.workload.baselineEB, 4);
  setNum("observed-scale", cfg.workload.observedTBpsPerEB, 4);
  setNum("prefetch-hours", cfg.workload.prefetchHorizonHours, 4);
  const prices = cfg.prices || {};
  setNum("price-node", prices.nodeCHF, 2);
  setNum("price-nvme", prices.nvmeCHFPerTB, 2);
  setNum("price-hdd", prices.hddCHFPerTB, 2);
  setNum("price-tape-drive", prices.tapeDriveCHF, 2);
  setNum("price-tape", prices.tapeCHFPerTB, 2);
  const power = cfg.power || {};
  setNum("power-node", power.nodeW, 2);
  setNum("power-nvme", power.nvmeDriveW, 2);
  setNum("power-hdd", power.hddDriveW, 2);
  setNum("power-tape", power.tapeDriveW, 2);
  setHybrid(Boolean(cfg.hybrid));
  setRepack(Boolean(cfg.repack));
}

function readConfig() {
  const ids = [
    "nvme-nodes", "nvme-drives", "nvme-size", "nvme-bw", "nvme-iops",
    "hdd-nodes", "hdd-drives", "hdd-size", "hdd-bw-mb", "hdd-iops",
    "read-streams", "write-streams", "segment-mb", "seek-ms",
    "tape-drives", "tape-bw-mb", "tape-capacity",
    "file-size", "compute-read", "compute-write", "nvme-hit", "nvme-through", "nvme-reread", "prefetch-hours",
    "working-set", "reserve-pct", "baseline-eb", "observed-scale",
    "archive-eb", "recall",
    "price-node", "price-nvme", "price-hdd", "price-tape-drive", "price-tape",
    "power-node", "power-nvme", "power-hdd", "power-tape",
  ];
  const v = {};
  for (const id of ids) {
    const n = num(id);
    if (n === null) return null;
    v[id] = n;
  }
  const nvmeNet = radio("nvme-net");
  const hddNet = radio("hdd-net");
  const layout = choice("hdd-layout");
  if (nvmeNet === null || hddNet === null || !layout) return null;
  return {
    nvme: {
      nodes: v["nvme-nodes"],
      networkGbps: nvmeNet,
      drivesPerNode: v["nvme-drives"],
      driveSizeTB: v["nvme-size"],
      driveBWGBps: v["nvme-bw"],
      driveIOPS: v["nvme-iops"],
    },
    hdd: {
      nodes: v["hdd-nodes"],
      networkGbps: hddNet,
      drivesPerNode: v["hdd-drives"],
      driveSizeTB: v["hdd-size"],
      driveBWGBps: v["hdd-bw-mb"] / 1000,
      driveIOPS: v["hdd-iops"],
    },
    tape: {
      drives: v["tape-drives"],
      driveBWGBps: v["tape-bw-mb"] / 1000,
      capacityEB: v["tape-capacity"],
    },
    stream: {
      readStreams: v["read-streams"],
      writeStreams: v["write-streams"],
      segmentMB: v["segment-mb"],
      seekMs: v["seek-ms"],
    },
    workload: {
      fileSizeGB: v["file-size"],
      computeReadGBps: v["compute-read"],
      computeWriteGBps: v["compute-write"],
      nvmeHitRate: v["nvme-hit"] / 100,
      nvmeThroughFraction: v["nvme-through"] / 100,
      nvmeRereadFactor: v["nvme-reread"],
      workingSetPB: v["working-set"],
      reserveFraction: v["reserve-pct"] / 100,
      archiveEBPerYear: v["archive-eb"],
      recallGBps: v.recall,
      baselineEB: v["baseline-eb"],
      observedTBpsPerEB: v["observed-scale"],
      prefetchHorizonHours: v["prefetch-hours"],
    },
    prices: {
      nodeCHF: v["price-node"],
      nvmeCHFPerTB: v["price-nvme"],
      hddCHFPerTB: v["price-hdd"],
      tapeDriveCHF: v["price-tape-drive"],
      tapeCHFPerTB: v["price-tape"],
    },
    power: {
      nodeW: v["power-node"],
      nvmeDriveW: v["power-nvme"],
      hddDriveW: v["power-hdd"],
      tapeDriveW: v["power-tape"],
    },
    hybrid: document.getElementById("hybrid").getAttribute("aria-pressed") === "true",
    repack: document.getElementById("repack").getAttribute("aria-pressed") === "true",
    layout,
  };
}

let timer = 0;
function schedule() {
  clearTimeout(timer);
  timer = setTimeout(run, 80);
}

async function run() {
  const my = ++seq;
  const cfg = readConfig();
  if (!cfg) return;
  try {
    const res = await fetch("/api/simulate", {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify(cfg),
    });
    if (!res.ok) throw new Error(await res.text());
    const data = await res.json();
    if (my !== seq) return;
    render(data);
  } catch (err) {
    if (my !== seq) return;
    const banner = document.getElementById("banner");
    banner.className = "banner over";
    banner.replaceChildren(el("h2", "", "Model request failed"), el("p", "", String(err.message || err)));
  }
}

function render(r) {
  renderBanner(r);
  renderFeet(r);
  const layoutHint = document.getElementById("hdd-layout-hint");
  if (layoutHint && r.flow && r.flow.layoutNote) layoutHint.textContent = r.flow.layoutNote;
  renderStack(r);
  renderBudgets(r);
  renderBounds(r);
  renderFormula(r);
  renderCost(r);
  renderSweep(r);
  renderFlow(r);
  renderSummary(r);
  markPresets(r);
}

function flowDots(gbps) {
  if (!(gbps > 0)) return 0;
  if (window.matchMedia("(prefers-reduced-motion: reduce)").matches) return 1;
  if (gbps < 20) return 2;
  if (gbps < 200) return 3;
  if (gbps < 1000) return 4;
  return 5;
}

function flowDur(gbps) {
  const dur = Math.max(0.65, 2.8 - Math.log10((gbps || 0) + 10) * 0.55);
  return dur.toFixed(2);
}

function flowLane(direction, label, gbps, color) {
  const row = el("div", "lane " + direction);
  const top = el("div", "lane-top");
  top.append(el("span", "", label), el("span", "lane-v", bw(gbps)));
  const track = el("div", "track" + (gbps > 0 ? "" : " idle"));
  const n = flowDots(gbps);
  const dur = flowDur(gbps);
  for (let i = 0; i < n; i++) {
    const dot = document.createElement("i");
    dot.style.setProperty("--flow-color", color);
    dot.style.animationDuration = dur + "s";
    dot.style.animationDelay = (-dur * i / n).toFixed(2) + "s";
    track.append(dot);
  }
  row.append(top, track);
  return row;
}

function flowMeter(used, cap) {
  const wrap = el("div", "flow-meter");
  const bar = el("div", "flow-bar");
  const capSafe = Math.max(cap, 0);
  const usedSafe = Math.max(used, 0);
  const reserve = capSafe - usedSafe;
  const usedPct = capSafe <= 0 ? (usedSafe > 0 ? 100 : 0) : Math.min(100, 100 * usedSafe / capSafe);
  const usedEl = el("span", "used");
  const reserveEl = el("span", "reserve");
  usedEl.style.width = usedPct + "%";
  reserveEl.style.width = Math.max(0, 100 - usedPct) + "%";
  if (reserve < -0.05) bar.classList.add("over");
  bar.append(usedEl, reserveEl);
  const label = reserve < -0.05
    ? `Used ${bw(usedSafe)} · Over by ${bw(-reserve)}`
    : `Used ${bw(usedSafe)} · Reserve ${bw(Math.max(0, reserve))}`;
  wrap.append(bar, el("p", "", label));
  return wrap;
}

function flowPort(name, rate, note) {
  const box = el("div", "port");
  box.append(el("span", "k", name), el("b", "", rate), el("span", "note", note));
  return box;
}

function renderFlow(r) {
  const host = document.getElementById("flow-anim");
  if (!host || !r.flow) return;
  const f = r.flow;
  const archiveOut = f.hddArchiveGBps + (f.hddRepackReadGBps || 0);
  const recallIn = f.hddRecallGBps + (f.hddRepackWriteGBps || 0);
  const readStreams = intish(r.hdd.logicalReadStreams || 0);
  const writeStreams = intish(r.hdd.logicalWriteStreams || 0);

  const clients = el("aside", "flow-clients");
  clients.append(
    el("h4", "", "Clients"),
    el("p", "", "Read " + bw(f.computeReadGBps)),
    el("p", "", "Write " + bw(f.computeWriteGBps)),
    el("p", "", "NVMe hit " + pct(f.nvmeHitRate)),
    el("p", "", "Read-through " + pct(f.nvmeThroughFraction || 0)),
    el("p", "", "Re-read ×" + trim(f.nvmeRereadFactor || 0, 2)),
  );

  const through = f.nvmeThroughGBps || 0;
  const nvmeEgress = f.nvmeEgressGBps || 0;
  const nvmeLanes = el("div", "flow-lanes nvme");
  nvmeLanes.append(flowLane("to-client", "Egress", nvmeEgress, "#e4c39a"));

  const nvme = el("article", "flow-tier nvme");
  const nvmePorts = el("div", "ports");
  nvmePorts.append(
    flowPort("Egress", bw(nvmeEgress), "re-read ×" + trim(f.nvmeRereadFactor || 0, 2) + " plus hits"),
    flowPort("Ingress", bw(through), "staged from HDD"),
  );
  nvme.append(el("h4", "", "NVMe"), flowMeter(nvmeEgress, r.nvme.deliveredGBps), nvmePorts);

  const rise = el("div", "flow-rise");
  const vtrack = el("div", "vtrack" + (through > 0 ? "" : " idle"));
  const n = flowDots(through);
  const dur = flowDur(through);
  for (let i = 0; i < n; i++) {
    const dot = document.createElement("i");
    dot.style.animationDuration = dur + "s";
    dot.style.animationDelay = (-dur * i / n).toFixed(2) + "s";
    vtrack.append(dot);
  }
  const riseText = el("div", "flow-rise-text");
  riseText.append(el("span", "", "HDD → NVMe"), el("b", "", bw(through) + " staged"));
  rise.append(vtrack, riseText);

  const hddLanes = el("div", "flow-lanes hdd");
  hddLanes.append(
    flowLane("to-client", "Egress", f.hddComputeReadGBps, "#e4c39a"),
    flowLane("to-tier", "Ingress", f.hddComputeWriteGBps, "#c4894a"),
  );

  const hdd = el("article", "flow-tier hdd");
  const hddPorts = el("div", "ports");
  hddPorts.append(
    flowPort("Egress", bw(f.hddComputeReadGBps), readStreams + " read streams"),
    flowPort("Ingress", bw(f.hddComputeWriteGBps), writeStreams + " write streams"),
  );
  hdd.append(
    el("h4", "", "HDD"),
    flowMeter(f.hddDemandGBps, r.hdd.deliveredGBps),
    el("p", "flow-cap", "Capacity reserve " + pb(f.reserveEB * 1000)),
    hddPorts,
  );

  const tapeLabelOut = f.repack ? "Archive + repack" : "Archive";
  const tapeLabelIn = f.repack ? "Recall + repack" : "Recall";
  const tapeLanes = el("div", "flow-lanes tape");
  tapeLanes.append(
    flowLane("to-tier", tapeLabelOut, archiveOut, "#8f5e34"),
    flowLane("to-client", tapeLabelIn, recallIn, "#d4a574"),
  );

  const tape = el("article", "flow-tier tape");
  tape.append(
    el("h4", "", "Tape"),
    flowMeter(f.tapeDemandGBps, r.tape.bandwidthGBps),
    el("p", "flow-cap", f.repack ? "Repack reads and rewrites the library." : "Archive down, recall up."),
  );

  const board = el("div", "flow-board");
  board.append(clients, nvmeLanes, nvme, rise, hddLanes, hdd, tapeLanes, tape);
  host.replaceChildren(board);
}

function renderSummary(r) {
  const host = document.getElementById("summary");
  if (!host || !r.summary) return;
  const s = r.summary;
  host.className = "summary-grid";
  host.replaceChildren(
    summaryFigure("Usable bandwidth", bw(s.usableBandwidthGBps), `NVMe ${bw(s.nvmeBandwidthGBps)} · HDD ${bw(s.hddBandwidthGBps)}, read or write`),
    summaryFigure("Usable IOPS", iopsCompact(s.usableIOPS), `NVMe ${iopsCompact(s.nvmeIOPS)} · HDD ${iopsCompact(s.hddUsableIOPS)}, read or write`),
    summaryFigure("Total I/O IOPS", iopsCompact(s.totalIOPS), `NVMe ${iopsCompact(s.nvmeIOPS)} · HDD ${iopsCompact(s.hddIOPS)} on disk`),
    summaryFigure("Files/s", filesRate(s.hddFilesPerSec), `${trim(s.fileSizeGB, 2)} GB files · HDD read ${filesRate(s.hddReadFilesPerSec)} · write ${filesRate(s.hddWriteFilesPerSec)} · NVMe ${filesRate(s.nvmeFilesPerSec)} · tape ${filesRate(s.tapeFilesPerSec)}`),
    summaryFigure("Cost", chf(s.costCHF), "NVMe, HDD, and tape, including servers and media"),
    summaryFigure("Total capacity", capEB(s.totalCapacityEB), `NVMe ${capEB(s.nvmeCapacityEB)} · HDD ${capEB(s.hddCapacityEB)} · tape ${capEB(s.tapeCapacityEB)}`),
  );
}

function summaryFigure(label, value, split) {
  const box = document.createElement("article");
  box.append(el("span", "k", label), el("div", "v", value), el("p", "split", split));
  return box;
}

function capEB(eb) {
  const n = Number(eb);
  if (!Number.isFinite(n)) return "0 EB";
  if (Math.abs(n) >= 0.01) return trim(n, 2) + " EB";
  return trim(n * 1000, 2) + " PB";
}

function renderBanner(r) {
  const banner = document.getElementById("banner");
  banner.className = "banner " + (r.verdict.tone || "");
  banner.replaceChildren();
  banner.append(el("h2", "", r.verdict.title));
  if (r.warnings && r.warnings.length) {
    banner.append(el("p", "warn", r.warnings.join(" ")));
  }
  const list = document.createElement("ul");
  for (const line of r.verdict.lines || []) {
    list.append(el("li", "", line));
  }
  banner.append(list);
}

function renderFeet(r) {
  const nvmeServers = r.hybrid
    ? `On the HDD nodes · ${intish(r.nvme.drives)} drives · no extra servers`
    : `${intish(r.nvme.nodes)} nodes · ${intish(r.nvme.drives)} drives`;
  const nvmeNodes = r.hybrid ? "servers none" : `nodes ${chf(r.cost.nvmeNodesCHF)}`;
  document.getElementById("foot-nvme").textContent =
    `${nvmeServers} · ${capTB(r.nvme.capacityTB)} · ${bw(r.nvme.deliveredGBps)} delivered · ${nvmeNodes} · media ${chf(r.cost.nvmeMediaCHF)} · ${watts(r.power.nvmeW)}`;
  document.getElementById("foot-hdd").textContent =
    `${intish(r.hdd.nodes)} nodes · ${intish(r.hdd.drives)} drives · ${capTB(r.hdd.capacityTB)} · ${bw(r.hdd.deliveredGBps)} delivered · ${sentence(r.hdd.limit)} · nodes ${chf(r.cost.hddNodesCHF)} · media ${chf(r.cost.hddMediaCHF)} · ${watts(r.power.hddW)}`;
  document.getElementById("foot-tape").textContent =
    `${intish(r.tape.drives)} drives · ${bw(r.tape.bandwidthGBps)} · ${r.tape.capacityEB.toFixed(2)} EB · ${r.tape.maxPBPerDay.toFixed(2)} PB/day at full rate · drives ${chf(r.cost.tapeDrivesCHF)} · media ${chf(r.cost.tapeMediaCHF)} · ${watts(r.power.tapeW)}`;
}

function metric(label, value) {
  const row = document.createElement("div");
  row.append(el("span", "", label), el("b", "", value));
  return row;
}

function block(kind, name, headline, rows) {
  const box = el("section", "block " + kind);
  const head = document.createElement("header");
  head.append(el("div", "name", name), el("div", "headline", headline));
  const metrics = el("div", "metrics");
  for (const [label, value] of rows) metrics.append(metric(label, value));
  box.append(head, metrics);
  return box;
}

function renderStack(r) {
  const host = document.getElementById("stack");
  const prefetchLabel = trim(r.flow.prefetchHours, 1) + " h prefetch";
  host.replaceChildren(
    block("compute", "Compute", bw(r.flow.computeReadGBps) + " read", [
      ["Read", bw(r.flow.computeReadGBps)],
      ["Write", bw(r.flow.computeWriteGBps)],
      ["NVMe hit", pct(r.flow.nvmeHitRate)],
      ["Read-through", pct(r.flow.nvmeThroughFraction || 0)],
      ["Re-read", "×" + trim(r.flow.nvmeRereadFactor || 0, 2)],
      ["Working set", pb(r.flow.workingSetPB)],
    ]),
    el("div", "flow", "Reads"),
    block("nvme", "NVMe", bw(r.nvme.deliveredGBps), [
      ["Capacity", capTB(r.nvme.capacityTB)],
      [r.hybrid ? "Shared network" : "Network", bw(r.nvme.networkGBps)],
      ["Drive BW", bw(r.nvme.driveAggregateGBps)],
      ["Served", bw(r.flow.nvmeServedGBps)],
      ["Staged", bw(r.flow.nvmeThroughGBps || 0)],
      ["Egress", bw(r.flow.nvmeEgressGBps || 0)],
      ["IOPS", iops(r.nvme.aggregateIOPS)],
      ["Files/s", filesRate(r.summary.nvmeFilesPerSec)],
      ["Limit", sentence(r.nvme.limit)],
      [r.hybrid ? "Servers" : "Node cost", r.hybrid ? "None" : chf(r.cost.nvmeNodesCHF)],
      ["Media cost", chf(r.cost.nvmeMediaCHF)],
      [r.hybrid ? "Server power" : "Node power", r.hybrid ? "None" : watts(r.power.nvmeNodesW)],
      ["Drive power", watts(r.power.nvmeDrivesW)],
    ]),
    el("div", "flow", "Misses and staging"),
    block("hdd", "HDD", bw(r.hdd.deliveredGBps), [
      ["Layout", r.flow.layoutName],
      ["Disk streams", trim(r.hdd.streamsPerDrive, 2) + " / drive"],
      ["Capacity", capTB(r.hdd.capacityTB)],
      ["Stream total", bw(r.hdd.streamAggregateGBps)],
      ["Read BW", bw(r.hdd.readDeliveredGBps)],
      ["Write BW", bw(r.hdd.writeDeliveredGBps)],
      ["Degradation", pct(r.hdd.degradation)],
      ["IOPS", iops(r.hdd.deliveredIOPS)],
      ["Files/s", filesRate(r.summary.hddFilesPerSec)],
      ["Read files/s", filesRate(r.summary.hddReadFilesPerSec)],
      ["Write files/s", filesRate(r.summary.hddWriteFilesPerSec)],
      [r.hybrid ? "Shared network" : "Network", bw(r.hdd.networkGBps)],
      ["Observed", r.flow.observedEnabled ? bw(r.hdd.observedGBps) : "Off"],
      ["Hardware slack", bw(r.flow.hddSlackGBps)],
      ["Limit", sentence(r.hdd.limit)],
      ["Node cost", chf(r.cost.hddNodesCHF)],
      ["Media cost", chf(r.cost.hddMediaCHF)],
      ["Node power", watts(r.power.hddNodesW)],
      ["Drive power", watts(r.power.hddDrivesW)],
    ]),
    el("div", "flow", "Archive down · recall up"),
    block("tape", "Tape", bw(r.tape.bandwidthGBps), [
      ["Library", r.tape.capacityEB.toFixed(2) + " EB"],
      ["Drives", intish(r.tape.drives)],
      ["Files/s", filesRate(r.summary.tapeFilesPerSec)],
      ["Archive", bw(r.flow.archiveGBps)],
      ["Recall", bw(r.flow.recallGBps)],
      ["3y repack", r.flow.repack ? bw(r.flow.repackGBps) + " read + " + bw(r.flow.repackGBps) + " write" : "Off"],
      ["Slack", bw(r.flow.tapeSlackGBps)],
      ["Recall / day", pb(r.flow.recallPBPerDay)],
      ["Cold restage", hours(r.flow.stageWorkingSetHours)],
      [prefetchLabel, pb(r.flow.prefetchPB)],
      ["Drive cost", chf(r.cost.tapeDrivesCHF)],
      ["Media cost", chf(r.cost.tapeMediaCHF)],
      ["Drive power", watts(r.power.tapeDrivesW)],
    ]),
  );
}

function barBlock(title, segments, markerFrac, caption) {
  const wrap = el("div", "budget");
  wrap.append(el("h4", "", title));
  const outer = el("div", "bar-wrap");
  const bar = el("div", "bar");
  const total = segments.reduce((sum, seg) => sum + Math.max(0, seg.value), 0);
  if (total <= 0) {
    const empty = el("span");
    empty.style.width = "100%";
    bar.append(empty);
  } else {
    for (const seg of segments) {
      if (seg.value <= 0) continue;
      const span = el("span");
      span.style.width = (100 * seg.value) / total + "%";
      span.style.background = seg.color;
      span.title = seg.name + " " + seg.text;
      bar.append(span);
    }
  }
  outer.append(bar);
  if (markerFrac != null && Number.isFinite(markerFrac)) {
    const mark = el("i", "cap-marker");
    const clamped = Math.max(0, Math.min(1, markerFrac));
    mark.style.left = clamped * 100 + "%";
    mark.title = "Delivered bandwidth";
    outer.append(mark);
  }
  const legend = el("div", "legend");
  for (const seg of segments) {
    if (seg.value <= 0) continue;
    const item = el("span");
    const sw = el("i", "swatch");
    sw.style.background = seg.color;
    item.append(sw, document.createTextNode(seg.name + " " + seg.text));
    legend.append(item);
  }
  wrap.append(outer, legend, el("p", "hint", caption));
  return wrap;
}

function renderBudgets(r) {
  const host = document.getElementById("budgets");
  const hddSegs = [
    { name: "Compute read", value: r.flow.hddComputeReadGBps, color: "#f3efe8", text: bw(r.flow.hddComputeReadGBps) },
    { name: "Compute write", value: r.flow.hddComputeWriteGBps, color: "#b7aa9a", text: bw(r.flow.hddComputeWriteGBps) },
    { name: "Archive", value: r.flow.hddArchiveGBps, color: "#8f5e34", text: bw(r.flow.hddArchiveGBps) },
    { name: "Recall", value: r.flow.hddRecallGBps, color: "#c4894a", text: bw(r.flow.hddRecallGBps) },
    { name: "Repack read", value: r.flow.hddRepackReadGBps, color: "#d4a574", text: bw(r.flow.hddRepackReadGBps) },
    { name: "Repack write", value: r.flow.hddRepackWriteGBps, color: "#6e4a2a", text: bw(r.flow.hddRepackWriteGBps) },
  ];
  let hddCaption = `${r.flow.layoutName}: reads ${factor(r.flow.readVolume)}, writes ${factor(r.flow.writeVolume)}. Delivered ${bw(r.hdd.deliveredGBps)}. Unallocated ${bw(r.flow.hddSlackGBps)}.`;
  let marker = null;
  if (r.flow.hddSlackGBps >= 0) {
    hddSegs.push({ name: "Unallocated", value: r.flow.hddSlackGBps, color: "#3a3a3e", text: bw(r.flow.hddSlackGBps) });
  } else if (r.flow.hddDemandGBps > 0) {
    marker = r.hdd.deliveredGBps / r.flow.hddDemandGBps;
    hddCaption = `${r.flow.layoutName}: reads ${factor(r.flow.readVolume)}, writes ${factor(r.flow.writeVolume)}. Demand ${bw(r.flow.hddDemandGBps)} exceeds delivered ${bw(r.hdd.deliveredGBps)} by ${bw(-r.flow.hddSlackGBps)}. The white mark is delivered bandwidth.`;
  }
  if (r.flow.observedEnabled) {
    hddCaption += ` Observed scaling for this capacity is ${bw(r.hdd.observedGBps)}, with slack ${bw(r.flow.observedSlackGBps)}.`;
  }

  const tapeSegs = [
    { name: "Archive", value: r.flow.archiveGBps, color: "#8f5e34", text: bw(r.flow.archiveGBps) },
    { name: "Recall", value: r.flow.recallGBps, color: "#c4894a", text: bw(r.flow.recallGBps) },
    { name: "Repack read", value: r.flow.repackGBps, color: "#d4a574", text: bw(r.flow.repackGBps) },
    { name: "Repack write", value: r.flow.repackGBps, color: "#6e4a2a", text: bw(r.flow.repackGBps) },
  ];
  let tapeCaption = `Tape supplies ${bw(r.tape.bandwidthGBps)} and has ${bw(r.flow.tapeSlackGBps)} unallocated.`;
  let tapeMarker = null;
  if (r.flow.tapeSlackGBps >= 0) {
    tapeSegs.push({ name: "Unallocated", value: r.flow.tapeSlackGBps, color: "#3a332c", text: bw(r.flow.tapeSlackGBps) });
  } else if (r.flow.tapeDemandGBps > 0) {
    tapeMarker = r.tape.bandwidthGBps / r.flow.tapeDemandGBps;
    tapeCaption = `Archive and recall require ${bw(r.flow.tapeDemandGBps)}, ${bw(-r.flow.tapeSlackGBps)} above tape bandwidth.`;
  }

  const reservePB = r.flow.reserveEB * 1000;
  const capPB = r.hdd.capacityEB * 1000;
  const capSegs = [
    { name: "Working set", value: r.flow.workingSetPB, color: "#c4894a", text: pb(r.flow.workingSetPB) },
    { name: "Reserve", value: reservePB, color: "#5c3d22", text: pb(reservePB) },
  ];
  let capMarker = null;
  let capCaption = `Free space after the working set and reserve is ${pb(r.flow.freePB)}.`;
  if (r.flow.freePB >= 0) {
    capSegs.push({ name: "Free", value: r.flow.freePB, color: "#e4c39a", text: pb(r.flow.freePB) });
  } else {
    const used = r.flow.workingSetPB + reservePB;
    if (used > 0) capMarker = capPB / used;
    capCaption = `Working set plus reserve needs ${pb(r.flow.workingSetPB + reservePB)}. This tier holds ${pb(capPB)}.`;
  }

  host.replaceChildren(
    barBlock("HDD bandwidth", hddSegs, marker, hddCaption),
    barBlock("Tape bandwidth", tapeSegs, tapeMarker, tapeCaption),
    barBlock("HDD capacity", capSegs, capMarker, capCaption),
  );
}

function boundText(enabled, ok, eb) {
  if (!enabled) return "—";
  if (!ok) return "Not reachable";
  return eb.toFixed(2) + " EB";
}

function minFigure(label, text, short) {
  const box = document.createElement("div");
  const strong = el("strong", short ? "short" : "", text);
  box.append(el("span", "", label), strong);
  return box;
}

function renderBounds(r) {
  const host = document.getElementById("bounds");
  const b = r.bounds;
  const current = r.hdd.capacityEB;
  const hwText = b.hardwareMinOK ? b.hardwareMinEB.toFixed(2) + " EB" : "Not reachable";
  const obsLabel = b.observedEnabled
    ? `Observed minimum · ${trim(b.observedTBpsPerEB, 2)} TB/s per EB`
    : "Observed minimum";
  const obsText = b.observedEnabled
    ? (b.observedMinOK ? b.observedMinEB.toFixed(2) + " EB" : "Not reachable")
    : "Off";
  const mins = el("div", "mins");
  mins.append(
    minFigure("Hardware minimum", hwText, b.hardwareMinOK && current + 1e-9 < b.hardwareMinEB),
    minFigure(obsLabel, obsText, b.observedEnabled && b.observedMinOK && current + 1e-9 < b.observedMinEB),
  );

  const table = document.createElement("table");
  const head = document.createElement("tr");
  for (const label of ["Bound", "Hardware model", b.observedEnabled ? `Observed ${trim(b.observedTBpsPerEB, 2)} TB/s per EB` : "Observed scaling"]) {
    head.append(el("th", "", label));
  }
  const thead = document.createElement("thead");
  thead.append(head);
  const body = document.createElement("tbody");
  const rows = [
    ["Working set + reserve", b.capacityFloorEB.toFixed(2) + " EB", b.capacityFloorEB.toFixed(2) + " EB"],
    ["Compute read and write", boundText(true, b.hardwareComputeOK, b.hardwareComputeEB), boundText(b.observedEnabled, b.observedComputeOK, b.observedComputeEB)],
    ["Compute + archive + recall", boundText(true, b.hardwareBudgetOK, b.hardwareBudgetEB), boundText(b.observedEnabled, b.observedBudgetOK, b.observedBudgetEB)],
    ["Minimum", boundText(true, b.hardwareMinOK, b.hardwareMinEB), boundText(b.observedEnabled, b.observedMinOK, b.observedMinEB)],
  ];
  for (const row of rows) {
    const tr = document.createElement("tr");
    for (const cell of row) tr.append(el("td", "", cell));
    body.append(tr);
  }
  table.append(thead, body);
  host.replaceChildren(mins, table);
}

function renderFormula(r) {
  const list = document.getElementById("formula-lines");
  list.replaceChildren();
  for (const line of r.formula.lines || []) list.append(el("li", "", line));
  const perDrive = r.hdd.drives > 0 ? r.hdd.aggregateIOPS / r.hdd.drives : 0;
  document.getElementById("iops-note").textContent =
    `Rated random IOPS: ${iops(r.hdd.aggregateIOPS)} (${trim(perDrive, 0)} / drive). Streaming IOPS follow delivered bandwidth divided by the segment size: ${iops(r.hdd.deliveredIOPS)} (${trim(r.hdd.drives ? r.hdd.deliveredIOPS / r.hdd.drives : 0, 1)} / drive).`;
  document.getElementById("op-point").textContent =
    `Operating point: ${intish(r.hdd.readStreams)} disk read and ${intish(r.hdd.writeStreams)} disk write streams, ${trim(r.hdd.streamsPerDrive, 2)} per drive. Degradation is ${pct(r.hdd.degradation)}. Delivered ${bw(r.hdd.deliveredGBps)} splits into ${bw(r.hdd.readDeliveredGBps)} read and ${bw(r.hdd.writeDeliveredGBps)} write.`;
  drawChart(r.curve || [], r.hdd.streamsPerDrive, r.flow.observedEnabled);
}

function svgEl(name, attrs) {
  const node = document.createElementNS("http://www.w3.org/2000/svg", name);
  for (const [key, value] of Object.entries(attrs)) node.setAttribute(key, value);
  return node;
}

function niceMax(v) {
  if (!Number.isFinite(v) || v <= 0) return 1;
  const pow = 10 ** Math.floor(Math.log10(v));
  const n = v / pow;
  for (const step of [1, 1.2, 1.5, 2, 2.5, 3, 4, 5, 6, 8, 10]) {
    if (step + 1e-9 >= n) return step * pow;
  }
  return 10 * pow;
}

function axisBW(gbps, max) {
  if (max >= 1000) return trim(gbps / 1000, 1) + " TB/s";
  return trim(gbps, 0) + " GB/s";
}

function drawChart(curve, streams, observed) {
  const host = document.getElementById("chart");
  host.replaceChildren();
  if (!curve.length) return;
  const W = 640;
  const H = 320;
  const pad = { l: 78, r: 48, t: 12, b: 48 };
  const ys = [];
  for (const p of curve) ys.push(p.spindleGBps, p.deliveredGBps, p.readGBps, p.writeGBps, p.networkGBps, observed ? p.observedGBps : 0);
  const maxY = niceMax(Math.max(...ys, 1));
  const minX = curve[0].streamsPerDrive;
  const maxX = curve[curve.length - 1].streamsPerDrive;
  const span = Math.max(1e-6, maxX - minX);
  const xOf = (v) => pad.l + ((v - minX) / span) * (W - pad.l - pad.r);
  const yOf = (v) => pad.t + (1 - v / maxY) * (H - pad.t - pad.b);
  const yDeg = (frac) => pad.t + (1 - frac) * (H - pad.t - pad.b);
  const svg = svgEl("svg", { viewBox: `0 0 ${W} ${H}` });

  for (let i = 0; i <= 4; i++) {
    const val = (maxY * i) / 4;
    const y = yOf(val);
    svg.append(svgEl("line", { x1: pad.l, x2: W - pad.r, y1: y, y2: y, class: "grid" }));
    const tick = svgEl("text", { x: pad.l - 8, y: y + 4, "text-anchor": "end", class: "tick" });
    tick.textContent = axisBW(val, maxY);
    svg.append(tick);
    const right = svgEl("text", { x: W - pad.r + 8, y: y + 4, "text-anchor": "start", class: "tick degrade-tick" });
    right.textContent = (i * 25) + "%";
    svg.append(right);
  }

  const poly = (key, cls, yMap) => {
    const points = curve.map((p) => `${xOf(p.streamsPerDrive).toFixed(2)},${yMap(p[key]).toFixed(2)}`).join(" ");
    svg.append(svgEl("polyline", { points, class: cls }));
  };
  poly("spindleGBps", "series spindle", yOf);
  poly("deliveredGBps", "series delivered", yOf);
  poly("readGBps", "series read", yOf);
  poly("writeGBps", "series write", yOf);
  poly("degradation", "series degrade", yDeg);
  const netY = yOf(curve[0].networkGBps);
  svg.append(svgEl("line", { x1: xOf(minX), x2: xOf(maxX), y1: netY, y2: netY, class: "series network" }));
  if (observed && curve[0].observedGBps > 0) {
    const y = yOf(curve[0].observedGBps);
    svg.append(svgEl("line", { x1: xOf(minX), x2: xOf(maxX), y1: y, y2: y, class: "series observed" }));
  }

  let closest = curve[0];
  for (const p of curve) {
    if (Math.abs(p.streamsPerDrive - streams) < Math.abs(closest.streamsPerDrive - streams)) closest = p;
  }
  const mx = xOf(closest.streamsPerDrive);
  svg.append(svgEl("line", { x1: mx, x2: mx, y1: pad.t, y2: H - pad.b, class: "marker-line" }));
  svg.append(svgEl("circle", { cx: mx, cy: yOf(closest.deliveredGBps), r: 4, class: "marker-dot" }));
  svg.append(svgEl("circle", { cx: mx, cy: yDeg(closest.degradation), r: 3.5, class: "marker-deg" }));

  for (const v of [minX, Math.round((minX + maxX) / 2), maxX]) {
    const tick = svgEl("text", { x: xOf(v), y: H - pad.b + 16, "text-anchor": "middle", class: "tick" });
    tick.textContent = Math.abs(v - Math.round(v)) < 0.05 ? String(Math.round(v)) : v.toFixed(1);
    svg.append(tick);
  }
  const label = svgEl("text", { x: (pad.l + W - pad.r) / 2, y: H - 6, "text-anchor": "middle", class: "axis-label" });
  label.textContent = "Disk streams per drive";
  svg.append(label);
  svg.setAttribute("role", "img");
  svg.setAttribute("aria-label", `At ${trim(closest.streamsPerDrive, 2)} streams per drive, delivered bandwidth is ${bw(closest.deliveredGBps)} and seek degradation is ${pct(closest.degradation)}.`);

  const legend = el("div", "legend");
  const items = [
    ["spindle", "Spindle formula"],
    ["delivered", "Delivered"],
    ["read", "Read share"],
    ["write", "Write share"],
    ["network", "Network cap"],
    ["degrade", "Degradation"],
  ];
  if (observed) items.push(["observed", "Observed scaling"]);
  for (const [cls, name] of items) {
    const item = el("span");
    item.append(el("i", "swatch " + cls), document.createTextNode(name));
    legend.append(item);
  }
  host.append(svg, legend);
}

function renderCost(r) {
  const c = r.cost || {};
  const host = document.getElementById("cost-tiers");
  host.replaceChildren(
    block("nvme", "NVMe", chf(c.nvmeCHF), [
      ["Nodes", chf(c.nvmeNodesCHF)],
      ["Media", chf(c.nvmeMediaCHF)],
    ]),
    block("hdd", "HDD", chf(c.hddCHF), [
      ["Nodes", chf(c.hddNodesCHF)],
      ["Media", chf(c.hddMediaCHF)],
    ]),
    block("tape", "Tape", chf(c.tapeCHF), [
      ["Drives", chf(c.tapeDrivesCHF)],
      ["Media", chf(c.tapeMediaCHF)],
    ]),
  );
  const barHost = document.getElementById("cost-bar");
  barHost.replaceChildren(barBlock("System", [
    { name: "NVMe nodes", value: c.nvmeNodesCHF, color: "#e8d2b0", text: chf(c.nvmeNodesCHF) },
    { name: "NVMe media", value: c.nvmeMediaCHF, color: "#e4c39a", text: chf(c.nvmeMediaCHF) },
    { name: "HDD nodes", value: c.hddNodesCHF, color: "#d4a574", text: chf(c.hddNodesCHF) },
    { name: "HDD media", value: c.hddMediaCHF, color: "#c4894a", text: chf(c.hddMediaCHF) },
    { name: "Tape drives", value: c.tapeDrivesCHF, color: "#8f5e34", text: chf(c.tapeDrivesCHF) },
    { name: "Tape media", value: c.tapeMediaCHF, color: "#5c3d22", text: chf(c.tapeMediaCHF) },
  ], null, "Shares of the capital cost. Node price covers servers; media covers raw terabytes."));
  document.getElementById("cost-total").textContent = `System total ${chf(c.totalCHF)}.`;
  const note = document.getElementById("cost-baseline");
  if (c.baselineNodes > 0) {
    const gap = Math.abs(c.hddDeltaCHF);
    const way = c.hddDeltaCHF < -0.5 ? "less" : c.hddDeltaCHF > 0.5 ? "more" : "the same as";
    const amount = Math.abs(c.hddDeltaCHF) <= 0.5 ? "" : `${chf(gap)} ${way} than`;
    note.textContent = amount
      ? `This HDD tier is ${amount} a ${trim(r.hdd.baselineEB, 2)} EB layout of ${intish(c.baselineNodes)} nodes at ${chf(c.baselineHDDCHF)}.`
      : `This HDD tier matches a ${trim(r.hdd.baselineEB, 2)} EB layout of ${intish(c.baselineNodes)} nodes.`;
  } else {
    note.textContent = "Set a baseline HDD size to compare this tier with a larger spindle layout.";
  }
}

function renderSweep(r) {
  const host = document.getElementById("sweep");
  const table = document.createElement("table");
  const head = document.createElement("tr");
  const reference = (r.sweep || []).find((p) => p.hddOnly);
  for (const label of ["Size", "Nodes", "Capacity", "Hardware", "Observed", "Tape/HW", "Tape/obs", "HW slack", "Obs slack", "Working set", "NVMe", "HDD", "Tape", "Total", "vs HDD", "Power"]) {
    head.append(el("th", "", label));
  }
  const thead = document.createElement("thead");
  thead.append(head);
  const body = document.createElement("tbody");
  for (const p of r.sweep || []) {
    const tr = document.createElement("tr");
    if (p.active) tr.className = "active";
    const size = p.hddOnly ? "HDD only" : p.custom ? "Custom" : p.targetEB.toFixed(2) + " EB";
    const compared = !reference || p.hddOnly ? "—" : chf(p.costCHF - reference.costCHF);
    const cells = [
      size,
      intish(p.nodes),
      p.capacityEB.toFixed(2) + " EB",
      bw(p.deliveredGBps),
      p.observedEnabled ? bw(p.observedGBps) : "—",
      p.hddOnly ? "—" : pct(p.tapeToHardware),
      p.hddOnly || !p.observedEnabled ? "—" : pct(p.tapeToObserved),
      bw(p.hardwareSlackGBps),
      p.observedEnabled ? bw(p.observedSlackGBps) : "—",
      p.workingSetFits ? "Fits" : "Short",
      p.hddOnly ? "—" : chf(p.nvmeCHF),
      chf(p.hddCHF),
      p.hddOnly ? "—" : chf(p.tapeCHF),
      chf(p.costCHF),
      compared,
      watts(p.powerW),
    ];
    cells.forEach((text, i) => {
      const td = el("td", "", text);
      const negativeSlack = (i === 7 && p.hardwareSlackGBps < 0) || (i === 8 && p.observedEnabled && p.observedSlackGBps < 0);
      const shortSet = i === 9 && !p.workingSetFits;
      if (negativeSlack || shortSet) td.className = "neg";
      tr.append(td);
    });
    body.append(tr);
  }
  table.append(thead, body);
  host.replaceChildren(table);
}

function markPresets(r) {
  const drives = r.hdd.nodes ? r.hdd.drives / r.hdd.nodes : 0;
  const size = r.hdd.drives ? r.hdd.capacityTB / r.hdd.drives : 0;
  document.querySelectorAll("[data-eb]").forEach((btn) => {
    const per = drives * size;
    const nodes = per > 0 ? Math.max(1, Math.round((Number(btn.dataset.eb) * 1e6) / per)) : 0;
    btn.classList.toggle("on", nodes === r.hdd.nodes);
  });
  const segment = Number(document.getElementById("segment-mb").value);
  document.querySelectorAll("[data-segment]").forEach((btn) => {
    btn.classList.toggle("on", Number(btn.dataset.segment) === segment);
  });
}

function driveCount() {
  const nodes = num("hdd-nodes") || 0;
  const drives = num("hdd-drives") || 0;
  return nodes * drives;
}

function scaleStreams(perDrive) {
  const drives = driveCount();
  const read = num("read-streams") || 0;
  const write = num("write-streams") || 0;
  const total = read + write;
  const next = Math.round(perDrive * (drives > 0 ? drives : 1));
  const readFrac = total > 0 ? read / total : 1;
  const readNext = Math.round(next * readFrac);
  document.getElementById("read-streams").value = String(readNext);
  document.getElementById("write-streams").value = String(next - readNext);
  setStreamReadout(perDrive);
}

function syncStreamSlider() {
  const drives = driveCount();
  const read = num("read-streams") || 0;
  const write = num("write-streams") || 0;
  const per = drives > 0 ? (read + write) / drives : 0;
  document.getElementById("streams-slider").value = String(per);
  setStreamReadout(per);
}

function setStreamReadout(per) {
  const node = document.getElementById("streams-readout");
  if (!node) return;
  node.textContent = Math.abs(per - Math.round(per)) < 0.05 ? String(Math.round(per)) : per.toFixed(2);
}

const tapeHintOff = "Aggregate bandwidth is the drive count times bandwidth per drive. Library capacity is the cartridge pool and does not follow from the drive count. Treat the aggregate as burst capability, not a continuous setpoint.";
const tapeHintOn = tapeHintOff + " A 3-year repack reads that library and writes it back, on top of archive and recall. The HDD tier reads the data out to the new tapes and writes the data in from the old ones.";

function setRepack(on) {
  const btn = document.getElementById("repack");
  btn.setAttribute("aria-pressed", on ? "true" : "false");
  btn.classList.toggle("on", on);
  const hint = document.getElementById("tape-hint");
  if (hint) hint.textContent = on ? tapeHintOn : tapeHintOff;
}

function setHybrid(on) {
  const btn = document.getElementById("hybrid");
  btn.setAttribute("aria-pressed", on ? "true" : "false");
  btn.classList.toggle("on", on);
  document.querySelector(".card.nvme").classList.toggle("hybrid", on);
  document.getElementById("nvme-nodes").disabled = on;
  document.querySelectorAll('input[name="nvme-net"]').forEach((input) => {
    input.disabled = on;
  });
  document.getElementById("nvme-drives-k").textContent = on ? "Drives / HDD node" : "Drives / node";
  document.getElementById("nvme-hint").textContent = on
    ? "NVMe drives sit in the HDD nodes and add no servers. Both tiers share the HDD network, and cache traffic is taken from that network before the HDD tier."
    : "Delivered bandwidth is the minimum of aggregate drive bandwidth and the node line rate.";
}

function bind() {
  form.addEventListener("submit", (e) => e.preventDefault());
  document.getElementById("hybrid").addEventListener("click", () => {
    const on = document.getElementById("hybrid").getAttribute("aria-pressed") !== "true";
    setHybrid(on);
    schedule();
  });
  document.getElementById("repack").addEventListener("click", () => {
    const on = document.getElementById("repack").getAttribute("aria-pressed") !== "true";
    setRepack(on);
    schedule();
  });
  form.addEventListener("input", (e) => {
    if (e.target.id === "streams-slider") {
      scaleStreams(Number(e.target.value));
    }
    if (e.target.id === "read-streams" || e.target.id === "write-streams" || e.target.id === "hdd-nodes" || e.target.id === "hdd-drives") {
      syncStreamSlider();
    }
    schedule();
  });
  document.querySelectorAll("[data-eb]").forEach((btn) => {
    btn.addEventListener("click", () => {
      const drives = num("hdd-drives");
      const size = num("hdd-size");
      if (!drives || !size) return;
      const nodes = Math.max(1, Math.round((Number(btn.dataset.eb) * 1e6) / (drives * size)));
      const input = document.getElementById("hdd-nodes");
      input.value = String(nodes);
      input.dispatchEvent(new Event("input", { bubbles: true }));
    });
  });
  document.querySelectorAll("[data-segment]").forEach((btn) => {
    btn.addEventListener("click", () => {
      const input = document.getElementById("segment-mb");
      input.value = btn.dataset.segment;
      input.dispatchEvent(new Event("input", { bubbles: true }));
    });
  });
}

function init() {
  if (!window.BOOT) {
    document.querySelector("#banner h2").textContent = "Defaults failed to load";
    return;
  }
  fill(window.BOOT);
  bind();
  run();
}

init();
