class Server {
    constructor(cmd) {
        this.cmd = cmd;
        this.retryCount = 0;
        this.maxRetries = 3;
        this.baseDelay = 1000;
    }
    request(data) {
        if (SERVER_CONNECTION === true) {
            return;
        }
        SERVER_CONNECTION = true;
        if (this.cmd !== "updateLog") {
            UNDO = new Object();
        }
        var protocol;
        switch (window.location.protocol) {
            case "http:":
                protocol = "ws://";
                break;
            case "https:":
                protocol = "wss://";
                break;
        }
        // Connect without token in URL - server reads token from HttpOnly cookie
        var url = protocol + window.location.hostname + ":" + window.location.port + "/data/";
        data["cmd"] = this.cmd;
        var self = this;
        var ws = new WebSocket(url);
        ws.onopen = function () {
            WS_AVAILABLE = true;
            self.retryCount = 0;
            this.send(JSON.stringify(data));
        };
        ws.onerror = function (e) {
            SERVER_CONNECTION = false;
            if (WS_AVAILABLE === false) {
                alert("{{.status.websocketError}}");
            }
            // Retry with exponential backoff
            if (self.retryCount < self.maxRetries) {
                self.retryCount++;
                var delay = self.baseDelay * Math.pow(2, self.retryCount - 1);
                setTimeout(function () {
                    SERVER_CONNECTION = false;
                    self.request(data);
                }, delay);
            }
        };
        ws.onmessage = function (e) {
            SERVER_CONNECTION = false;
            showElement("loading", false);
            var response;
            try {
                response = JSON.parse(e.data);
            } catch (parseErr) {
                return;
            }
            // Token is now managed via HttpOnly cookie set by the server
            if (response["status"] === false) {
                alert(response["err"]);
                if (response.hasOwnProperty("reload")) {
                    location.reload();
                }
                return;
            }
            if (response.hasOwnProperty("probeInfo")) {
                var probeEl = document.getElementById("probeDetails");
                if (probeEl) {
                    if (response["probeInfo"]["resolution"] !== undefined) {
                        // Build probe info safely using DOM methods
                        probeEl.textContent = "";
                        var info = [
                            { label: "{{.status.resolution}}", value: response["probeInfo"]["resolution"] },
                            { label: "{{.status.frameRate}}", value: response["probeInfo"]["frameRate"] + " FPS" },
                            { label: "{{.status.audio}}", value: response["probeInfo"]["audioChannel"] }
                        ];
                        info.forEach(function (item) {
                            var p = document.createElement("P");
                            p.textContent = item.label + ": ";
                            var span = document.createElement("SPAN");
                            span.className = "text-accent";
                            span.textContent = item.value;
                            p.appendChild(span);
                            probeEl.appendChild(p);
                        });
                    }
                }
            }
            if (response.hasOwnProperty("logoURL")) {
                var div = document.getElementById("channel-icon");
                div.value = response["logoURL"];
                div.className = "changed";
                return;
            }
            switch (data["cmd"]) {
                case "updateLog":
                    SERVER["log"] = response["log"];
                    if (document.getElementById("content_log")) {
                        showLogs(false);
                    }
                    if (document.getElementById("playlist-connection-information")) {
                        var playlistEl = document.getElementById("playlist-connection-information");
                        var activePlaylist = response["clientInfo"]["activePlaylist"];
                        var totalPlaylist = response["clientInfo"]["totalPlaylist"];
                        var playlistClass = "text-accent";
                        if (activePlaylist / totalPlaylist >= 0.8) {
                            playlistClass = "text-danger";
                        } else if (activePlaylist / totalPlaylist >= 0.6) {
                            playlistClass = "text-warning";
                        }
                        playlistEl.textContent = "";
                        playlistEl.appendChild(document.createTextNode("{{.status.playlistConnections}}: "));
                        var span = document.createElement("SPAN");
                        span.className = playlistClass;
                        span.textContent = activePlaylist + " / " + totalPlaylist;
                        playlistEl.appendChild(span);
                    }
                    if (document.getElementById("client-connection-information")) {
                        var clientEl = document.getElementById("client-connection-information");
                        var activeClients = response["clientInfo"]["activeClients"];
                        var totalClients = response["clientInfo"]["totalClients"];
                        var clientClass = "text-accent";
                        if (activeClients / totalClients >= 0.8) {
                            clientClass = "text-danger";
                        } else if (activeClients / totalClients >= 0.6) {
                            clientClass = "text-warning";
                        }
                        clientEl.textContent = "";
                        clientEl.appendChild(document.createTextNode("{{.status.clientConnections}}: "));
                        var cspan = document.createElement("SPAN");
                        cspan.className = clientClass;
                        cspan.textContent = activeClients + " / " + totalClients;
                        clientEl.appendChild(cspan);
                    }
                    return;
                case "saveSettings":
                    // Update local data without re-rendering the page
                    SERVER["settings"] = response["settings"];
                    if (response["clientInfo"]) SERVER["clientInfo"] = response["clientInfo"];
                    applyAccentColor();
                    // Clear changed indicators
                    var changed = document.getElementsByClassName("changed");
                    while (changed.length > 0) {
                        changed[0].classList.remove("changed");
                    }
                    if (response.hasOwnProperty("reload")) {
                        location.reload();
                    }
                    return;
                default:
                    SERVER = new Object();
                    SERVER = response;
                    break;
            }
            if (response.hasOwnProperty("openMenu")) {
                var menu = document.getElementById(response["openMenu"]);
                menu.click();
                showElement("popup", false);
            }
            if (response.hasOwnProperty("openLink")) {
                window.location = response["openLink"];
            }
            if (response.hasOwnProperty("alert")) {
                alert(response["alert"]);
            }
            if (response.hasOwnProperty("reload")) {
                location.reload();
            }
            if (response.hasOwnProperty("wizard")) {
                createLayout();
                configurationWizard[response["wizard"]].createWizard();
                return;
            }
            createLayout();
        };
    }
}
function getCookie(name) {
    var value = "; " + document.cookie;
    var parts = value.split("; " + name + "=");
    if (parts.length === 2)
        return parts.pop().split(";").shift();
}
