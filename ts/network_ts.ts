class Server {
  protocol: string
  cmd: string

  constructor(cmd: string) {
    this.cmd = cmd
  }

  request(data: Object): any {

    if (SERVER_CONNECTION == true) {
      return
    }

    SERVER_CONNECTION = true

    console.log(data)
    if (this.cmd != "updateLog") {
      showElement("loading", true)
      UNDO = new Object()
    }

    switch (window.location.protocol) {
      case "http:":
        this.protocol = "ws://"
        break
      case "https:":
        this.protocol = "wss://"
        break
    }

    var url = this.protocol + window.location.hostname + ":" + window.location.port + "/data/" + "?Token=" + getCookie("Token")

    data["cmd"] = this.cmd
    var ws = new WebSocket(url)

    // Timeout fallback: hide loading after 30 seconds if no response
    var loadingTimeout = setTimeout(function() {
      showElement("loading", false)
      SERVER_CONNECTION = false
      console.log("Request timed out after 30 seconds")
    }, 30000)

    ws.onopen = function () {

      WS_AVAILABLE = true

      console.log("REQUEST (JS):");
      console.log(data)

      console.log("REQUEST: (JSON)");
      console.log(JSON.stringify(data))

      this.send(JSON.stringify(data));

    }

    ws.onerror = function (e) {

      clearTimeout(loadingTimeout)
      showElement("loading", false)
      console.log("No websocket connection to Threadfin could be established. Check your network configuration.")
      SERVER_CONNECTION = false

      if (WS_AVAILABLE == false) {
        alert("No websocket connection to Threadfin could be established. Check your network configuration.")
      }

    }


    ws.onmessage = function (e) {

      clearTimeout(loadingTimeout)
      SERVER_CONNECTION = false
      showElement("loading", false)

      console.log("RESPONSE:");
      var response = JSON.parse(e.data);

      console.log(response);

      if (response.hasOwnProperty("token")) {
        document.cookie = "Token=" + response["token"]
      }

      if (response["status"] == false) {

        alert(response["err"])

        if (response.hasOwnProperty("reload")) {
          location.reload()
        }

        return
      }

      if (response.hasOwnProperty("probeInfo")) {
        if (document.getElementById("probeDetails")) {
          if (response["probeInfo"]["resolution"] !== undefined) {
            document.getElementById("probeDetails").innerHTML =
              "<div class='probe-details-card'>" +
              "<div class='probe-detail-item'><span class='material-symbols-outlined'>aspect_ratio</span><span>Resolution:</span> <span class='probe-detail-value'>" + response["probeInfo"]["resolution"] + "</span></div>" +
              "<div class='probe-detail-item'><span class='material-symbols-outlined'>speed</span><span>Frame Rate:</span> <span class='probe-detail-value'>" + response["probeInfo"]["frameRate"] + " FPS</span></div>" +
              "<div class='probe-detail-item'><span class='material-symbols-outlined'>volume_up</span><span>Audio:</span> <span class='probe-detail-value'>" + response["probeInfo"]["audioChannel"] + "</span></div>" +
              "</div>"
          }
        }
      }


      if (response.hasOwnProperty("logoURL")) {
        var div = (document.getElementById("channel-icon") as HTMLInputElement)
        div.value = response["logoURL"]
        div.className = "changed"
        var logoPreview = document.getElementById("logo-preview-img") as HTMLImageElement
        if (logoPreview) {
          logoPreview.setAttribute("src", response["logoURL"])
          logoPreview.style.display = ""
        }
        return
      }

      switch (data["cmd"]) {
        case "updateLog":
          SERVER["log"] = response["log"]
          if (document.getElementById("content_log")) {
            showLogs(false)
          }
          if (document.getElementById("playlist-connection-information")) {
            let activeClass = "text-primary"
            if (response["clientInfo"]["activePlaylist"] / response["clientInfo"]["totalPlaylist"] >= 0.6 && response["clientInfo"]["activePlaylist"] / response["clientInfo"]["totalPlaylist"] < 0.8) {
              activeClass = "text-warning"
            } else if (response["clientInfo"]["activePlaylist"] / response["clientInfo"]["totalPlaylist"] >= 0.8) {
              activeClass = "text-danger"
            }
            document.getElementById("playlist-connection-information").innerHTML = "<span class='material-symbols-outlined conn-icon'>playlist_play</span>Playlist: <span class='" + activeClass + "'>" + response["clientInfo"]["activePlaylist"] + " / " + response["clientInfo"]["totalPlaylist"] + "</span>"
          }

          if (document.getElementById("client-connection-information")) {
            let activeClass = "text-primary"
            if (response["clientInfo"]["activeClients"] / response["clientInfo"]["totalClients"] >= 0.6 && response["clientInfo"]["activeClients"] / response["clientInfo"]["totalClients"] < 0.8) {
              activeClass = "text-warning"
            } else if (response["clientInfo"]["activeClients"] / response["clientInfo"]["totalClients"] >= 0.8) {
              activeClass = "text-danger"
            }
            document.getElementById("client-connection-information").innerHTML = "<span class='material-symbols-outlined conn-icon'>devices</span>Clients: <span class='" + activeClass + "'>" + response["clientInfo"]["activeClients"] + " / " + response["clientInfo"]["totalClients"] + "</span>"
          }
          updateErrorBadge()
          return
          break;

        default:
          SERVER = new Object()
          SERVER = response
          break;
      }

      if (response.hasOwnProperty("openMenu")) {
        var menu = document.getElementById(response["openMenu"])
        menu.click()
        showElement("popup", false)
      }

      if (response.hasOwnProperty("openLink")) {
        window.location = response["openLink"]
      }

      if (response.hasOwnProperty("alert")) {
        alert(response["alert"])
      }

      if (response.hasOwnProperty("reload")) {
        location.reload()
      }


      if (response.hasOwnProperty("wizard")) {
        createLayout()
        configurationWizard[response["wizard"]].createWizard()
        return
      }

      createLayout()

    }

  }

}

function getCookie(name) {
  var value = "; " + document.cookie;
  var parts = value.split("; " + name + "=");
  if (parts.length == 2) return parts.pop().split(";").shift();
}