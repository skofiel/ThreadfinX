class Server {
    constructor(cmd) {
        this.cmd = cmd;
    }
    request(data) {
        if (SERVER_CONNECTION == true) {
            return;
        }
        SERVER_CONNECTION = true;
        console.log(data);
        if (this.cmd != "updateLog") {
            // showElement("loading", true)
            UNDO = new Object();
        }
        switch (window.location.protocol) {
            case "http:":
                this.protocol = "ws://";
                break;
            case "https:":
                this.protocol = "wss://";
                break;
        }
        var url = this.protocol + window.location.hostname + ":" + window.location.port + "/data/" + "?Token=" + getCookie("Token");
        data["cmd"] = this.cmd;
        var ws = new WebSocket(url);
        ws.onopen = function () {
            WS_AVAILABLE = true;
            console.log("REQUEST (JS):");
            console.log(data);
            console.log("REQUEST: (JSON)");
            console.log(JSON.stringify(data));
            this.send(JSON.stringify(data));
        };
        ws.onerror = function (e) {
            console.log("{{.status.websocketError}}");
            SERVER_CONNECTION = false;
            if (WS_AVAILABLE == false) {
                alert("{{.status.websocketError}}");
            }
        };
        ws.onmessage = function (e) {
            SERVER_CONNECTION = false;
            showElement("loading", false);
            console.log("RESPONSE:");
            var response = JSON.parse(e.data);
            console.log(response);
            if (response.hasOwnProperty("token")) {
                document.cookie = "Token=" + response["token"];
            }
            if (response["status"] == false) {
                alert(response["err"]);
                if (response.hasOwnProperty("reload")) {
                    location.reload();
                }
                return;
            }
            if (response.hasOwnProperty("probeInfo")) {
                if (document.getElementById("probeDetails")) {
                    if (response["probeInfo"]["resolution"] !== undefined) {
                        document.getElementById("probeDetails").innerHTML = "<p>{{.status.resolution}}: <span class='text-accent'>" + response["probeInfo"]["resolution"] + "</span></p><p>{{.status.frameRate}}: <span class='text-accent'>" + response["probeInfo"]["frameRate"] + " FPS</span></p><p>{{.status.audio}}: <span class='text-accent'>" + response["probeInfo"]["audioChannel"] + "</span></p>";
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
                        let activeClass = "text-accent";
                        if (response["clientInfo"]["activePlaylist"] / response["clientInfo"]["totalPlaylist"] >= 0.6 && response["clientInfo"]["activePlaylist"] / response["clientInfo"]["totalPlaylist"] < 0.8) {
                            activeClass = "text-warning";
                        }
                        else if (response["clientInfo"]["activePlaylist"] / response["clientInfo"]["totalPlaylist"] >= 0.8) {
                            activeClass = "text-danger";
                        }
                        document.getElementById("playlist-connection-information").innerHTML = "{{.status.playlistConnections}}: <span class='" + activeClass + "'>" + response["clientInfo"]["activePlaylist"] + " / " + response["clientInfo"]["totalPlaylist"] + "</span>";
                    }
                    if (document.getElementById("client-connection-information")) {
                        let activeClass = "text-accent";
                        if (response["clientInfo"]["activeClients"] / response["clientInfo"]["totalClients"] >= 0.6 && response["clientInfo"]["activeClients"] / response["clientInfo"]["totalClients"] < 0.8) {
                            activeClass = "text-warning";
                        }
                        else if (response["clientInfo"]["activeClients"] / response["clientInfo"]["totalClients"] >= 0.8) {
                            activeClass = "text-danger";
                        }
                        document.getElementById("client-connection-information").innerHTML = "{{.status.clientConnections}}: <span class='" + activeClass + "'>" + response["clientInfo"]["activeClients"] + " / " + response["clientInfo"]["totalClients"] + "</span>";
                    }
                    return;
                    break;
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
    if (parts.length == 2)
        return parts.pop().split(";").shift();
}
