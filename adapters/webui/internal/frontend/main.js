document.addEventListener("DOMContentLoaded", function() {
    collectAndUpdateTable();
    pollForUpdates();
});

const updatePath = '{{.Paths.Update}}'
const objectDataPath = '{{.Paths.ObjectData}}'
const listPath = '{{.Paths.List}}'

async function collectAndUpdateTable() {
    const data = await collectRecords();
    const tableBody = document.getElementById('tableBody');
    tableBody.innerHTML = ''; // Clear previous data

    if (!data) {
        return;
    }

    if (!data.items) {
        return;
    }

    data.items.forEach(record => {
        const green = 'text-center bg-green-50 dark:bg-green-800 text-green-600 dark:text-green-300 text-sm py-4 px-4 rounded-full';
        const red = 'text-center bg-red-50 dark:bg-red-800 text-red-600 dark:text-red-300 text-sm py-4 px-4 rounded-full';
        const yellow = 'text-center bg-yellow-50 dark:bg-yellow-800 text-yellow-600 dark:text-yellow-300 text-sm py-4 px-4 rounded-full';
        const gray = 'text-center bg-gray-50 dark:bg-gray-800 text-gray-600 dark:text-gray-300 text-sm py-4 px-4 rounded-full';

        const createdAt = new Date(record.created_at).toISOString().replace('T', ' ').replace('Z', '');
        const updatedAt = new Date(record.updated_at).toISOString().replace('T', ' ').replace('Z', '');

        const resumeButton = `<button
                    onclick="recordAction('resume', ${record.id})"
                    class="bg-blue-50 dark:bg-blue-800 text-blue-600 dark:text-blue-300 text-sm py-2 px-4 m-1 rounded-full hover:bg-blue-50 dark:hover:bg-blue-700">
                    Resume
                </button>`

        const pauseButton = `<button
                    onclick="recordAction('pause', ${record.id})"
                    class="bg-yellow-50 dark:bg-yellow-800 text-yellow-600 dark:text-yellow-300 text-sm py-2 px-4 m-1 rounded-full hover:bg-yellow-50 dark:hover:bg-yellow-700">
                    Pause
                </button>`

        const cancelButton = `<button
                    onclick="recordAction('cancel', ${record.id})"
                    class="bg-red-50 dark:bg-red-800 text-red-600 dark:text-red-300 text-sm py-2 px-4 m-1 rounded-full hover:bg-red-50 dark:hover:bg-red-700">
                    Cancel
                </button>`

        const deleteButton = `<button
                    onclick="recordAction('delete', ${record.id})"
                    class="bg-gray-50 dark:bg-gray-800 text-gray-600 dark:text-gray-300 text-sm py-2 px-4 m-1 rounded-full hover:bg-gray-50 dark:hover:bg-gray-700">
                    Delete
                </button>`

        let selectedClass = ''
        let buttons = ''
        switch (record.run_state) {
            case 'Initiated':
                selectedClass = gray
                buttons = pauseButton
                break
            case 'Running':
                selectedClass = green
                buttons = pauseButton + cancelButton
                break
            case 'Paused':
                selectedClass = yellow
                buttons = resumeButton + cancelButton
                break
            case 'Cancelled':
                selectedClass = red
                buttons = deleteButton
                break
            case 'Completed':
                selectedClass = green
                buttons = deleteButton
                break
            case 'Data Deleted':
                selectedClass = gray
                buttons = deleteButton
                break
            case 'Requested Data Deleted':
                selectedClass = yellow
                break
        }


        const row = document.createElement('tr');
        row.classList.add('hover:bg-gray-50', 'dark:hover:bg-gray-800', 'transition', 'duration-150', 'ease-in-out');
        row.innerHTML = `
            <td class="py-4 px-4 text-center text-sm text-gray-800 dark:text-gray-300">${record.id}</td>
            <td class="py-4 px-4 text-center text-sm text-gray-800 dark:text-gray-300">${record.foreign_id}</td>
            <td class="py-4 px-4 text-center text-sm text-gray-800 dark:text-gray-300">${record.run_id}</td>
            <td>
                <div class="${selectedClass}">${record.run_state}</div>
            </td>
            <td class="py-4 px-4 text-center text-sm text-gray-800 dark:text-gray-300">${record.status}</td>
            <td class="py-4 px-4 text-center text-sm text-gray-600 dark:text-gray-400">${createdAt}</td>
            <td class="py-4 px-4 text-center text-sm text-gray-600 dark:text-gray-400">${updatedAt}</td>
            <td class="py-4 px-4 text-center text-sm text-gray-800 dark:text-gray-300">
                <button
                    class="bg-green-50 dark:bg-green-800 text-green-600 dark:text-green-300 text-sm py-2 px-4 rounded-full hover:bg-green-50 dark:hover:bg-green-700"
                    onclick="getObjectData(${record.id})"
                >View</button>
            </td>
            <td class="py-4 px-4 text-center">
                ${buttons}
            </td>
        `;
        tableBody.appendChild(row);
    });
}

async function getObjectData(id) {
    const requestData = {
        record_id: id,
    };


    const response = await fetch(objectDataPath, {
        method: 'POST',
        headers: {
            'Content-Type': 'application/json',
        },
        body: JSON.stringify(requestData),
    });

    const data = await response.json();
    openModal(data)
}

function createCollapsibleJSON(obj, container) {
    try {
        for (const key in obj) {
            const value = obj[key];
            const item = document.createElement("div");
            item.classList.add("ml-4");

            // Key element with expanded indicator
            const keyEl = document.createElement("span");
            keyEl.classList.add("font-semibold", "text-gray-700", "cursor-pointer");
            keyEl.textContent = key + ": ";
            item.appendChild(keyEl);

            // Check if value is an object or array for collapsible feature
            if (typeof value === "object" && value !== null) {
                const nestedContainer = document.createElement("div");
                nestedContainer.classList.add("ml-4"); // Visible by default

                keyEl.textContent += "▼"

                // Add toggle functionality
                keyEl.onclick = () => {
                    nestedContainer.classList.toggle("hidden");
                    keyEl.textContent = keyEl.textContent.includes("►") ? key + ": ▼" : key + ": ►";
                };

                // Recursive call for nested objects
                createCollapsibleJSON(value, nestedContainer);
                item.appendChild(nestedContainer);
            } else {
                // Display primitive values
                const valueEl = document.createElement("span");
                valueEl.classList.add("text-gray-600");
                valueEl.textContent = JSON.stringify(value);
                item.appendChild(valueEl);
            }
            container.appendChild(item);
        }
    } catch (e) {
        console.error("request failed:", e);
    }

}

function openModal(data) {
    document.getElementById('jsonModal').classList.remove('hidden');

    const jsonContainer = document.getElementById('jsonContainer');
    jsonContainer.innerHTML = "";  // Clear previous content
    createCollapsibleJSON(data, jsonContainer);
}

function closeModal() {
    document.getElementById('jsonModal').classList.add('hidden');
}

async function recordAction(action, id) {
    switch (action) {
        case 'resume':
            break
        case 'pause':
            break
        case 'cancel':
            break
        case 'delete':
            break
        default:
            window.alert('Unknown action')
            return
    }

    const data = {
        record_id: id,
        action: action,
    }

    try {
        await fetch(updatePath, {
            method: 'POST',
            headers: {
                'Content-Type': 'application/json',
            },
            body: JSON.stringify(data),
        });
        await collectAndUpdateTable();
    }catch (e) {
        console.error("request failed:", e);
    }
}

async function collectRecords() {
    try {
        const workflowName = document.getElementById('workflowName').value;
        const foreignID = document.getElementById('foreignID').value;
        const runState = document.getElementById('runState').value;
        const status = Number(document.getElementById('status').value);
        const offset = Number(document.getElementById('offset').value);
        const limit = Number(document.getElementById('limit').value);
        const order = document.getElementById('order').value;

        const requestData = {
            workflow_name: workflowName,
            offset_id: offset,
            limit: limit,
            order: order,
            filter_by_foreign_id: foreignID,
            filter_by_run_state: runState ? parseInt(runState) : null,
            filter_by_status: status ? parseInt(status) : null,
        };

        const response = await fetch(listPath, {
            method: 'POST',
            headers: {
                'Content-Type': 'application/json',
            },
            body: JSON.stringify(requestData),
        });

        return await response.json();
    } catch (e) {
        console.error(e)
    }
}

// Variable to hold the polling timeout ID and control the polling state
let pollingActive = false;
let pollingTimeoutId = null;

async function pollForUpdates() {
    try {
        await collectAndUpdateTable();
    } catch (e) {
        console.error(e);
    } finally {
        if (pollingActive) {
            scheduleNextPoll();
        }
    }
}

// Schedule the next poll with setTimeout to avoid blocking the event loop
function scheduleNextPoll() {
    const pollingInterval = 5000; // 5 seconds
    pollingTimeoutId = setTimeout(pollForUpdates, pollingInterval);
}

// Toggle function for starting and stopping the polling
function togglePolling() {
    const pollButton = document.getElementById("pollButton");
    if (pollingActive) {
        // Stop polling
        clearTimeout(pollingTimeoutId);
        pollingActive = false;
        pollButton.textContent = "Enable Polling";
        pollButton.classList.replace("bg-red-500", "bg-blue-500");
        pollButton.classList.replace("hover:bg-red-700", "hover:bg-blue-700");
    } else {
        // Start polling
        pollingActive = true;
        pollButton.textContent = "Disable Polling";
        pollButton.classList.replace("bg-blue-500", "bg-red-500");
        pollButton.classList.replace("hover:bg-blue-700", "hover:bg-red-700");
        pollForUpdates(); // Start the first poll immediately
    }
}