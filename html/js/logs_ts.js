// Log table state
var LOG_SORT_COLUMN = 0; // default sort by date
var LOG_SORT_DIR = "asc";
var LOG_FILTER_TYPE = "all"; // all, info, warning, error, debug

function parseLogEntry(raw) {
    // Format: "2026-02-24 22:22:49 [Threadfin] XEPG: message..."
    // or:    "2026-02-24 22:22:49 [DEBUG] source: message..."
    var result = { date: "", type: "INFO", source: "", message: "", raw: raw };

    // Extract date (first 19 chars: YYYY-MM-DD HH:MM:SS)
    if (raw.length >= 19) {
        result.date = raw.substring(0, 19);
    }

    var rest = raw.substring(20);

    // Detect type
    if (rest.indexOf("[ERROR]") !== -1) {
        result.type = "ERROR";
    } else if (rest.indexOf("[WARNING]") !== -1) {
        result.type = "WARNING";
    } else if (rest.indexOf("[DEBUG]") !== -1) {
        result.type = "DEBUG";
    } else {
        result.type = "INFO";
    }

    // Extract source and message
    // Pattern: [Threadfin] Source: Message or [DEBUG] Source: Message
    var bracketEnd = rest.indexOf("] ");
    if (bracketEnd !== -1) {
        var afterBracket = rest.substring(bracketEnd + 2).trim();
        // Remove type tags like [ERROR], [WARNING] from the remaining text
        afterBracket = afterBracket.replace(/\[(ERROR|WARNING)\]\s*/, "");
        var colonIdx = afterBracket.indexOf(":");
        if (colonIdx !== -1 && colonIdx < 40) {
            result.source = afterBracket.substring(0, colonIdx).trim();
            result.message = afterBracket.substring(colonIdx + 1).trim();
        } else {
            result.source = "-";
            result.message = afterBracket;
        }
    } else {
        result.source = "-";
        result.message = rest;
    }

    return result;
}

function getTypeClass(type) {
    switch (type) {
        case "ERROR": return "errorMsg";
        case "WARNING": return "warningMsg";
        case "DEBUG": return "debugMsg";
        default: return "";
    }
}

function getTypeBadgeClass(type) {
    switch (type) {
        case "ERROR": return "log-badge log-badge-error";
        case "WARNING": return "log-badge log-badge-warning";
        case "DEBUG": return "log-badge log-badge-debug";
        default: return "log-badge log-badge-info";
    }
}

function showLogs(bottom) {
    var logs = SERVER["log"]["log"];
    var div = document.getElementById("content_log");
    if (!div) return;
    div.innerHTML = "";

    var keys = getObjKeys(logs);
    var entries = [];
    keys.forEach(function(logID) {
        entries.push(parseLogEntry(logs[logID]));
    });

    // Build filter toolbar
    var toolbar = document.createElement("DIV");
    toolbar.className = "log-toolbar";

    // Type filter buttons
    var filterGroup = document.createElement("DIV");
    filterGroup.className = "log-filter-group";

    var counts = { all: entries.length, INFO: 0, WARNING: 0, ERROR: 0, DEBUG: 0 };
    entries.forEach(function(e) { counts[e.type]++; });

    var filters = [
        { key: "all", label: "{{.log.filter.all}}", count: counts.all },
        { key: "ERROR", label: "{{.log.filter.error}}", count: counts.ERROR },
        { key: "WARNING", label: "{{.log.filter.warning}}", count: counts.WARNING },
        { key: "DEBUG", label: "{{.log.filter.debug}}", count: counts.DEBUG },
        { key: "INFO", label: "{{.log.filter.info}}", count: counts.INFO }
    ];

    filters.forEach(function(f) {
        var btn = document.createElement("BUTTON");
        btn.className = "log-filter-btn" + (LOG_FILTER_TYPE === f.key ? " active" : "");
        btn.textContent = f.label + " (" + f.count + ")";
        btn.setAttribute("data-filter", f.key);
        btn.onclick = function() {
            LOG_FILTER_TYPE = f.key;
            showLogs(false);
        };
        filterGroup.appendChild(btn);
    });

    toolbar.appendChild(filterGroup);

    // Search input
    var searchWrap = document.createElement("DIV");
    searchWrap.className = "search-container";
    var searchIcon = document.createElement("SPAN");
    searchIcon.className = "material-symbols-outlined search-icon";
    searchIcon.textContent = "search";
    searchWrap.appendChild(searchIcon);
    var searchInput = document.createElement("INPUT");
    searchInput.type = "text";
    searchInput.className = "search";
    searchInput.placeholder = "{{.log.search}}";
    searchInput.id = "log-search-input";
    // Restore previous search value
    if (window._logSearchValue) {
        searchInput.value = window._logSearchValue;
    }
    searchInput.oninput = function() {
        window._logSearchValue = this.value;
        filterLogTable();
    };
    searchWrap.appendChild(searchInput);
    toolbar.appendChild(searchWrap);

    div.appendChild(toolbar);

    // Apply type filter
    var filtered = entries;
    if (LOG_FILTER_TYPE !== "all") {
        filtered = entries.filter(function(e) { return e.type === LOG_FILTER_TYPE; });
    }

    // Sort
    filtered.sort(function(a, b) {
        var valA, valB;
        switch (LOG_SORT_COLUMN) {
            case 0: valA = a.date; valB = b.date; break;
            case 1: valA = a.type; valB = b.type; break;
            case 2: valA = a.source.toLowerCase(); valB = b.source.toLowerCase(); break;
            case 3: valA = a.message.toLowerCase(); valB = b.message.toLowerCase(); break;
            default: valA = a.date; valB = b.date;
        }
        if (valA < valB) return LOG_SORT_DIR === "asc" ? -1 : 1;
        if (valA > valB) return LOG_SORT_DIR === "asc" ? 1 : -1;
        return 0;
    });

    // Build table
    var table = document.createElement("TABLE");
    table.className = "log-table";
    table.id = "log_table";

    // Header
    var thead = document.createElement("THEAD");
    var headerRow = document.createElement("TR");
    var columns = [
        { label: "{{.log.column.date}}", idx: 0 },
        { label: "{{.log.column.type}}", idx: 1 },
        { label: "{{.log.column.source}}", idx: 2 },
        { label: "{{.log.column.message}}", idx: 3 }
    ];

    columns.forEach(function(col) {
        var th = document.createElement("TH");
        th.className = "log-th" + (LOG_SORT_COLUMN === col.idx ? " log-th-sorted" : "");
        th.textContent = col.label;
        // Sort arrow
        if (LOG_SORT_COLUMN === col.idx) {
            var arrow = document.createElement("SPAN");
            arrow.className = "log-sort-arrow";
            arrow.textContent = LOG_SORT_DIR === "asc" ? " \u25B2" : " \u25BC";
            th.appendChild(arrow);
        }
        th.onclick = function() {
            if (LOG_SORT_COLUMN === col.idx) {
                LOG_SORT_DIR = LOG_SORT_DIR === "asc" ? "desc" : "asc";
            } else {
                LOG_SORT_COLUMN = col.idx;
                LOG_SORT_DIR = "asc";
            }
            showLogs(false);
        };
        headerRow.appendChild(th);
    });
    thead.appendChild(headerRow);
    table.appendChild(thead);

    // Body
    var tbody = document.createElement("TBODY");
    tbody.id = "log_tbody";

    filtered.forEach(function(entry) {
        var tr = document.createElement("TR");
        tr.className = getTypeClass(entry.type);

        var tdDate = document.createElement("TD");
        tdDate.className = "log-td-date";
        tdDate.textContent = entry.date;
        tr.appendChild(tdDate);

        var tdType = document.createElement("TD");
        tdType.className = "log-td-type";
        var badge = document.createElement("SPAN");
        badge.className = getTypeBadgeClass(entry.type);
        badge.textContent = entry.type;
        tdType.appendChild(badge);
        tr.appendChild(tdType);

        var tdSource = document.createElement("TD");
        tdSource.className = "log-td-source";
        tdSource.textContent = entry.source;
        tr.appendChild(tdSource);

        var tdMsg = document.createElement("TD");
        tdMsg.className = "log-td-message";
        tdMsg.textContent = entry.message;
        tr.appendChild(tdMsg);

        tbody.appendChild(tr);
    });

    table.appendChild(tbody);
    div.appendChild(table);

    // Apply text search filter if exists
    if (window._logSearchValue) {
        filterLogTable();
    }

    // Scroll to bottom
    setTimeout(function() {
        if (bottom === true) {
            var wrapper = document.getElementById("box-wrapper");
            if (wrapper) {
                wrapper.scrollTop = wrapper.scrollHeight;
            }
        }
    }, 10);
}

function filterLogTable() {
    var searchValue = (document.getElementById("log-search-input") || {}).value || "";
    searchValue = searchValue.toLowerCase();
    var tbody = document.getElementById("log_tbody");
    if (!tbody) return;
    var rows = tbody.getElementsByTagName("TR");
    for (var i = 0; i < rows.length; i++) {
        var text = rows[i].textContent.toLowerCase();
        rows[i].style.display = (searchValue.length === 0 || text.indexOf(searchValue) !== -1) ? "" : "none";
    }
}

function resetLogs() {
    var cmd = "resetLogs";
    var data = new Object();
    var server = new Server(cmd);
    server.request(data);
}
