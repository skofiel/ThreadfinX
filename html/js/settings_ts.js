class SettingsCategory {
    constructor() {
        this.DocumentID = "content_settings";
    }
    createCategoryHeadline(value) {
        var element = document.createElement("H4");
        element.innerHTML = value;
        return element;
    }
    createHR() {
        var element = document.createElement("HR");
        return element;
    }
    createSettings(settingsKey) {
        var setting = document.createElement("TR");
        var content = new PopupContent();
        var data = SERVER["settings"][settingsKey];
        switch (settingsKey) {
            // Texteingaben
            case "update":
                var tdLeft = document.createElement("TD");
                tdLeft.innerHTML = "{{.settings.update.title}}" + ":";
                var tdRight = document.createElement("TD");
                var input = content.createInput("text", "update", data.toString());
                input.setAttribute("placeholder", "{{.settings.update.placeholder}}");
                input.setAttribute("onchange", "javascript: this.className = 'changed'");
                tdRight.appendChild(input);
                setting.appendChild(tdLeft);
                setting.appendChild(tdRight);
                break;
            case "backup.path":
                var tdLeft = document.createElement("TD");
                tdLeft.innerHTML = "{{.settings.backupPath.title}}" + ":";
                var tdRight = document.createElement("TD");
                var input = content.createInput("text", "backup.path", data);
                input.setAttribute("placeholder", "{{.settings.backupPath.placeholder}}");
                input.setAttribute("onchange", "javascript: this.className = 'changed'");
                tdRight.appendChild(input);
                setting.appendChild(tdLeft);
                setting.appendChild(tdRight);
                break;
            case "temp.path":
                var tdLeft = document.createElement("TD");
                tdLeft.innerHTML = "{{.settings.tempPath.title}}" + ":";
                var tdRight = document.createElement("TD");
                var input = content.createInput("text", "temp.path", data);
                input.setAttribute("placeholder", "{{.settings.tmpPath.placeholder}}");
                input.setAttribute("onchange", "javascript: this.className = 'changed'");
                tdRight.appendChild(input);
                setting.appendChild(tdLeft);
                setting.appendChild(tdRight);
                break;
            case "user.agent":
                var tdLeft = document.createElement("TD");
                tdLeft.innerHTML = "{{.settings.userAgent.title}}" + ":";
                var tdRight = document.createElement("TD");
                var input = content.createInput("text", "user.agent", data);
                input.setAttribute("placeholder", "{{.settings.userAgent.placeholder}}");
                input.setAttribute("onchange", "javascript: this.className = 'changed'");
                tdRight.appendChild(input);
                setting.appendChild(tdLeft);
                setting.appendChild(tdRight);
                break;
            case "buffer.timeout":
                var tdLeft = document.createElement("TD");
                tdLeft.innerHTML = "{{.settings.bufferTimeout.title}}" + ":";
                var tdRight = document.createElement("TD");
                var input = content.createInput("text", "buffer.timeout", data);
                input.setAttribute("placeholder", "{{.settings.bufferTimeout.placeholder}}");
                input.setAttribute("onchange", "javascript: this.className = 'changed'");
                tdRight.appendChild(input);
                setting.appendChild(tdLeft);
                setting.appendChild(tdRight);
                break;
            case "ffmpeg.path":
                var tdLeft = document.createElement("TD");
                tdLeft.innerHTML = "{{.settings.ffmpegPath.title}}" + ":";
                var tdRight = document.createElement("TD");
                var input = content.createInput("text", "ffmpeg.path", data);
                input.setAttribute("placeholder", "{{.settings.ffmpegPath.placeholder}}");
                input.setAttribute("onchange", "javascript: this.className = 'changed'");
                tdRight.appendChild(input);
                setting.appendChild(tdLeft);
                setting.appendChild(tdRight);
                break;
            case "ffmpeg.options":
                var tdLeft = document.createElement("TD");
                tdLeft.innerHTML = "{{.settings.ffmpegOptions.title}}" + ":";
                var tdRight = document.createElement("TD");
                var input = content.createInput("text", "ffmpeg.options", data);
                input.setAttribute("placeholder", "{{.settings.ffmpegOptions.placeholder}}");
                input.setAttribute("onchange", "javascript: this.className = 'changed'");
                tdRight.appendChild(input);
                setting.appendChild(tdLeft);
                setting.appendChild(tdRight);
                break;
            case "ffmpeg.forceHttp":
                var tdLeft = document.createElement("TD");
                tdLeft.innerHTML = "{{.settings.ffmpegForceHttp.title}}" + ":";
                var tdRight = document.createElement("TD");
                var input = content.createCheckbox(settingsKey);
                input.checked = data;
                input.setAttribute("onchange", "javascript: this.className = 'changed'");
                tdRight.appendChild(input);
                setting.appendChild(tdLeft);
                setting.appendChild(tdRight);
                break;
            case "vlc.path":
                var tdLeft = document.createElement("TD");
                tdLeft.innerHTML = "{{.settings.vlcPath.title}}" + ":";
                var tdRight = document.createElement("TD");
                var input = content.createInput("text", "vlc.path", data);
                input.setAttribute("placeholder", "{{.settings.vlcPath.placeholder}}");
                input.setAttribute("onchange", "javascript: this.className = 'changed'");
                tdRight.appendChild(input);
                setting.appendChild(tdLeft);
                setting.appendChild(tdRight);
                break;
            case "vlc.options":
                var tdLeft = document.createElement("TD");
                tdLeft.innerHTML = "{{.settings.vlcOptions.title}}" + ":";
                var tdRight = document.createElement("TD");
                var input = content.createInput("text", "vlc.options", data);
                input.setAttribute("placeholder", "{{.settings.vlcOptions.placeholder}}");
                input.setAttribute("onchange", "javascript: this.className = 'changed'");
                tdRight.appendChild(input);
                setting.appendChild(tdLeft);
                setting.appendChild(tdRight);
                break;
            // Checkboxen
            case "authentication.web":
                var tdLeft = document.createElement("TD");
                tdLeft.innerHTML = "{{.settings.authenticationWEB.title}}" + ":";
                var tdRight = document.createElement("TD");
                var input = content.createCheckbox(settingsKey);
                input.checked = data;
                input.setAttribute("onchange", "javascript: this.className = 'changed'");
                tdRight.appendChild(input);
                setting.appendChild(tdLeft);
                setting.appendChild(tdRight);
                break;
            case "authentication.pms":
                var tdLeft = document.createElement("TD");
                tdLeft.innerHTML = "{{.settings.authenticationPMS.title}}" + ":";
                var tdRight = document.createElement("TD");
                var input = content.createCheckbox(settingsKey);
                input.checked = data;
                input.setAttribute("onchange", "javascript: this.className = 'changed'");
                tdRight.appendChild(input);
                setting.appendChild(tdLeft);
                setting.appendChild(tdRight);
                break;
            case "authentication.m3u":
                var tdLeft = document.createElement("TD");
                tdLeft.innerHTML = "{{.settings.authenticationM3U.title}}" + ":";
                var tdRight = document.createElement("TD");
                var input = content.createCheckbox(settingsKey);
                input.checked = data;
                input.setAttribute("onchange", "javascript: this.className = 'changed'");
                tdRight.appendChild(input);
                setting.appendChild(tdLeft);
                setting.appendChild(tdRight);
                break;
            case "authentication.xml":
                var tdLeft = document.createElement("TD");
                tdLeft.innerHTML = "{{.settings.authenticationXML.title}}" + ":";
                var tdRight = document.createElement("TD");
                var input = content.createCheckbox(settingsKey);
                input.checked = data;
                input.setAttribute("onchange", "javascript: this.className = 'changed'");
                tdRight.appendChild(input);
                setting.appendChild(tdLeft);
                setting.appendChild(tdRight);
                break;
            case "authentication.api":
                var tdLeft = document.createElement("TD");
                tdLeft.innerHTML = "{{.settings.authenticationAPI.title}}" + ":";
                var tdRight = document.createElement("TD");
                var input = content.createCheckbox(settingsKey);
                input.checked = data;
                input.setAttribute("onchange", "javascript: this.className = 'changed'");
                tdRight.appendChild(input);
                setting.appendChild(tdLeft);
                setting.appendChild(tdRight);
                break;
            case "files.update":
                var tdLeft = document.createElement("TD");
                tdLeft.innerHTML = "{{.settings.filesUpdate.title}}" + ":";
                var tdRight = document.createElement("TD");
                var input = content.createCheckbox(settingsKey);
                input.checked = data;
                input.setAttribute("onchange", "javascript: this.className = 'changed'");
                tdRight.appendChild(input);
                setting.appendChild(tdLeft);
                setting.appendChild(tdRight);
                break;
            case "cache.images":
                var tdLeft = document.createElement("TD");
                tdLeft.innerHTML = "{{.settings.cacheImages.title}}" + ":";
                var tdRight = document.createElement("TD");
                var input = content.createCheckbox(settingsKey);
                input.checked = data;
                input.setAttribute("onchange", "javascript: this.className = 'changed'");
                tdRight.appendChild(input);
                setting.appendChild(tdLeft);
                setting.appendChild(tdRight);
                break;
            case "xepg.replace.missing.images":
                var tdLeft = document.createElement("TD");
                tdLeft.innerHTML = "{{.settings.replaceEmptyImages.title}}" + ":";
                var tdRight = document.createElement("TD");
                var input = content.createCheckbox(settingsKey);
                input.checked = data;
                input.setAttribute("onchange", "javascript: this.className = 'changed'");
                tdRight.appendChild(input);
                setting.appendChild(tdLeft);
                setting.appendChild(tdRight);
                break;
            case "xepg.replace.channel.title":
                var tdLeft = document.createElement("TD");
                tdLeft.innerHTML = "{{.settings.replaceChannelTitle.title}}" + ":";
                var tdRight = document.createElement("TD");
                var input = content.createCheckbox(settingsKey);
                input.checked = data;
                input.setAttribute("onchange", "javascript: this.className = 'changed'");
                tdRight.appendChild(input);
                setting.appendChild(tdLeft);
                setting.appendChild(tdRight);
                break;
            case "storeBufferInRAM":
                var tdLeft = document.createElement("TD");
                tdLeft.innerHTML = "{{.settings.storeBufferInRAM.title}}" + ":";
                var tdRight = document.createElement("TD");
                var input = content.createCheckbox(settingsKey);
                input.checked = data;
                input.setAttribute("onchange", "javascript: this.className = 'changed'");
                tdRight.appendChild(input);
                setting.appendChild(tdLeft);
                setting.appendChild(tdRight);
                break;
            case "forceHttps":
                var tdLeft = document.createElement("TD");
                tdLeft.innerHTML = "{{.settings.forceHttps.title}}" + ":";
                var tdRight = document.createElement("TD");
                var input = content.createCheckbox(settingsKey);
                input.checked = data;
                input.setAttribute("onchange", "javascript: this.className = 'changed'");
                tdRight.appendChild(input);
                setting.appendChild(tdLeft);
                setting.appendChild(tdRight);
                break;
            case "excludeStreamHttps":
                var tdLeft = document.createElement("TD");
                tdLeft.innerHTML = "{{.settings.excludeStreamHttps.title}}" + ":";
                var tdRight = document.createElement("TD");
                var input = content.createCheckbox(settingsKey);
                input.checked = data;
                input.setAttribute("onchange", "javascript: this.className = 'changed'");
                tdRight.appendChild(input);
                setting.appendChild(tdLeft);
                setting.appendChild(tdRight);
                break;
            case "httpsPort":
                var tdLeft = document.createElement("TD");
                tdLeft.innerHTML = "{{.settings.httpsPort.title}}" + ":";
                var tdRight = document.createElement("TD");
                var input = content.createInput("text", "httpsPort", data.toString());
                input.setAttribute("placeholder", "{{.settings.httpsPort.placeholder}}");
                input.setAttribute("onchange", "javascript: this.className = 'changed'");
                tdRight.appendChild(input);
                setting.appendChild(tdLeft);
                setting.appendChild(tdRight);
                break;
            case "httpsThreadfinDomain":
                var tdLeft = document.createElement("TD");
                tdLeft.innerHTML = "{{.settings.httpsThreadfinDomain.title}}" + ":";
                var tdRight = document.createElement("TD");
                var input = content.createInput("text", "httpsThreadfinDomain", data.toString());
                input.setAttribute("placeholder", "{{.settings.httpsThreadfinDomain.placeholder}}");
                input.setAttribute("onchange", "javascript: this.className = 'changed'");
                tdRight.appendChild(input);
                setting.appendChild(tdLeft);
                setting.appendChild(tdRight);
                break;
            case "bindIpAddress":
                var tdLeft = document.createElement("TD");
                tdLeft.innerHTML = "{{.settings.bindIpAddress.title}}" + ":";
                var tdRight = document.createElement("TD");
                var input = content.createInput("text", "bindIpAddress", data.toString());
                input.setAttribute("placeholder", "{{.settings.bindIpAddress.placeholder}}");
                input.setAttribute("onchange", "javascript: this.className = 'changed'");
                tdRight.appendChild(input);
                setting.appendChild(tdLeft);
                setting.appendChild(tdRight);
                break;
            case "httpThreadfinDomain":
                var tdLeft = document.createElement("TD");
                tdLeft.innerHTML = "{{.settings.httpThreadfinDomain.title}}" + ":";
                var tdRight = document.createElement("TD");
                var input = content.createInput("text", "httpThreadfinDomain", data.toString());
                input.setAttribute("placeholder", "{{.settings.httpThreadfinDomain.placeholder}}");
                input.setAttribute("onchange", "javascript: this.className = 'changed'");
                tdRight.appendChild(input);
                setting.appendChild(tdLeft);
                setting.appendChild(tdRight);
                break;
            case "enableNonAscii":
                var tdLeft = document.createElement("TD");
                tdLeft.innerHTML = "{{.settings.enableNonAscii.title}}" + ":";
                var tdRight = document.createElement("TD");
                var input = content.createCheckbox(settingsKey);
                input.checked = data;
                input.setAttribute("onchange", "javascript: this.className = 'changed'");
                tdRight.appendChild(input);
                setting.appendChild(tdLeft);
                setting.appendChild(tdRight);
                break;
            case "epgCategories":
                var tdLeft = document.createElement("TD");
                tdLeft.innerHTML = "{{.settings.epgCategories.title}}" + ":";
                var tdRight = document.createElement("TD");
                var input = content.createInput("text", "epgCategories", data.toString());
                input.setAttribute("placeholder", "{{.settings.epgCategories.placeholder}}");
                input.setAttribute("onchange", "javascript: this.className = 'changed'");
                tdRight.appendChild(input);
                setting.appendChild(tdLeft);
                setting.appendChild(tdRight);
                break;
            case "epgCategoriesColors":
                var tdLeft = document.createElement("TD");
                tdLeft.innerHTML = "{{.settings.epgCategoriesColors.title}}" + ":";
                var tdRight = document.createElement("TD");
                var input = content.createInput("text", "epgCategoriesColors", data.toString());
                input.setAttribute("placeholder", "{{.settings.epgCategoriesColors.placeholder}}");
                input.setAttribute("onchange", "javascript: this.className = 'changed'");
                tdRight.appendChild(input);
                setting.appendChild(tdLeft);
                setting.appendChild(tdRight);
                break;
            case "language":
                var tdLeft = document.createElement("TD");
                tdLeft.innerHTML = "{{.settings.language.title}}" + ":";
                var tdRight = document.createElement("TD");
                var text = ["English", "Español"];
                var values = ["en", "es"];
                var select = content.createSelect(text, values, data, settingsKey);
                select.setAttribute("onchange", "javascript: this.className = 'changed'");
                tdRight.appendChild(select);
                setting.appendChild(tdLeft);
                setting.appendChild(tdRight);
                break;
            case "accentColor":
                var tdLeft = document.createElement("TD");
                tdLeft.innerHTML = "{{.settings.accentColor.title}}" + ":";
                var tdRight = document.createElement("TD");
                var colorWrapper = document.createElement("DIV");
                colorWrapper.className = "accent-color-picker";
                var colorInput = document.createElement("INPUT");
                colorInput.setAttribute("type", "color");
                colorInput.setAttribute("name", "accentColor_picker");
                colorInput.setAttribute("value", data.toString() || "#d46c4a");
                colorInput.className = "color-wheel";
                colorInput.setAttribute("oninput", "javascript: var hex=document.getElementsByName('accentColor')[0]; hex.value=this.value; hex.className='changed'; this.className='color-wheel changed'; document.documentElement.style.setProperty('--accent', this.value);");
                colorWrapper.appendChild(colorInput);
                var hexInput = content.createInput("text", "accentColor", data.toString());
                hexInput.setAttribute("placeholder", "{{.settings.accentColor.placeholder}}");
                hexInput.setAttribute("oninput", "javascript: this.className = 'hex-input changed'; var picker=document.getElementsByName('accentColor_picker')[0]; if(this.value.match(/^#[0-9a-fA-F]{6}$/)){picker.value=this.value; document.documentElement.style.setProperty('--accent', this.value);}");
                hexInput.className = "hex-input";
                colorWrapper.appendChild(hexInput);
                tdRight.appendChild(colorWrapper);
                setting.appendChild(tdLeft);
                setting.appendChild(tdRight);
                break;
            case "fontSize":
                var tdLeft = document.createElement("TD");
                tdLeft.innerHTML = "{{.settings.fontSize.title}}" + ":";
                var tdRight = document.createElement("TD");
                var text = ["12px", "13px", "14px", "15px", "16px", "17px", "18px"];
                var values = ["12px", "13px", "14px", "15px", "16px", "17px", "18px"];
                var select = content.createSelect(text, values, data || "14px", settingsKey);
                select.setAttribute("onchange", "javascript: this.className = 'changed'; document.documentElement.style.setProperty('--font-size-base', this.value);");
                tdRight.appendChild(select);
                setting.appendChild(tdLeft);
                setting.appendChild(tdRight);
                break;
            case "ThreadfinAutoUpdate":
                var tdLeft = document.createElement("TD");
                tdLeft.innerHTML = "{{.settings.ThreadfinAutoUpdate.title}}" + ":";
                var tdRight = document.createElement("TD");
                var input = content.createCheckbox(settingsKey);
                input.checked = data;
                input.setAttribute("onchange", "javascript: this.className = 'changed'");
                tdRight.appendChild(input);
                setting.appendChild(tdLeft);
                setting.appendChild(tdRight);
                break;
            case "ssdp":
                var tdLeft = document.createElement("TD");
                tdLeft.innerHTML = "{{.settings.ssdp.title}}" + ":";
                var tdRight = document.createElement("TD");
                var input = content.createCheckbox(settingsKey);
                input.checked = data;
                input.setAttribute("onchange", "javascript: this.className = 'changed'");
                tdRight.appendChild(input);
                setting.appendChild(tdLeft);
                setting.appendChild(tdRight);
                break;
            case "dummy":
                var tdLeft = document.createElement("TD");
                tdLeft.innerHTML = "{{.settings.dummy.title}}" + ":";
                var tdRight = document.createElement("TD");
                var input = content.createCheckbox(settingsKey);
                input.checked = data;
                input.setAttribute("onchange", "javascript: this.className = 'changed'");
                tdRight.appendChild(input);
                setting.appendChild(tdLeft);
                setting.appendChild(tdRight);
                break;
            case "dummyChannel":
                var tdLeft = document.createElement("TD");
                tdLeft.innerHTML = "{{.settings.dummyChannel.title}}" + ":";
                var tdRight = document.createElement("TD");
                var text = ["PPV", "30 Minutes", "60 Minutes", "90 Minutes", "120 Minutes", "180 Minutes", "240 Minutes", "360 Minutes"];
                var values = ["PPV", "30_Minutes", "60_Minutes", "90_Minutes", "120_Minutes", "180_Minutes", "240_Minutes", "360_Minutes"];
                var select = content.createSelect(text, values, data, settingsKey);
                select.setAttribute("onchange", "javascript: this.className = 'changed'");
                tdRight.appendChild(select);
                setting.appendChild(tdLeft);
                setting.appendChild(tdRight);
                break;
            case "ignoreFilters":
                var tdLeft = document.createElement("TD");
                tdLeft.innerHTML = "{{.settings.ignoreFilters.title}}" + ":";
                var tdRight = document.createElement("TD");
                var input = content.createCheckbox(settingsKey);
                input.checked = data;
                input.setAttribute("onchange", "javascript: this.className = 'changed'");
                tdRight.appendChild(input);
                setting.appendChild(tdLeft);
                setting.appendChild(tdRight);
                break;
            case "api":
                var tdLeft = document.createElement("TD");
                tdLeft.innerHTML = "{{.settings.api.title}}" + ":";
                var tdRight = document.createElement("TD");
                var input = content.createCheckbox(settingsKey);
                input.checked = data;
                input.setAttribute("onchange", "javascript: this.className = 'changed'");
                tdRight.appendChild(input);
                setting.appendChild(tdLeft);
                setting.appendChild(tdRight);
                break;
            // Select
            case "tuner":
                var tdLeft = document.createElement("TD");
                tdLeft.innerHTML = "{{.settings.tuner.title}}" + ":";
                var tdRight = document.createElement("TD");
                var text = new Array();
                var values = new Array();
                for (var i = 1; i <= 100; i++) {
                    text.push(i);
                    values.push(i);
                }
                var select = content.createSelect(text, values, data, settingsKey);
                select.setAttribute("onchange", "javascript: this.className = 'changed'");
                tdRight.appendChild(select);
                setting.appendChild(tdLeft);
                setting.appendChild(tdRight);
                break;
            case "epgSource":
                var tdLeft = document.createElement("TD");
                tdLeft.innerHTML = "{{.settings.epgSource.title}}" + ":";
                var tdRight = document.createElement("TD");
                var text = ["PMS", "XEPG"];
                var values = ["PMS", "XEPG"];
                var select = content.createSelect(text, values, data, settingsKey);
                select.setAttribute("onchange", "javascript: this.className = 'changed'");
                tdRight.appendChild(select);
                setting.appendChild(tdLeft);
                setting.appendChild(tdRight);
                break;
            case "backup.keep":
                var tdLeft = document.createElement("TD");
                tdLeft.innerHTML = "{{.settings.backupKeep.title}}" + ":";
                var tdRight = document.createElement("TD");
                var text = ["5", "10", "20", "30", "40", "50"];
                var values = ["5", "10", "20", "30", "40", "50"];
                var select = content.createSelect(text, values, data, settingsKey);
                select.setAttribute("onchange", "javascript: this.className = 'changed'");
                tdRight.appendChild(select);
                setting.appendChild(tdLeft);
                setting.appendChild(tdRight);
                break;
            case "buffer.size.kb":
                var tdLeft = document.createElement("TD");
                tdLeft.innerHTML = "{{.settings.bufferSize.title}}" + ":";
                var tdRight = document.createElement("TD");
                var text = ["0.5 MB", "1 MB", "2 MB", "3 MB", "4 MB", "5 MB", "6 MB", "7 MB", "8 MB"];
                var values = ["512", "1024", "2048", "3072", "4096", "5120", "6144", "7168", "8192"];
                var select = content.createSelect(text, values, data, settingsKey);
                select.setAttribute("onchange", "javascript: this.className = 'changed'");
                tdRight.appendChild(select);
                setting.appendChild(tdLeft);
                setting.appendChild(tdRight);
                break;
            case "buffer":
                var tdLeft = document.createElement("TD");
                tdLeft.innerHTML = "{{.settings.streamBuffering.title}}" + ":";
                var tdRight = document.createElement("TD");
                var text = ["{{.settings.streamBuffering.info_false}}", "FFmpeg: ({{.settings.streamBuffering.info_ffmpeg}})", "VLC: ({{.settings.streamBuffering.info_vlc}})"];
                var values = ["-", "ffmpeg", "vlc"];
                var select = content.createSelect(text, values, data, settingsKey);
                select.setAttribute("onchange", "javascript: this.className = 'changed'");
                tdRight.appendChild(select);
                setting.appendChild(tdLeft);
                setting.appendChild(tdRight);
                break;
            case "udpxy":
                var tdLeft = document.createElement("TD");
                tdLeft.innerHTML = "{{.settings.udpxy.title}}" + ":";
                var tdRight = document.createElement("TD");
                var input = content.createInput("text", "udpxy", data);
                input.setAttribute("placeholder", "{{.settings.udpxy.placeholder}}");
                input.setAttribute("onchange", "javascript: this.className = 'changed'");
                tdRight.appendChild(input);
                setting.appendChild(tdLeft);
                setting.appendChild(tdRight);
                break;
        }
        return setting;
    }
    createDescription(settingsKey) {
        var description = document.createElement("TR");
        var text;
        switch (settingsKey) {
            case "authentication.web":
                text = "{{.settings.authenticationWEB.description}}";
                break;
            case "authentication.m3u":
                text = "{{.settings.authenticationM3U.description}}";
                break;
            case "authentication.pms":
                text = "{{.settings.authenticationPMS.description}}";
                break;
            case "authentication.xml":
                text = "{{.settings.authenticationXML.description}}";
                break;
            case "authentication.api":
                if (SERVER["settings"]["authentication.web"] == true) {
                    text = "{{.settings.authenticationAPI.description}}";
                }
                break;
            case "ThreadfinAutoUpdate":
                text = "{{.settings.ThreadfinAutoUpdate.description}}";
                break;
            case "backup.keep":
                text = "{{.settings.backupKeep.description}}";
                break;
            case "backup.path":
                text = "{{.settings.backupPath.description}}";
                break;
            case "temp.path":
                text = "{{.settings.tempPath.description}}";
                break;
            case "buffer":
                text = "{{.settings.streamBuffering.description}}";
                break;
            case "buffer.size.kb":
                text = "{{.settings.bufferSize.description}}";
                break;
            case "storeBufferInRAM":
                text = "{{.settings.storeBufferInRAM.description}}";
                break;
            case "forceHttps":
                text = "{{.settings.forceHttps.description}}";
                break;
            case "excludeStreamHttps":
                text = "{{.settings.excludeStreamHttps.description}}";
                break;
            case "httpsPort":
                text = "{{.settings.httpsPort.description}}";
                break;
            case "httpsThreadfinDomain":
                text = "{{.settings.httpsThreadfinDomain.description}}";
                break;
            case "bindIpAddress":
                text = "{{.settings.bindIpAddress.description}}";
                break;
            case "httpThreadfinDomain":
                text = "{{.settings.httpThreadfinDomain.description}}";
                break;
            case "enableNonAscii":
                text = "{{.settings.enableNonAscii.description}}";
                break;
            case "epgCategories":
                text = "{{.settings.epgCategories.description}}";
                break;
            case "epgCategoriesColors":
                text = "{{.settings.epgCategoriesColors.description}}";
                break;
            case "accentColor":
                text = "{{.settings.accentColor.description}}";
                break;
            case "fontSize":
                text = "{{.settings.fontSize.description}}";
                break;
            case "language":
                text = "{{.settings.language.description}}";
                break;
            case "buffer.timeout":
                text = "{{.settings.bufferTimeout.description}}";
                break;
            case "user.agent":
                text = "{{.settings.userAgent.description}}";
                break;
            case "ffmpeg.path":
                text = "{{.settings.ffmpegPath.description}}";
                break;
            case "ffmpeg.options":
                text = "{{.settings.ffmpegOptions.description}}";
                break;
            case "ffmpeg.forceHttp":
                text = "{{.settings.ffmpegForceHttp.description}}";
                break;
            case "vlc.path":
                text = "{{.settings.vlcPath.description}}";
                break;
            case "vlc.options":
                text = "{{.settings.vlcOptions.description}}";
                break;
            case "epgSource":
                text = "{{.settings.epgSource.description}}";
                break;
            case "tuner":
                text = "{{.settings.tuner.description}}";
                break;
            case "update":
                text = "{{.settings.update.description}}";
                break;
            case "api":
                text = "{{.settings.api.description}}";
                break;
            case "ssdp":
                text = "{{.settings.ssdp.description}}";
                break;
            case "files.update":
                text = "{{.settings.filesUpdate.description}}";
                break;
            case "cache.images":
                text = "{{.settings.cacheImages.description}}";
                break;
            case "xepg.replace.missing.images":
                text = "{{.settings.replaceEmptyImages.description}}";
                break;
            case "xepg.replace.channel.title":
                text = "{{.settings.replaceChannelTitle.description}}";
                break;
            case "udpxy":
                text = "{{.settings.udpxy.description}}";
                break;
            default:
                text = "";
                break;
        }
        var tdLeft = document.createElement("TD");
        tdLeft.innerHTML = "";
        var tdRight = document.createElement("TD");
        var pre = document.createElement("PRE");
        pre.innerHTML = text;
        tdRight.appendChild(pre);
        description.appendChild(tdLeft);
        description.appendChild(tdRight);
        return description;
    }
}
class SettingsCategoryItem extends SettingsCategory {
    constructor(headline, settingsKeys) {
        super();
        this.headline = headline;
        this.settingsKeys = settingsKeys;
    }
    createCategory() {
        var headline = this.createCategoryHeadline(this.headline);
        var settingsKeys = this.settingsKeys;
        var doc = document.getElementById(this.DocumentID);
        doc.appendChild(headline);
        // Tabelle für die Kategorie erstellen
        var table = document.createElement("TABLE");
        var keys = settingsKeys.split(",");
        keys.forEach(settingsKey => {
            switch (settingsKey) {
                case "authentication.pms":
                case "authentication.m3u":
                case "authentication.xml":
                case "authentication.api":
                    if (SERVER["settings"]["authentication.web"] == false) {
                        break;
                    }
                default:
                    var item = this.createSettings(settingsKey);
                    var description = this.createDescription(settingsKey);
                    table.appendChild(item);
                    table.appendChild(description);
                    break;
            }
        });
        doc.appendChild(table);
        doc.appendChild(this.createHR());
    }
}
function showSettings() {
    console.log("SETTINGS");
    for (let i = 0; i < settingsCategory.length; i++) {
        settingsCategory[i].createCategory();
    }
}
function saveSettings() {
    console.log("Save Settings");
    var cmd = "saveSettings";
    var div = document.getElementById("content_settings");
    var settings = div.getElementsByClassName("changed");
    var newSettings = new Object();
    for (let i = 0; i < settings.length; i++) {
        var name;
        var value;
        switch (settings[i].tagName) {
            case "INPUT":
                switch (settings[i].type) {
                    case "checkbox":
                        name = settings[i].name;
                        value = settings[i].checked;
                        newSettings[name] = value;
                        break;
                    case "text":
                        name = settings[i].name;
                        value = settings[i].value;
                        switch (name) {
                            case "update":
                                value = value.split(",");
                                value = value.filter(function (e) { return e; });
                                break;
                            case "buffer.timeout":
                                value = parseFloat(value);
                                break;
                        }
                        newSettings[name] = value;
                        break;
                    case "color":
                        // Skip color picker inputs (we use the text hex input)
                        break;
                }
                break;
            case "SELECT":
                name = settings[i].name;
                value = settings[i].value;
                if (isNaN(value)) {
                    newSettings[name] = value;
                }
                else {
                    newSettings[name] = parseInt(value);
                }
                break;
        }
    }
    if (Object.keys(newSettings).length === 0) {
        showSaveConfirmation("{{.settings.noChanges}}");
        return;
    }
    var data = new Object();
    data["settings"] = newSettings;
    var server = new Server(cmd);
    server.request(data);
    // Show success toast
    showSaveConfirmation();
}
function showSaveConfirmation(message) {
    var existing = document.getElementById("save-toast");
    if (existing) existing.remove();
    var toast = document.createElement("DIV");
    toast.id = "save-toast";
    toast.className = message ? "save-toast save-toast-info" : "save-toast";
    toast.innerHTML = message || "{{.settings.savedSuccess}}";
    document.body.appendChild(toast);
    // Trigger animation
    setTimeout(function() { toast.classList.add("show"); }, 10);
    setTimeout(function() {
        toast.classList.remove("show");
        setTimeout(function() { toast.remove(); }, 300);
    }, 3000);
}
