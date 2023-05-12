// const initialSSEConnection = new EventSource("/event")
// initialSSEConnection.addEventListener("error", event => {
//     statusMessage.innerText = "Couldn't make initializing SSE connection";
// });
// initialSSEConnection.onopen = function () {
//     console.log("made the init connection")
//     setTimeout(() => {
//         console.log("Waiting 1 seconds")
//     }, 1000);
//     initialSSEConnection.close()
// };

//Addresses the finished starting issue. Another way could be to make backend redirect only when that is done...maybe?
// fetch('/interface', {
//     method: 'POST',
//     headers: {
//         'Content-Type': 'application/json'
//     },
//     body: JSON.stringify("start")
// })
//     .then(response => {
//         console.log("Sent init message")
//     })
//     .catch(error => {
//         console.error('Error sending POST request:', error);
//         statusMessage.innerText = "Failed to send start message"
//     });
let scrollDown = true
let stopCounter = 0
let listening = false
let sse_clean_done = false
const csrfTokenInput = document.querySelector('input[name="csrf_token"]');
const csrfToken = csrfTokenInput.value;

//Clear all the packetDumps set before
let itemsToRemove = localStorage.getItem("setValues")
for (i = 0; i <= itemsToRemove; i++) {
    localStorage.removeItem(i)
}



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


//Interface selections and Customization
const controls = document.getElementById("controls");
const checkbox = controls.elements.time_method;
checkbox.checked = localStorage.getItem('time_method') === 'on';

let interfaceInput = controls.elements.interface;
let interfaceInLocalStorage = localStorage.getItem('interfaceValue');
if (interfaceInLocalStorage == null) {
    interfaceInput.placeholder = "en0"
    interfaceInput.value = "en0"
} else {
    interfaceInput.value = interfaceInLocalStorage;
    interfaceInput.placeholder = interfaceInLocalStorage;
}

controls.addEventListener('submit', function (event) {
    localStorage.setItem('interfaceValue', interfaceInput.value);
    if (checkbox.checked) {
        localStorage.setItem('time_method', 'on');
    } else {
        localStorage.removeItem('time_method');
    }

    es.close();
    //console.log("ES IS NOW CLOSED")
    form.submit();
});


//Control Buttons
const packetTable = document.querySelector('#packetTable');
const statusMessage = document.getElementById("status")
const customization = document.getElementById("customization")
const stopButton = document.getElementById('stop');
const startButton = document.getElementById('start');
const clearButton = document.getElementById('clear');
function stopProgram() {
    listening = false
    es.close();
    console.log("ES IS CLOSED")
    statusMessage.innerText = "Stopped"
    let data = { key: 'stop' };
    let url = '/interface';
    let requestOptions = {
        method: 'POST',
        headers: {
            'Content-Type': 'application/json',
        },
        body: JSON.stringify(data)
    };
    fetch(url, requestOptions)
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
startButton.addEventListener('click', function () {
    listening = true
    sse_clean_done = false
    statusMessage.innerText = "Started"
    let data = { key: 'start' };
    let url = '/interface';
    let requestOptions = {
        method: 'POST',
        headers: {
            'Content-Type': 'application/json',
        },
        body: JSON.stringify(data)
    }
    fetch(url, requestOptions)
        .then(response => {
        })
        .catch(error => {
            console.error('Error sending POST request:', error);
            statusMessage.innerText = "Failed to send start message"
        });
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
    statusMessage.innerText = "Cleared"
    fetch(`http://localhost:8080/packetinfo?packetnumber=clear`)
        .catch(error => {
            statusMessage.innerText = "Couldn't Clear";
            console.error(error); // Log any errors to the console
        });
    popupContainer.style.display = "none"
})


const es = new EventSource('/event?csrf_token=' + encodeURIComponent(csrfToken));
const tableBody = document.getElementById('table-body');
const maxRows = 2000;
const minPacketRate = 10; // Minimum packet rate to process
let packetRate = 50; // Start with a packet rate of 50 packets per second
let lastTime = performance.now();
let startedInt = 0
let localStorageKeys = [];


es.addEventListener("error", event => {
    statusMessage.innerText = "Stopped on an error";
});
es.addEventListener('price-update', (event) => {
    const jsonStr = event.data.replace('data:', '').trim(); // Remove the "data:" prefix and any leading/trailing whitespace
    const data = JSON.parse(JSON.parse(jsonStr).data); // Parse the JSON string twice to extract the data property]
    appendingTable(data)
})
function appendingTable(data) {
    console.log(data)
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
        const packetNumber = document.createElement('td');
        const timeCell = document.createElement('td');

        interface.innerText = data.interface;

        if (data.interface == 'SSE_CLEAN') {
            if (data.srcAddr == "done") {
                statusMessage.innerText = "Finished setting up"
                sse_clean_done = true
            }
            return;
        }
        if (data.err) {
            statusMessage.innerText = data.err
        }
        setTimeout(function () {
            if (!sse_clean_done) {
                console.log("SSE CLEAN IS NOT DONE BUT PROGRAM STOPPED")
                //es.close()
                //location.reload()
            }
        }, 3000);
        if (data.interface != interfaceInLocalStorage && interfaceInLocalStorage != null && data.interface != "SSE_CLEAN") {
            statusMessage.innerText = "dev: recieving information not relating to selected interface. interfaceInLocalStorage is: ", interfaceInLocalStorage;
            return;
        }

        if (startedInt <= 3) {
            if (data.customization) {
                if (data.customization == "timestamp") {
                    checkbox.checked = true
                } else if (data.customization == "proccessed_timestamp") {
                    checkbox.checked = false
                }
            }
        }
        protocol.innerText = data.protocol;
        srcAddr.innerText = data.srcAddr;
        dstnAddr.innerText = data.dstnAddr;
        packetNumber.innerText = data.packetNumber;
        timeCell.innerText = data.time;

        row.appendChild(interface);
        row.appendChild(protocol);
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
            fetch(`http://localhost:8080/packetinfo?packetnumber=${packetNumberSelected}`)
                .catch(error => {
                    statusMessage.innerText = "Couldn't retrieve information";
                    console.error(error); // Log any errors to the console
                    return
                })
                .then(response => response.json())
                .then(data => {
                    //data.packetDump); // Log the packet information to the console
                    //showDump(data.packetDump, data.packetNumber)
                    showPopupBox(data.packetNumber, data.packetDump)

                })
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

// function showDump(data, number) {
//     const popupBox = document.getElementById('popupBox');
//     const packetDumpOutput = document.getElementById('packetDumpOutput');
//     const packetNumber = document.getElementById("packetNumber")
//     const closeButton = document.getElementById('closeButton');

//     function showPopupBox() {
//         packetDumpOutput.innerText = data;
//         packetNumber.innerText = number
//         popupBox.style.display = 'block';
//     }

//     function closePopupBox() {
//         popupBox.style.display = 'none';
//     }

//     showPopupBox()

//     // Close the popup box when the close button is clicked
//     closeButton.addEventListener('click', closePopupBox);
// }

// // Get the popup and the handle
// const popup = document.querySelector('#popupBox');
// const handle = popup.querySelector('.popup-handle');

// // Define variables to store the initial position of the handle and the mouse
// let startY = 0;
// let initialHeight = 0;

// // Add event listeners for the mouse events
// handle.addEventListener('mousedown', startDrag);
// window.addEventListener('mouseup', endDrag);

// // Function to start the drag event
// function startDrag(e) {
//     startY = e.clientY;
//     initialHeight = popup.offsetHeight;
//     window.addEventListener('mousemove', drag);
// }

// // Function to end the drag event
// function endDrag() {
//     window.removeEventListener('mousemove', drag);
// }

// // Function to handle the drag event
// function drag(e) {
//     const delta = e.clientY - startY;
//     const newHeight = initialHeight - delta;
//     popup.style.height = `${newHeight}px`;
// }


// Event listener example

function showPopupBox(number, data) {
    const popupContainer = document.getElementById('popupContainer');
    const popupBox = document.getElementById('popupBox');
    const dragHandle = document.getElementById('dragHandle');
    const packetDumpOutput = document.getElementById('packetDumpOutput');
    const packetNumber = document.getElementById("packetNumber")
    var startY, startHeight;

    // Show the popup box
    popupContainer.style.height = 'auto';
    popupBox.style.height = 'auto'; // Initial height
    popupBox.style.visibility = 'visible'

    // Set the packet number and data
    packetNumber.innerText = number;
    packetDumpOutput.innerText = data;
    // popupContent.innerHTML = '';
    // popupContent.appendChild(packetNumber);
    // popupContent.appendChild(packetDumpOutput);


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
}
