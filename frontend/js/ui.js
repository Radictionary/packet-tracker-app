let scrollDown = true //start interface with interface lock set to on
let lengthState = true



function applySettings(event) {
    event.preventDefault();
    const form = event.target.form;
    const formData = new FormData(form);

    fetch("/change", {
        method: "POST",
        body: new URLSearchParams(formData), // Convert FormData to URL-encoded format
        headers: {
            "Content-Type": "application/x-www-form-urlencoded"
        }
    })
        .then(() => {
            statusMessage.innerText = "Applied!";
        })
        .catch(error => {
            statusMessage.innerText = "Error changing selected settings";
        });
}


document.getElementById("controls").addEventListener("submit", function (event) {
    applySettings(event);
});

document.getElementById("interfaceInput").addEventListener("input", function (event) {
    applySettings(event);
});

document.getElementById("protocols").addEventListener("input", function (event) {
    applySettings(event);
});



//Keyboard Shortcuts
const tableView = document.getElementById("tableView")
document.addEventListener('keydown', function (event) {
    if (event.key === "Enter" || event.key === "Return") {
        if (scrollDown == false) {
            scrollDown = true
            tableView.innerText = "(locked)"
            // window.scrollBy({
            //     top: packetTable.offsetHeight, // Scroll to the the end of the table's height
            //     behavior: "auto"
            // });
        } else {
            scrollDown = false
            tableView.innerText = "(unlocked)"
        }
    }
});
document.addEventListener("keydown", function (event) {
    if (event.key === "t") {
        window.scrollBy({
            top: -packetTable.offsetHeight, // Scroll to the the top of the table's height
            behavior: "auto"
        });
    }
});



//Control Buttons
const controls = document.getElementById("controls");
const packetTable = document.querySelector('#packetTable');
const statusMessage = document.getElementById("status")
const customization = document.getElementById("customization")
const stopButton = document.getElementById('stop');
const startButton = document.getElementById('start');
const clearButton = document.getElementById('clear');
const restartButton = document.getElementById('restart');
const saveButton = document.getElementById("save");
const savePacketsButton = document.getElementById('save_method');
startButton.addEventListener('click', function () {
    settingsSync()
    statusMessage.innerText = "Started"
    let data = { key: 'start' };
    let requestOptions = {
        method: 'POST',
        headers: {
            'Content-Type': 'application/json',
        },
        body: JSON.stringify(data)
    }
    fetch("/change", requestOptions)
        .catch(error => {
            console.error('Error sending POST request:', error);
            statusMessage.innerText = "Failed to send start message"
        });
});
function stopProgram() {
    let data = { key: 'stop' };
    let requestOptions = {
        method: 'POST',
        headers: {
            'Content-Type': 'application/json',
        },
        body: JSON.stringify(data)
    };
    fetch("/change", requestOptions)
        .then(response => {
            if (response.ok) {
                statusMessage.innerText = "Stopped"
            }
        })
        .catch(error => {
            console.error('Error sending POST request:', error);
            statusMessage.innerText = "Failed to send stop message"
        });
}
stopButton.addEventListener('click', function () {
    stopProgram()
});
function removeAllRows() {
    var rowCount = packetTable.rows.length;
    // Start from the last row and remove each row
    for (var i = rowCount - 1; i > 0; i--) {
        packetTable.deleteRow(i);
    }
}
clearButton.addEventListener("click", function () {
    removeAllRows()
    fetch(`/packetsearch?packetnumber=clear`)
        .then(response => {
            if (response.ok) {
                statusMessage.innerText = "Cleared"
            }
        })
        .catch(error => {
            statusMessage.innerText = "Couldn't Clear";
            console.error("Coulnd't clear" + error)
        });
    popupContainer.style.height = '0';
    popupBox.style.height = '0';
    popupBox.style.visibility = 'hidden';
})
saveButton.addEventListener("click", function () {
    var fullPath = prompt("Choose a directory to save the file, including the file name:");
    let data = { "save": fullPath }
    let requestOptions = {
        method: 'POST',
        headers: {
            'Content-Type': 'application/json',
        },
        body: JSON.stringify(data)
    }
    statusMessage.innerText = "Saving..."
    fetch("/change", requestOptions)
        .then(response => {
            if (response.ok) {
                statusMessage.innerText = "Saved"
            }
        })
        .catch(error => {
            console.error('Error sending POST request:', error);
            statusMessage.innerText = "Failed to save"
        });
})




//Popup box
const closeButton = document.getElementById('close');
const popupContainer = document.getElementById('popupContainer');
const popupBox = document.getElementById('popupBox');
const dragHandle = document.getElementById('dragHandle');
popupContainer.style.height = '0';
popupBox.style.height = '0';
function showPopupBox(number, type, length, saved, data, dnsInformation) {
    var startY, startHeight;
    // Show the popup box
    popupContainer.style.height = 'auto';
    popupBox.style.height = 'auto'; // Initial height
    popupBox.style.visibility = 'visible'

    // Set the packet number and data
    if (saved == 1) {
        saved = "true"
    } else {
        saved = "false"
    }
    document.getElementById("packetNumber").innerText = number
    document.getElementById("packetType").innerText = type
    document.getElementById("packetLength").innerText = length
    document.getElementById("packetSaved").innerText = saved
    if (dnsInformation != null) {
        document.getElementById("details").style.visibility = "visible"
        document.getElementById("packetDnsInformation").innerText = dnsInformation
    }
    beutficalDisplay(data)
    // Make the box resizable
    dragHandle.addEventListener('mousedown', startDrag, false);
    function startDrag(e) {
        e.preventDefault();
        startY = e.clientY;
        startHeight = parseInt(window.getComputedStyle(popupBox).height, 10);
        document.addEventListener('mousemove', doDrag, false);
        document.addEventListener('mouseup', stopDrag, false);
    }
    function doDrag(e) {
        var newHeight = startHeight + (startY - e.clientY);
        popupBox.style.height = newHeight + 'px';
    }
    function stopDrag() {
        document.removeEventListener('mousemove', doDrag, false);
        document.removeEventListener('mouseup', stopDrag, false);
    }
    closeButton.addEventListener('click', function () {
        popupContainer.style.height = '0';
        popupBox.style.height = '0';
        popupBox.style.visibility = 'hidden';
        document.getElementById("details").style.visibility = "hidden"
    })
}



var checkbox = document.getElementById('includeLength');
var lengthRow = document.getElementById('lengthRow');

checkbox.addEventListener('change', function () {
    if (checkbox.checked) {
        lengthRow.style.display = 'table-cell';
        lengthState = true
    } else {
        lengthRow.style.display = 'none';
        lengthState = false
    }
});



function beutficalDisplay(packetData) {
    // Parse packet data into an object with named layers
    let packetDataLines
    function parsePacketData(packetData) {
        if (packetData != null) {
            packetDataLines = packetData.split('\n');
        }
        const packetInfo = {};
        let currentLayer = null;
        let currentContent = '';

        for (const line of packetDataLines) {
            if (line.startsWith('---')) {
                if (currentLayer) {
                    packetInfo[currentLayer.layerName] = currentContent;
                }
                currentLayer = extractLayerInfo(line);
                currentContent = '';
            } else {
                currentContent += line + '\n';
            }
        }

        if (currentLayer) {
            packetInfo[currentLayer.layerName] = currentContent;
        }

        return packetInfo;
    }

    // Extract layer name from the line
    function extractLayerInfo(line) {
        const match = line.match(/---\s+(.+)\s+---/);
        if (match) {
            const layerName = match[1].trim();
            return { layerName };
        }
        return null;
    }

    // Generate the HTML for the dropdowns
    function generateDropdowns(packetInfo) {
        let html = '';

        for (const layer in packetInfo) {
            html += `
      <details open>
        <summary>${layer}</summary>
        <pre>${packetInfo[layer]}</pre>
      </details>
    `;
        }

        return html;
    }

    const parsedPacketData = parsePacketData(packetData);
    const dropdownsHTML = generateDropdowns(parsedPacketData);

    const container = document.getElementById('packet-container');
    container.innerHTML = dropdownsHTML;
}

//Saving
function showFileDialog(checked) {
    if (checked) {
        var fullPath = prompt("Choose a directory to save the file, including the file name:");
        if (fullPath != null) {
            const comfirmation = confirm("Are you sure you want to save the packets to " + fullPath + ".pcap?");
            if (comfirmation) {
                fetch("/change", {
                    method: 'POST',
                    body: JSON.stringify({ fullPath }),
                })
                    .then(() => {
                        statusMessage.innerText = "Saving packets from now on"
                        document.getElementById("savingToFileNotification").style.visibility = "visible"
                        document.getElementById("savingToFileNotification").innerText = "Saving to: " + fullPath
                    })
                    .catch(error => {
                        console.error('Error sending POST request:', error);
                        statusMessage.innerText = "Failed to start saving packets"
                    });
            }
        }
    } else {
        fullPath = ""
        fetch("/change", {
            method: 'POST',
            body: JSON.stringify({ fullPath }),
        })
            .then(() => {
                document.getElementById("savingToFileNotification").style.visibility = "hidden"
                statusMessage.innerText = "Not saving packets anymore to the file";
            })
            .catch(error => {
                console.error('Error sending POST request:', error);
                statusMessage.innerText = "Failed to stop saving packets"
            });
    }
}

document.getElementById('pcapUpload').addEventListener('click', function () {
    document.getElementById('fileInput').click();
});

document.getElementById('fileInput').addEventListener('change', function (event) {
    var file = event.target.files[0];
    if (file) {
        uploadFile(file);
    }
});


function startTimer(checked) {
    if (checked) {
        document.getElementById("timerSelection").style.visibility = "visible"
    } else {
        document.getElementById("timerSelection").style.visibility = "hidden"
    }
}

setInterval(() => {
    tableView.innerText = ""
}, 5000);