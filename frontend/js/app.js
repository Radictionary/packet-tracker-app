startSSE() //start inital connection with server

//Table
const tableBody = document.getElementById('table-body');
const tableContainer = document.querySelector('.table-container');
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
        console.error(event)
    });
    es.addEventListener('new-packet', (event) => {
        const jsonData = event.data
        const data = JSON.parse(jsonData)
        appendingTable(data)
    })
}

function appendingTable(data) {
    if (startedInt <= 2) {
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
        if (lengthState) { row.appendChild(length) }
        row.appendChild(srcAddr);
        row.appendChild(dstnAddr);
        row.appendChild(packetNumber);
        row.appendChild(timeCell);
        tableBody.appendChild(row);
        if (scrollDown) { tableContainer.scrollTop = tableContainer.scrollHeight; }
        if (tableBody.children.length > maxRows) {
            const excessRows = tableBody.children.length - maxRows;
            for (let i = 0; i < excessRows; i++) {
                tableBody.removeChild(tableBody.firstChild);
            }
        }

        row.addEventListener('click', () => {
            let packetNumberSelected = packetNumber.innerText;
            fetch(`packetsearch?packetnumber=${packetNumberSelected}`)
                .then(response => {
                    if (!response.ok) {
                        throw new Error("Could not get packet information");
                    }
                    return response.json();
                })
                .then(data => {
                    showPopupBox(data.packetNumber, data.protocol, data.length, data.saved, data.packetDump);
                })
                .catch(error => {
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


fetch("/retrieve?get=recover")
    .then(response => response.json())
    .then(data => {
        let recoverdPacketsNum = data.length
        if (data.length === 0) {
            statusMessage.innerText = "No recovered packets"
        } else {
            data.forEach(packet => {
                const recov_row = tableBody.insertRow();
                const recov_interface = recov_row.insertCell();
                const recov_protocol = recov_row.insertCell();
                const recov_length = recov_row.insertCell();
                const recov_srcAddr = recov_row.insertCell();
                const recov_dstnAddr = recov_row.insertCell();
                const recov_packetNumber = recov_row.insertCell();
                const recov_timeCell = recov_row.insertCell();
                recov_interface.innerText = packet.interface;
                recov_protocol.innerText = packet.protocol;
                recov_srcAddr.innerText = packet.srcAddr;
                recov_dstnAddr.innerText = packet.dstnAddr;
                if (lengthState) { recov_length.innerText = packet.length; }
                recov_packetNumber.innerText = packet.packetNumber;
                recov_timeCell.innerText = packet.time;
                recov_row.addEventListener('click', () => {
                    let packetNumberSelected = recov_packetNumber.innerText;
                    fetch(`/packetinfo?packetnumber=${packetNumberSelected}`)
                        .then(response => {
                            if (!response.ok) {
                                throw new Error("Could not get packet information");
                            }
                            return response.json();
                        })
                        .then(data => {
                            showPopupBox(data.packetNumber, data.protocol, data.length, data.saved, data.packetDump);

                        })
                        .catch(error => {
                            statusMessage.innerText = "Couldn't retrieve information about the packet";
                            console.error(error);
                        });

                });
            });
            statusMessage.innerText = "Recovered " + (recoverdPacketsNum) + " unsaved packets";
        }
        setTimeout(function () {
            statusMessage.innerText = "Waiting for start"
        }, 850)
    })
    .catch(error => {
        statusMessage.innerText = "Failed to recover packets";
        console.error(error);
    });

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
            console.Error(error)
        });
}

selectFilterField.addEventListener("blur", function () {
    settingsSync()
});

function uploadFile(file) {
    var formData = new FormData();
    formData.append('file', file);

    fetch('/upload', {
        method: 'POST',
        body: formData
    })
        .then(function (response) {
            if (!response.ok) {
                statusMessage.innerText = "Failed to upload file"
                console.error('Failed to upload file:', response.statusText);
            } else {
                statusMessage.innerText = "Retrieving file contents..."
                fetch("/retrieve?get=filecontents")
                    .then(response => response.json())
                    .then(data => {
                        let filePacketsNum = data.length
                        if (data.length === 0) {
                            statusMessage.innerText = "No packets retrieved from file"
                        } else {
                            data.forEach(packet => {
                                const file_row = tableBody.insertRow();
                                const file_interface = file_row.insertCell();
                                const file_protocol = file_row.insertCell();
                                const file_length = file_row.insertCell();
                                const file_srcAddr = file_row.insertCell();
                                const file_dstnAddr = file_row.insertCell();
                                const file_packetNumber = file_row.insertCell();
                                const file_timeCell = file_row.insertCell();
                                file_interface.innerText = packet.interface;
                                file_protocol.innerText = packet.protocol;
                                file_srcAddr.innerText = packet.srcAddr;
                                file_dstnAddr.innerText = packet.dstnAddr;
                                if (lengthState) { file_length.innerText = packet.length; }
                                file_packetNumber.innerText = packet.packetNumber;
                                file_timeCell.innerText = packet.time;
                                file_row.addEventListener('click', () => {
                                    let packetNumberSelected = file_packetNumber.innerText;
                                    fetch(`/packetinfo?packetnumber=${packetNumberSelected}`)
                                        .then(response => {
                                            if (!response.ok) {
                                                throw new Error("Could not get packet information");
                                            }
                                            return response.json();
                                        })
                                        .then(data => {
                                            showPopupBox(data.packetNumber, data.protocol, data.length, data.saved, data.packetDump);

                                        })
                                        .catch(error => {
                                            statusMessage.innerText = "Couldn't retrieve information about the packet";
                                            console.error(error);
                                        });

                                });
                            });
                            statusMessage.innerText = "Retrieved " + (filePacketsNum) + " from file";
                        }
                        setTimeout(function () {
                            statusMessage.innerText = "Waiting for start"
                        }, 850)
                    })
                    .catch(error => {
                        statusMessage.innerText = "Failed to recover packets";
                        console.error(error);
                    });
            }
        })
        .catch(function (error) {
            console.error('Failed to upload file:', error);
        });
}
