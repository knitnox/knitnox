const fetch = require('node-fetch') || globalThis.fetch;

async function test() {
    console.log("Sending POST to initialize...");
    const initPayload = {
        jsonrpc: '2.0',
        id: 1,
        method: 'initialize',
        params: {
            protocolVersion: '2024-11-05',
            capabilities: {},
            clientInfo: { name: 'KnitnoxClient', version: '1.0.0' }
        }
    };

    try {
        const res = await fetch('http://localhost:8000/mcp', {
            method: 'POST',
            headers: {
                'Content-Type': 'application/json',
                'Accept': 'application/json, text/event-stream'
            },
            body: JSON.stringify(initPayload)
        });
        
        console.log("Status:", res.status);
        console.log("Headers:", Object.fromEntries(res.headers.entries()));
        console.log("Body:", await res.text());
    } catch (e) {
        console.error("Error:", e);
    }
}
test();