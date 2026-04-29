const state = {
  trucks: [],
  destinations: [],
};

const el = {
  truckSelect: document.getElementById("truckSelect"),
  destinationsContainer: document.getElementById("destinationsContainer"),
  sendBtn: document.getElementById("sendBtn"),
  logsOutput: document.getElementById("logsOutput"),
  fuelProgress: document.getElementById("fuelProgress"),
  fuelLabel: document.getElementById("fuelLabel"),
  materialSelect: document.getElementById("materialSelect"),
  addByMaterialBtn: document.getElementById("addByMaterialBtn"),
  addTruckBtn: document.getElementById("addTruckBtn"),
  addDestinationBtn: document.getElementById("addDestinationBtn"),
  truckStatusBtn: document.getElementById("truckStatusBtn"),
};

const modal = {
  overlay: document.getElementById("modalOverlay"),
  title: document.getElementById("modalTitle"),
  message: document.getElementById("modalMessage"),
  ok: document.getElementById("modalOkBtn"),
};

const addTruck = {
  overlay: document.getElementById("addTruckOverlay"),
  name: document.getElementById("truckNameInput"),
  fuel: document.getElementById("truckFuelInput"),
  add: document.getElementById("truckAddConfirmBtn"),
  cancel: document.getElementById("truckAddCancelBtn"),
};

const addDestination = {
  overlay: document.getElementById("addDestinationOverlay"),
  name: document.getElementById("destNameInput"),
  distance: document.getElementById("destDistanceInput"),
  material: document.getElementById("destMaterialInput"),
  add: document.getElementById("destAddConfirmBtn"),
  cancel: document.getElementById("destAddCancelBtn"),
};

const statusModal = {
  overlay: document.getElementById("truckStatusOverlay"),
  truckSelect: document.getElementById("statusTruckSelect"),
  progress: document.getElementById("statusProgress"),
  text: document.getElementById("statusText"),
  ok: document.getElementById("statusOkBtn"),
};

async function api(path, method = "GET", payload) {
  const res = await fetch(path, {
    method,
    headers: { "Content-Type": "application/json" },
    body: payload ? JSON.stringify(payload) : undefined,
  });
  const data = await res.json();
  if (!res.ok) {
    throw new Error(data.error || "Request failed");
  }
  return data;
}

function showError(message, title = "Error") {
  modal.title.textContent = title;
  modal.message.textContent = message;
  modal.overlay.classList.remove("hidden");
}

function hideError() {
  modal.overlay.classList.add("hidden");
}

function showOverlay(overlay) {
  overlay.classList.remove("hidden");
}

function hideOverlay(overlay) {
  overlay.classList.add("hidden");
}

function selectedDestinationIds() {
  const checks = [...document.querySelectorAll(".destination-check")];
  return checks.filter((c) => c.checked).map((c) => c.value);
}

function appendLog(message) {
  const now = new Date().toLocaleTimeString();
  el.logsOutput.value += `[${now}] ${message}\n`;
  el.logsOutput.scrollTop = el.logsOutput.scrollHeight;
}

function renderState() {
  el.truckSelect.innerHTML = "";
  statusModal.truckSelect.innerHTML = "";
  state.trucks.forEach((t) => {
    const opt = document.createElement("option");
    opt.value = t.id;
    opt.textContent = `${t.name} (fuel: ${t.fuelCapacity})`;
    el.truckSelect.appendChild(opt);

    const statusOpt = document.createElement("option");
    statusOpt.value = t.id;
    statusOpt.textContent = `${t.name} (fuel: ${t.fuelCapacity})`;
    statusModal.truckSelect.appendChild(statusOpt);
  });

  el.destinationsContainer.innerHTML = "";
  state.destinations.forEach((d) => {
    const wrap = document.createElement("label");
    wrap.className = "checkbox-item";
    wrap.innerHTML = `
      <input class="destination-check" type="checkbox" value="${d.id}" />
      <span>${d.name} (dist: ${d.distance}, mat: ${d.material})</span>
    `;
    el.destinationsContainer.appendChild(wrap);
  });

  bindSelectionListeners();
}

function bindSelectionListeners() {
  document.querySelectorAll(".destination-check").forEach((c) => {
    c.addEventListener("change", updateProgress);
  });
}

async function loadState() {
  const data = await api("/api/state");
  state.trucks = data.trucks || [];
  state.destinations = data.destinations || [];
  renderState();
  await updateProgress();
}

async function updateProgress() {
  const truckId = el.truckSelect.value;
  if (!truckId) {
    el.fuelProgress.value = 0;
    el.fuelLabel.textContent = "0%";
    return;
  }
  try {
    const resp = await api("/api/route/preview", "POST", {
      truckId,
      destinationIds: selectedDestinationIds(),
      allowEmptyRoute: true,
    });
    const pct = Math.max(0, Number(resp.fuelPercentage || 0));
    el.fuelProgress.value = pct > 100 ? 100 : pct;
    el.fuelLabel.textContent = `${pct.toFixed(2)}% of truck fuel`;
  } catch (err) {
    el.fuelProgress.value = 0;
    el.fuelLabel.textContent = "0%";
    showError(err.message);
  }
}

async function sendRoute() {
  const truckId = el.truckSelect.value;
  if (!truckId) {
    showError("Please select a truck.");
    return;
  }
  try {
    const resp = await api("/api/route/send", "POST", {
      truckId,
      destinationIds: selectedDestinationIds(),
    });
    appendLog(resp.message);
    document.querySelectorAll(".destination-check").forEach((c) => {
      c.checked = false;
    });
    await updateProgress();
  } catch (err) {
    showError(err.message);
  }
}

async function addByMaterial() {
  const truckId = el.truckSelect.value;
  if (!truckId) {
    showError("Please select a truck first.");
    return;
  }
  try {
    const resp = await api("/api/add-by-material", "POST", {
      truckId,
      material: el.materialSelect.value,
      currentDestinationIds: selectedDestinationIds(),
    });
    const selectedIds = new Set(resp.destinationIds || []);
    document.querySelectorAll(".destination-check").forEach((c) => {
      c.checked = selectedIds.has(c.value);
    });
    appendLog(resp.message);
    await updateProgress();
  } catch (err) {
    showError(err.message);
  }
}

async function createTruck() {
  const name = addTruck.name.value.trim();
  const fuelCapacity = Number(addTruck.fuel.value);
  if (!name) {
    showError("Truck name is required.");
    return;
  }
  if (Number.isNaN(fuelCapacity) || fuelCapacity <= 0) {
    showError("Fuel capacity must be a positive number.");
    return;
  }
  try {
    await api("/api/trucks", "POST", { name, fuelCapacity });
    hideOverlay(addTruck.overlay);
    addTruck.name.value = "";
    addTruck.fuel.value = "";
    await loadState();
    appendLog(`Truck "${name}" added.`);
  } catch (err) {
    showError(err.message);
  }
}

async function createDestination() {
  const name = addDestination.name.value.trim();
  const distance = Number(addDestination.distance.value);
  const material = addDestination.material.value;

  if (!name) {
    showError("Destination name is required.");
    return;
  }
  if (Number.isNaN(distance)) {
    showError("Distance must be a number.");
    return;
  }
  try {
    await api("/api/destinations", "POST", { name, distance, material });
    hideOverlay(addDestination.overlay);
    addDestination.name.value = "";
    addDestination.distance.value = "";
    await loadState();
    appendLog(`Destination "${name}" added.`);
  } catch (err) {
    showError(err.message);
  }
}

async function showTruckStatus() {
  const truckId = statusModal.truckSelect.value;
  if (!truckId) {
    showError("Select truck for status.");
    return;
  }
  try {
    const resp = await api("/api/truck-status", "POST", { truckId });
    const pct = Number(resp.fuelPercentage || 0);
    statusModal.progress.value = pct > 100 ? 100 : Math.max(0, pct);
    statusModal.text.value = resp.message;
    showOverlay(statusModal.overlay);
  } catch (err) {
    showError(err.message);
  }
}

function bindEvents() {
  modal.ok.addEventListener("click", hideError);
  el.sendBtn.addEventListener("click", sendRoute);
  el.truckSelect.addEventListener("change", updateProgress);
  el.addByMaterialBtn.addEventListener("click", addByMaterial);

  el.addTruckBtn.addEventListener("click", () => showOverlay(addTruck.overlay));
  addTruck.cancel.addEventListener("click", () => hideOverlay(addTruck.overlay));
  addTruck.add.addEventListener("click", createTruck);

  el.addDestinationBtn.addEventListener("click", () => showOverlay(addDestination.overlay));
  addDestination.cancel.addEventListener("click", () => hideOverlay(addDestination.overlay));
  addDestination.add.addEventListener("click", createDestination);

  el.truckStatusBtn.addEventListener("click", async () => {
    statusModal.truckSelect.value = el.truckSelect.value;
    await showTruckStatus();
  });
  statusModal.ok.addEventListener("click", () => hideOverlay(statusModal.overlay));
  statusModal.truckSelect.addEventListener("change", showTruckStatus);
}

async function init() {
  bindEvents();
  await loadState();
}

init().catch((err) => showError(err.message || "Initialization failed"));
