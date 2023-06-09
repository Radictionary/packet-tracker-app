startSSE() //start inital connection with server

let scrollDown = true //start interface with interface lock set to on
let lengthState = true

document.getElementById("controls").addEventListener("submit", function (event) {
    applySettings(event);
});

document.getElementById("interfaceInput").addEventListener("input", function (event) {
    applySettings(event);
});

document.getElementById("protocols").addEventListener("input", function (event) {
    applySettings(event);
});



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



//Keyboard Shortcuts
const tableView = document.getElementById("tableView")
document.addEventListener('keydown', function (event) {
    if (event.key === "Enter" || event.key === "Return") {
        if (scrollDown == false) {
            scrollDown = true
            tableView.innerText = "(locked)"
            window.scrollBy({
                top: packetTable.offsetHeight, // Scroll to the the end of the table's height
                behavior: "auto"
            });
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
        console.log("The key 't' is pressed!")
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
    statusMessage.innerText = "Stopped"
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
        })
        .catch(error => {
            console.error('Error sending POST request:', error);
            statusMessage.innerText = "Failed to send stop message"
        });
}
stopButton.addEventListener('click', function () {
    stopProgram()
});
restartButton.addEventListener("click", function () {
    let data = { key: 'reset' };
    let requestOptions = {
        method: 'POST',
        headers: {
            'Content-Type': 'application/json',
        },
        body: JSON.stringify(data)
    };
    fetch("/change", requestOptions)
        .then(response => {
            statusMessage.innerText = "Restarted!"
        })
        .catch(error => {
            console.error('Error sending POST request:', error);
            statusMessage.innerText = "Failed to restart"
        });
})
function removeAllRows() {
    var rowCount = packetTable.rows.length;
    // Start from the last row and remove each row
    for (var i = rowCount - 1; i > 0; i--) {
        packetTable.deleteRow(i);
    }
}
clearButton.addEventListener("click", function () {
    removeAllRows()
    statusMessage.innerText = "Cleared"
    fetch(`/packetinfo?packetnumber=clear`)
        .catch(error => {
            statusMessage.innerText = "Couldn't Clear";
        });
    popupContainer.style.height = '0';
    popupBox.style.height = '0';
    popupBox.style.visibility = 'hidden';
})



//Table
const tableBody = document.getElementById('table-body');
const maxRows = 2000;
const minPacketRate = 10; // Minimum packet rate to process
let packetRate = 50; // Start with a packet rate of 50 packets per second
let lastTime = performance.now();
let startedInt = 0

function startSSE() {
    settingsSync()
    const es = new EventSource("/packet");
    es.addEventListener("error", event => {
        statusMessage.innerText = "Stopped on an error";
    });
    es.addEventListener('new-packet', (event) => {
        const jsonData = event.data
        const data = JSON.parse(jsonData)
        appendingTable(data)
    })
}

function appendingTable(data) {
    if (startedInt <= 3) {
        statusMessage.innerText = "Started"
    }

    // Update the packet rate based on the current load on the browser
    const currentTime = performance.now();
    const elapsedTime = currentTime - lastTime;
    lastTime = currentTime;
    const expectedTimePerPacket = 1000 / packetRate;
    const loadFactor = elapsedTime / expectedTimePerPacket;
    packetRate = packetRate * loadFactor;

    // Clamp the packet rate to a reasonable range
    packetRate = Math.max(minPacketRate, Math.min(packetRate, 1000));

    // Determine the number of packets to process based on the packet rate
    const packetsToProcess = Math.max(1, Math.floor(packetRate / 1000));

    // Process the specified number of packets
    for (let i = 0; i < packetsToProcess; i++) {
        const row = document.createElement('tr');
        const interface = document.createElement('td');
        const protocol = document.createElement('td');
        const srcAddr = document.createElement('td');
        const dstnAddr = document.createElement('td');
        const length = document.createElement('td');
        const packetNumber = document.createElement('td');
        const timeCell = document.createElement('td');


        if (data.err) {
            statusMessage.innerText = data.err
        }

        interface.innerText = data.interface;
        protocol.innerText = data.protocol;
        srcAddr.innerText = data.srcAddr;
        dstnAddr.innerText = data.dstnAddr;
        if (lengthState) { length.innerText = data.length; }
        packetNumber.innerText = data.packetNumber;
        timeCell.innerText = data.time;

        row.appendChild(interface);
        row.appendChild(protocol);
        if (lengthState) { row.appendChild(length); console.log("Appending to row, and the variable of length is set to: " + lengthState) }
        row.appendChild(srcAddr);
        row.appendChild(dstnAddr);
        row.appendChild(packetNumber);
        row.appendChild(timeCell);
        tableBody.appendChild(row);
        if (tableBody.children.length > maxRows) {
            const excessRows = tableBody.children.length - maxRows;
            for (let i = 0; i < excessRows; i++) {
                tableBody.removeChild(tableBody.firstChild);
            }
        }

        // Add click event listener to the row
        row.addEventListener('click', () => {
            let packetNumberSelected = packetNumber.innerText;
            fetch(`/packetinfo?packetnumber=${packetNumberSelected}`)
                .then(response => {
                    if (!response.ok) {
                        throw new Error("Server returned an error");
                    }
                    return response.json();
                })
                .then(data => {
                    // Process the successful response
                    showPopupBox(data.packetNumber, data.packetDump);

                    console.log(data.packetDump)
                })
                .catch(error => {
                    // Handle the error case
                    statusMessage.innerText = "Couldn't retrieve information about the packet";
                    console.error(error);
                });

        });
        if (scrollDown) {
            window.scrollBy({
                top: packetTable.offsetHeight, // Scroll to the the end of the table's height
                behavior: "auto"
            });
        }
    }
    startedInt++
}



//Popup box
const closeButton = document.getElementById('close');
const popupContainer = document.getElementById('popupContainer');
const popupBox = document.getElementById('popupBox');
const dragHandle = document.getElementById('dragHandle');
const packetDumpOutput = document.getElementById('packetDumpOutput');
const packetNumber = document.getElementById("packetNumber")
popupContainer.style.height = '0';
popupBox.style.height = '0';
function showPopupBox(number, data) {
    var startY, startHeight;
    // Show the popup box
    popupContainer.style.height = 'auto';
    popupBox.style.height = 'auto'; // Initial height
    popupBox.style.visibility = 'visible'

    // Set the packet number and data
    packetNumber.innerText = number;
    // packetDumpOutput.innerText = data;
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
    })


}



//retrieves user settings from the backend
const selectFilterField = document.getElementById("protocols");
function settingsSync() {
    fetch(`/settings`)
        .then(response => {
            if (!response.ok) {
                throw new Error("Server returned an error");
            }
            return response.json();
        })
        .then(data => {
           
            interfaceInput.value = data.interface;
            interfaceInput.placeholder = data.interface;
            var selectedOptions = data.filter.split(" or ");
            // Iterate through each <option> element and set the selected attribute
            Array.from(selectFilterField.options).forEach(function (option) {
                if (selectedOptions.includes(option.value)) {
                    option.selected = true;
                }
            });
            if (data.packetSaveDir === "") {
                savePacketsButton.checked = false
                document.getElementById("savingToFileNotification").style.visibility = "hidden"
            } else {
                savePacketsButton.checked = true
                document.getElementById("savingToFileNotification").innerText = "Saving to: " + data.packetSaveDir + ".pcap"
            }
        })
        .catch(error => {
            statusMessage.innerText = "Couldn't retrieve user settings";
            console.log(error)
        });
}

selectFilterField.addEventListener("blur", function () {
    settingsSync()
});

function beutficalDisplay(packetData) {
    // Parse packet data into an object with named layers
    function parsePacketData(packetData) {
        const packetDataLines = packetData.split('\n');
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

    // Parse packet data and generate dropdowns
    const parsedPacketData = parsePacketData(packetData);
    const dropdownsHTML = generateDropdowns(parsedPacketData);

    // Display the dropdowns on the page
    const container = document.getElementById('packet-container');
    container.innerHTML = dropdownsHTML;

}


//Saving and Uploading pcap files
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

function uploadFile(file) {
    var formData = new FormData();
    formData.append('file', file);

    fetch('/upload', {
        method: 'POST',
        body: formData
    })
        .then(function (response) {
            if (response.ok) {
                console.log('File uploaded successfully.');
                // Perform any necessary UI updates
            } else {
                console.error('Failed to upload file:', response.statusText);
                // Perform any necessary UI updates
            }
        })
        .catch(function (error) {
            console.error('Failed to upload file:', error);
            // Perform any necessary UI updates
        });
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
