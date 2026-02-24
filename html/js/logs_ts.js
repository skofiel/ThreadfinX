class Log {
    createLog(entry) {
        var element = document.createElement("PRE");
        if (entry.indexOf("WARNING") !== -1) {
            element.className = "warningMsg";
        }
        if (entry.indexOf("ERROR") !== -1) {
            element.className = "errorMsg";
        }
        if (entry.indexOf("DEBUG") !== -1) {
            element.className = "debugMsg";
        }
        element.textContent = entry;
        return element;
    }
}
function showLogs(bottom) {
    var log = new Log();
    var logs = SERVER["log"]["log"];
    var div = document.getElementById("content_log");
    div.textContent = "";
    var keys = getObjKeys(logs);
    // Use DocumentFragment for batch DOM insertion (avoids multiple reflows)
    var fragment = document.createDocumentFragment();
    keys.forEach(function (logID) {
        var entry = log.createLog(logs[logID]);
        fragment.appendChild(entry);
    });
    div.appendChild(fragment);
    setTimeout(function () {
        if (bottom === true) {
            var wrapper = document.getElementById("box-wrapper");
            if (wrapper) {
                wrapper.scrollTop = wrapper.scrollHeight;
            }
        }
    }, 10);
}
function resetLogs() {
    var cmd = "resetLogs";
    var data = new Object();
    var server = new Server(cmd);
    server.request(data);
}
