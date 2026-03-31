(function () {
    const state = {
        ws: null,
        allLogs: [],
        activeId: null,
        activeName: null,
        showStderr: true,
        autoFollow: true,
        filterText: "",
        lineIndex: 0,
    };

    const elements = {};

    window.addEventListener("DOMContentLoaded", init);

    async function init() {
        // Check authentication first
        if (!(await checkAuth())) {
            window.location.href = "/login";
            return;
        }

        cacheElements();
        bindEvents();
        connectWS();
        loadContainers();
        window.setInterval(loadContainers, 15000);
    }

    async function checkAuth() {
        try {
            const response = await fetch("/api/auth/check");
            return response.ok;
        } catch (error) {
            return false;
        }
    }

    function cacheElements() {
        elements.cList = document.getElementById("cList");
        elements.clearBtn = document.getElementById("clearBtn");
        elements.emptyState = document.getElementById("emptyState");
        elements.errBtn = document.getElementById("errBtn");
        elements.filterInput = document.getElementById("filterInput");
        elements.followBtn = document.getElementById("followBtn");
        elements.hdrCount = document.getElementById("hdrCount");
        elements.hdrLines = document.getElementById("hdrLines");
        elements.logOut = document.getElementById("logOut");
        elements.nodeCount = document.getElementById("nodeCount");
        elements.activeView = document.getElementById("activeView");
        elements.refreshBtn = document.getElementById("refreshBtn");
        elements.restartBtn = document.getElementById("restartBtn");
        elements.sbContainer = document.getElementById("sbContainer");
        elements.sbContName = document.getElementById("sbContName");
        elements.sbLines = document.getElementById("sbLines");
        elements.scrollBtn = document.getElementById("scrollBtn");
        elements.spinner = document.getElementById("spinner");
        elements.startBtn = document.getElementById("startBtn");
        elements.stopBtn = document.getElementById("stopBtn");
        elements.tbId = document.getElementById("tbId");
        elements.tbName = document.getElementById("tbName");
        elements.wsDot = document.getElementById("wsDot");
        elements.wsLabel = document.getElementById("wsLabel");
        elements.wsStatus = document.getElementById("wsStatus");
        elements.logoutBtn = document.getElementById("logoutBtn");
    }

    function bindEvents() {
        elements.clearBtn.addEventListener("click", clearLogs);
        elements.errBtn.addEventListener("click", toggleStderr);
        elements.filterInput.addEventListener("input", applyFilter);
        elements.followBtn.addEventListener("click", toggleFollow);
        elements.logOut.addEventListener("scroll", handleLogScroll);
        elements.refreshBtn.addEventListener("click", loadContainers);
        elements.scrollBtn.addEventListener("click", scrollToBottom);
        elements.startBtn.addEventListener("click", () =>
            containerAction("start"),
        );
        elements.stopBtn.addEventListener("click", () =>
            containerAction("stop"),
        );
        elements.restartBtn.addEventListener("click", () =>
            containerAction("restart"),
        );
        elements.logoutBtn.addEventListener("click", logout);
    }

    async function logout() {
        try {
            await fetch("/api/logout", { method: "POST" });
        } catch (error) {
            console.error("Logout error:", error);
        }
        window.location.href = "/login";
    }

    function connectWS() {
        const proto = window.location.protocol === "https:" ? "wss" : "ws";
        state.ws = new WebSocket(`${proto}://${window.location.host}/ws`);

        state.ws.onopen = () => {
            setWSStatus(true);
            if (state.activeId) {
                sendWS({ action: "subscribe", containerId: state.activeId });
            }
        };

        state.ws.onclose = () => {
            setWSStatus(false);
            window.setTimeout(connectWS, 3000);
        };

        state.ws.onerror = () => {
            setWSStatus(false);
        };

        state.ws.onmessage = (event) => {
            const message = JSON.parse(event.data);
            if (message.error || message.containerId !== state.activeId) {
                return;
            }

            const parsed = parseTimestamp(message.text);
            const entry = {
                ts: parsed.ts,
                stream: message.type,
                text: parsed.body,
            };
            state.allLogs.push(entry);
            appendLine(entry, true);
        };
    }

    async function loadContainers() {
        renderLoadingState();

        try {
            const containers = await fetchJSON("/api/containers");
            renderContainers(containers);
        } catch (error) {
            elements.cList.innerHTML = `<div class="loading-row">${escapeHTML(`Failed to load: ${error.message}`)}</div>`;
        }
    }

    function renderLoadingState() {
        elements.cList.innerHTML = `
      <div class="loading-row">
        <div class="spinner visible"></div>
        <span>Loading containers...</span>
      </div>
    `;
    }

    function renderContainers(containers) {
        elements.nodeCount.textContent = String(containers.length);
        elements.hdrCount.textContent = String(containers.length);
        elements.cList.innerHTML = "";

        if (!containers.length) {
            elements.cList.innerHTML =
                '<div class="loading-row">No running containers</div>';
            return;
        }

        containers.forEach((container, index) => {
            const item = document.createElement("div");
            item.className = `c-item${container.fullId === state.activeId ? " active" : ""}`;
            item.dataset.id = container.fullId;
            item.style.animationDelay = `${index * 40}ms`;
            item.innerHTML = `
        <div class="c-row1">
          <div class="c-status ${container.state === "running" ? "" : "stopped"}"></div>
          <div class="c-name">${escapeHTML(container.name)}</div>
          <div class="c-live">LIVE</div>
        </div>
        <div class="c-row2">
          <div class="c-image">${escapeHTML(container.image)}</div>
          <div class="c-id">${escapeHTML(container.id)}</div>
        </div>
      `;
            item.addEventListener("click", () =>
                selectContainer(container.fullId, container.name),
            );
            elements.cList.appendChild(item);
        });
    }

    async function selectContainer(id, name) {
        state.activeId = id;
        state.activeName = name;
        state.allLogs = [];
        state.lineIndex = 0;

        document.querySelectorAll(".c-item").forEach((item) => {
            item.classList.toggle("active", item.dataset.id === id);
        });

        elements.emptyState.style.display = "none";
        elements.activeView.style.display = "flex";
        elements.tbName.textContent = name;
        elements.tbId.textContent = id.substring(0, 12);
        elements.sbContName.textContent = name;
        elements.sbContainer.style.display = "flex";
        elements.logOut.innerHTML = "";
        updateCount(0);

        sendWS({ action: "subscribe", containerId: id });

        elements.spinner.style.display = "block";
        try {
            const data = await fetchJSON(`/api/logs/${id}`);
            if (data.logs) {
                state.allLogs = data.logs.map((line) => {
                    const parsed = parseTimestamp(line.text);
                    return {
                        ts: parsed.ts,
                        stream: line.type,
                        text: parsed.body,
                    };
                });
                renderAll();
            }
        } catch (error) {
            console.error(error);
        } finally {
            elements.spinner.style.display = "none";
            if (state.autoFollow) {
                scrollToBottom();
            }
        }
    }

    async function containerAction(action) {
        if (!state.activeId) {
            return;
        }

        const buttonMap = {
            start: elements.startBtn,
            stop: elements.stopBtn,
            restart: elements.restartBtn,
        };

        const button = buttonMap[action];
        if (!button || button.disabled) {
            return;
        }

        button.classList.add("loading");
        button.disabled = true;

        try {
            await fetch(`/api/containers/${state.activeId}/${action}`, {
                method: "POST",
            });
            await loadContainers();
            if (action === "restart" && state.activeId) {
                selectContainer(state.activeId, state.activeName);
            }
        } catch (error) {
            console.error(`Failed to ${action} container:`, error);
        } finally {
            button.classList.remove("loading");
            button.disabled = false;
        }
    }

    function renderAll() {
        const fragment = document.createDocumentFragment();
        const filter = state.filterText.toLowerCase();

        elements.logOut.innerHTML = "";
        state.lineIndex = 0;

        let visibleCount = 0;
        state.allLogs.forEach((line) => {
            if (!shouldDisplayLine(line, filter)) {
                return;
            }

            fragment.appendChild(buildLine(line, filter, false));
            visibleCount += 1;
        });

        elements.logOut.appendChild(fragment);
        updateCount(visibleCount);
    }

    function appendLine(line, animate) {
        const filter = state.filterText.toLowerCase();
        if (!shouldDisplayLine(line, filter)) {
            return;
        }

        elements.logOut.appendChild(buildLine(line, filter, animate));
        updateCount(
            Number.parseInt(elements.sbLines.textContent || "0", 10) + 1,
        );

        if (state.autoFollow) {
            scrollToBottom();
        }
    }

    function shouldDisplayLine(line, filter) {
        if (!state.showStderr && line.stream === "stderr") {
            return false;
        }

        if (!filter) {
            return true;
        }

        return (
            line.text.toLowerCase().includes(filter) ||
            line.ts.toLowerCase().includes(filter)
        );
    }

    function buildLine(line, filter, animate) {
        state.lineIndex += 1;

        const element = document.createElement("div");
        const isHighlighted = filter && shouldHighlight(line, filter);

        element.className = "log-line";
        if (line.stream === "stderr") {
            element.classList.add("is-err");
        }
        if (isHighlighted) {
            element.classList.add("is-hl");
        }
        if (animate) {
            element.classList.add("is-new");
        }

        const bodyHTML = filter
            ? highlightMatch(escapeHTML(line.text), filter)
            : escapeHTML(line.text);

        element.innerHTML =
            `<span class="ll-lnum">${state.lineIndex}</span>` +
            `<span class="ll-ts">${escapeHTML(line.ts)}</span>` +
            `<span class="ll-badge"><span class="${line.stream === "stderr" ? "err-tag" : "out-tag"}">${line.stream === "stderr" ? "ERR" : "OUT"}</span></span>` +
            `<span class="ll-text">${bodyHTML}</span>`;

        return element;
    }

    function updateCount(count) {
        const value = String(count);
        elements.sbLines.textContent = value;
        elements.hdrLines.textContent = value;
    }

    function applyFilter() {
        state.filterText = elements.filterInput.value;
        renderAll();
    }

    function toggleStderr() {
        state.showStderr = !state.showStderr;
        elements.errBtn.classList.toggle("on", state.showStderr);
        renderAll();
    }

    function toggleFollow() {
        state.autoFollow = !state.autoFollow;
        elements.followBtn.classList.toggle("on", state.autoFollow);
        if (state.autoFollow) {
            scrollToBottom();
        }
    }

    function clearLogs() {
        state.allLogs = [];
        state.lineIndex = 0;
        elements.logOut.innerHTML = "";
        updateCount(0);
    }

    function handleLogScroll() {
        const atBottom =
            elements.logOut.scrollHeight -
                elements.logOut.scrollTop -
                elements.logOut.clientHeight <
            80;
        elements.scrollBtn.style.display = atBottom ? "none" : "flex";
    }

    function scrollToBottom() {
        elements.logOut.scrollTop = elements.logOut.scrollHeight;
    }

    function setWSStatus(isConnected) {
        elements.wsDot.className = isConnected ? "dot-g" : "dot-d";
        elements.wsLabel.textContent = isConnected
            ? "Connected"
            : "Disconnected";
        elements.wsStatus.className = `v ${isConnected ? "ws-ok" : "ws-no"}`;
        elements.wsStatus.textContent = isConnected ? "● Live" : "● Offline";
    }

    function sendWS(payload) {
        if (state.ws && state.ws.readyState === WebSocket.OPEN) {
            state.ws.send(JSON.stringify(payload));
        }
    }

    async function fetchJSON(url) {
        const response = await fetch(url);

        // Handle authentication errors
        if (response.status === 401) {
            window.location.href = "/login";
            throw new Error("Session expired");
        }

        const text = await response.text();

        let data = null;
        if (text) {
            try {
                data = JSON.parse(text);
            } catch (error) {
                throw new Error(`Invalid JSON from ${url}`);
            }
        }

        if (!response.ok) {
            throw new Error(
                (data && data.error) ||
                    `Request failed with status ${response.status}`,
            );
        }

        return data;
    }

    function parseTimestamp(raw) {
        const match = raw.match(/^(\d{4}-\d{2}-\d{2}T[\d:.]+Z)\s+(.*)$/s);
        if (!match) {
            return { ts: "-", body: raw };
        }

        const date = new Date(match[1]);
        const timestamp =
            date.toLocaleDateString("en-GB", {
                day: "2-digit",
                month: "short",
                year: "numeric",
            }) +
            " " +
            date.toLocaleTimeString("en-GB", { hour12: false }) +
            "." +
            String(date.getMilliseconds()).padStart(3, "0");

        return { ts: timestamp, body: match[2] };
    }

    function shouldHighlight(line, filter) {
        return (
            line.text.toLowerCase().includes(filter) ||
            line.ts.toLowerCase().includes(filter)
        );
    }

    function escapeHTML(text) {
        return text
            .replace(/&/g, "&amp;")
            .replace(/</g, "&lt;")
            .replace(/>/g, "&gt;");
    }

    function highlightMatch(html, query) {
        const pattern = new RegExp(
            query.replace(/[.*+?^${}()|[\]\\]/g, "\\$&"),
            "gi",
        );
        return html.replace(
            pattern,
            (match) => `<span class="match">${match}</span>`,
        );
    }
})();
