(function () {
    const state = {
        ws: null,
        shellWs: null,
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
            redirectToLogin();
            return;
        }

        cacheElements();
        bindEvents();
        connectWS();
        await loadContainers();
        restoreFromParams();
        window.setInterval(loadContainers, 15000);
    }

    function restoreFromParams() {
        const params = new URLSearchParams(window.location.search);
        const containerId = params.get("container");
        if (!containerId || containerId === state.activeId) return;

        const item = document.querySelector(
            `.c-item[data-id="${CSS.escape(containerId)}"]`,
        );
        if (item) {
            const name = item.querySelector(".c-name")?.textContent || "";
            selectContainer(containerId, name);
        }
    }

    async function checkAuth() {
        try {
            const response = await fetch("/api/auth/check");
            return response.ok;
        } catch (error) {
            return false;
        }
    }

    function redirectToLogin() {
        const currentUrl = window.location.pathname + window.location.search;
        window.location.replace(
            "/login?next=" + encodeURIComponent(currentUrl || "/"),
        );
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
        elements.menuToggle = document.getElementById("menuToggle");
        elements.sidebar = document.getElementById("sidebar");
        elements.sidebarOverlay = document.getElementById("sidebarOverlay");
        elements.confirmModal = document.getElementById("confirmModal");
        elements.modalIcon = document.getElementById("modalIcon");
        elements.modalTitle = document.getElementById("modalTitle");
        elements.modalMessage = document.getElementById("modalMessage");
        elements.modalCancel = document.getElementById("modalCancel");
        elements.modalConfirm = document.getElementById("modalConfirm");

        // Shell elements
        elements.shellBtn = document.getElementById("shellBtn");
        elements.shellPasswordModal =
            document.getElementById("shellPasswordModal");
        elements.shellPasswordInput =
            document.getElementById("shellPasswordInput");
        elements.shellError = document.getElementById("shellError");
        elements.shellPasswordCancel = document.getElementById(
            "shellPasswordCancel",
        );
        elements.shellPasswordConfirm = document.getElementById(
            "shellPasswordConfirm",
        );
        elements.shellTerminalModal =
            document.getElementById("shellTerminalModal");
        elements.shellTerminal = document.getElementById("shellTerminal");
        elements.shellCloseBtn = document.getElementById("shellCloseBtn");
        elements.shellContainerName =
            document.getElementById("shellContainerName");
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
        elements.stopBtn.addEventListener("click", () => confirmAction("stop"));
        elements.restartBtn.addEventListener("click", () =>
            confirmAction("restart"),
        );
        elements.logoutBtn.addEventListener("click", () =>
            confirmAction("logout"),
        );

        // Mobile sidebar toggle
        elements.menuToggle.addEventListener("click", toggleSidebar);
        elements.sidebarOverlay.addEventListener("click", closeSidebar);

        // Modal events
        elements.modalCancel.addEventListener("click", closeModal);
        elements.confirmModal.addEventListener("click", (e) => {
            if (e.target === elements.confirmModal) closeModal();
        });

        // Shell events
        elements.shellBtn.addEventListener("click", openShellPasswordDialog);
        elements.shellPasswordCancel.addEventListener(
            "click",
            closeShellPasswordDialog,
        );
        elements.shellPasswordModal.addEventListener("click", (e) => {
            if (e.target === elements.shellPasswordModal)
                closeShellPasswordDialog();
        });
        elements.shellPasswordConfirm.addEventListener(
            "click",
            verifyShellPassword,
        );
        elements.shellPasswordInput.addEventListener("keydown", (e) => {
            if (e.key === "Enter") verifyShellPassword();
        });
        elements.shellCloseBtn.addEventListener("click", closeShellTerminal);
        elements.shellTerminalModal.addEventListener("click", (e) => {
            if (e.target === elements.shellTerminalModal) closeShellTerminal();
        });
    }

    let pendingAction = null;

    function confirmAction(action) {
        if (action !== "logout" && !state.activeId) return;

        pendingAction = action;
        const containerName = state.activeName || "this container";

        const config = {
            stop: {
                title: "Stop Container",
                message: `Are you sure you want to stop "${containerName}"? Any running processes will be terminated.`,
                icon: "stop",
                iconSvg:
                    '<svg viewBox="0 0 12 12" fill="currentColor"><rect x="1" y="1" width="10" height="10" rx="1"></rect></svg>',
                btnText: "Stop",
            },
            restart: {
                title: "Restart Container",
                message: `Are you sure you want to restart "${containerName}"? The container will be stopped and started again.`,
                icon: "restart",
                iconSvg:
                    '<svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><path d="M19 13.5C19 17.6421 15.6421 21 11.5 21C7.35786 21 4 17.6421 4 13.5C4 9.35786 7.35786 6 11.5 6H20M20 6L17 3M20 6L17 9" stroke-linecap="round" stroke-linejoin="round"></path></svg>',
                btnText: "Restart",
            },
            logout: {
                title: "Logout",
                message:
                    "Are you sure you want to logout? You will need to sign in again to access the dashboard.",
                icon: "stop",
                iconSvg:
                    '<svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><path d="M9 21H5a2 2 0 0 1-2-2V5a2 2 0 0 1 2-2h4M16 17l5-5-5-5M21 12H9" stroke-linecap="round" stroke-linejoin="round"></path></svg>',
                btnClass: "",
                btnText: "Logout",
            },
        };

        const cfg = config[action];
        elements.modalIcon.className = `modal-icon ${cfg.icon}`;
        elements.modalIcon.innerHTML = cfg.iconSvg;
        elements.modalTitle.textContent = cfg.title;
        elements.modalMessage.textContent = cfg.message;
        elements.modalConfirm.className = `modal-btn confirm ${action === "restart" ? "restart" : ""}`;
        elements.modalConfirm.textContent = cfg.btnText;
        elements.modalConfirm.onclick = executeConfirmedAction;
        elements.confirmModal.classList.add("visible");
    }

    function executeConfirmedAction() {
        if (pendingAction) {
            if (pendingAction === "logout") {
                logout();
            } else {
                containerAction(pendingAction);
            }
            closeModal();
        }
    }

    function closeModal() {
        elements.confirmModal.classList.remove("visible");
        pendingAction = null;
    }

    // Shell dialog functions
    function openShellPasswordDialog() {
        if (!state.activeId) return;
        elements.shellPasswordInput.value = "";
        elements.shellError.textContent = "";
        elements.shellPasswordModal.classList.add("visible");
        elements.shellPasswordInput.focus();
    }

    function closeShellPasswordDialog() {
        elements.shellPasswordModal.classList.remove("visible");
        elements.shellPasswordInput.value = "";
        elements.shellError.textContent = "";
    }

    async function verifyShellPassword() {
        const password = elements.shellPasswordInput.value;
        if (!password) {
            elements.shellError.textContent = "Password is required";
            return;
        }

        elements.shellError.textContent = "";
        elements.shellPasswordConfirm.disabled = true;
        elements.shellPasswordConfirm.textContent = "Connecting...";

        try {
            const response = await fetch(
                `/api/containers/${state.activeId}/shell`,
                {
                    method: "POST",
                    headers: { "Content-Type": "application/json" },
                    body: JSON.stringify({ password }),
                },
            );

            const data = await response.json();

            if (!response.ok) {
                elements.shellError.textContent =
                    data.error || "Invalid password";
                return;
            }

            // Password verified, open shell terminal with the token
            closeShellPasswordDialog();
            openShellTerminal(data.shellToken);
        } catch (error) {
            elements.shellError.textContent =
                "Connection failed: " + error.message;
        } finally {
            elements.shellPasswordConfirm.disabled = false;
            elements.shellPasswordConfirm.textContent = "Connect";
        }
    }

    function openShellTerminal(shellToken) {
        elements.shellContainerName.textContent =
            state.activeName || state.activeId;
        elements.shellTerminal.innerHTML = "";
        elements.shellTerminalModal.classList.add("visible");

        // Create xterm.js terminal
        state.xterm = new Terminal({
            cursorBlink: true,
            fontSize: 14,
            fontFamily: "'JetBrains Mono', 'Consolas', monospace",
            theme: {
                background: "#1a1a2e",
                foreground: "#e5e5e5",
                cursor: "#e5e5e5",
                selectionBackground: "rgba(255, 255, 255, 0.3)",
                black: "#000000",
                red: "#cd0000",
                green: "#00cd00",
                yellow: "#cdcd00",
                blue: "#0000ee",
                magenta: "#cd00cd",
                cyan: "#00cdcd",
                white: "#e5e5e5",
                brightBlack: "#7f7f7f",
                brightRed: "#ff0000",
                brightGreen: "#00ff00",
                brightYellow: "#ffff00",
                brightBlue: "#5c5cff",
                brightMagenta: "#ff00ff",
                brightCyan: "#00ffff",
                brightWhite: "#ffffff",
            },
        });

        const fitAddon = new FitAddon.FitAddon();
        state.xtermFitAddon = fitAddon;
        state.xterm.loadAddon(fitAddon);
        state.xterm.open(elements.shellTerminal);

        // Fit terminal to container
        setTimeout(() => {
            fitAddon.fit();
        }, 10);

        // Connect to shell WebSocket with token
        const proto = window.location.protocol === "https:" ? "wss" : "ws";
        const wsUrl = `${proto}://${window.location.host}/ws/shell/${state.activeId}?token=${encodeURIComponent(shellToken)}`;
        state.shellWs = new WebSocket(wsUrl);

        state.shellWs.onopen = () => {
            // Send initial terminal size
            sendShellResize();
        };

        state.shellWs.onmessage = (event) => {
            try {
                const msg = JSON.parse(event.data);
                if (msg.error) {
                    state.xterm.write(
                        "\r\n\x1b[31mError: " + msg.error + "\x1b[0m\r\n",
                    );
                } else if (msg.type === "output") {
                    state.xterm.write(msg.data);
                }
            } catch (e) {
                state.xterm.write(event.data);
            }
        };

        state.shellWs.onclose = () => {
            state.xterm.write("\r\n\x1b[33mConnection closed.\x1b[0m\r\n");
            state.xterm.write("\x1b[?25l"); // Hide cursor using ANSI escape
        };

        state.shellWs.onerror = () => {
            state.xterm.write("\r\n\x1b[31mWebSocket error.\x1b[0m\r\n");
            state.xterm.write("\x1b[?25l"); // Hide cursor using ANSI escape
        };

        // Forward keyboard input to WebSocket
        state.xterm.onData((data) => {
            if (state.shellWs && state.shellWs.readyState === WebSocket.OPEN) {
                state.shellWs.send(data);
            }
        });

        // Handle window resize
        state.resizeHandler = () => {
            if (state.xtermFitAddon) {
                state.xtermFitAddon.fit();
                sendShellResize();
            }
        };
        window.addEventListener("resize", state.resizeHandler);

        // Focus terminal
        state.xterm.focus();
    }

    function sendShellResize() {
        if (
            state.shellWs &&
            state.shellWs.readyState === WebSocket.OPEN &&
            state.xterm
        ) {
            state.shellWs.send(
                JSON.stringify({
                    type: "resize",
                    width: state.xterm.cols,
                    height: state.xterm.rows,
                }),
            );
        }
    }

    function closeShellTerminal() {
        elements.shellTerminalModal.classList.remove("visible");
        if (state.resizeHandler) {
            window.removeEventListener("resize", state.resizeHandler);
            state.resizeHandler = null;
        }
        if (state.shellWs) {
            state.shellWs.close();
            state.shellWs = null;
        }
        if (state.xterm) {
            state.xterm.dispose();
            state.xterm = null;
            state.xtermFitAddon = null;
        }
        elements.shellTerminal.innerHTML = "";
    }

    function toggleSidebar() {
        const isOpen = elements.sidebar.classList.toggle("open");
        elements.sidebarOverlay.classList.toggle("visible", isOpen);
        document.body.style.overflow = isOpen ? "hidden" : "";
    }

    function closeSidebar() {
        elements.sidebar.classList.remove("open");
        elements.sidebarOverlay.classList.remove("visible");
        document.body.style.overflow = "";
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
        // Close sidebar on mobile after selection
        closeSidebar();

        state.activeId = id;
        state.activeName = name;
        state.allLogs = [];
        state.lineIndex = 0;

        // Update URL query param for persistence
        const url = new URL(window.location);
        url.searchParams.set("container", id);
        history.replaceState(null, "", url);

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
            redirectToLogin();
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
