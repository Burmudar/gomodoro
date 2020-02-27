
let process = true;
let msgQueue = new Array<string>();
let CLIENT_ID = "";

let timerStore = new Map<string, object>();

async function connect():Promise<WebSocket> {

    return new Promise( (resolve, reject) => {
        let socket:WebSocket = new WebSocket(`ws://${location.hostname}:3000/api/v1/websocket`);
        socket.onopen = (ev: Event):any => {
            console.log("Connection opened");
            socket.onmessage = (ev: MessageEvent) => {
                msgQueue.push(ev.data);
            };
            resolve(socket);
        };

        socket.onerror = (ev: Event):any => {
            console.log(`Connection error! ${ev}`);
            process = false;
            reject(ev);
        }
    });

}

async function registerWithServer(socket:WebSocket) {
    let payload = { type: "register", timestamp: Date.now().valueOf()};
    return sendMsg(socket, payload);
}

async function identifyWithServer(socket:WebSocket, clientId:string) {
    let payload = { type: "identify", timestamp: Date.now().valueOf(), clientId: clientId };
    return sendMsg(socket, payload);
}

async function wait(timeInMs:number):Promise<any> {
    return new Promise(resolve => setTimeout(resolve, timeInMs));
}

async function sendMsg(socket:WebSocket, payload:object) {
    if (!payload) {
        return
    }
    let data = JSON.stringify(payload);
    socket.send(data);
    console.log("Sent", data);
}

function setClientId(value:string) {
    CLIENT_ID = value;
    localStorage.setItem("clientId", value);
}

function createPayloadFromConfigs(configs:any) {
    let rIdx = Math.floor(Math.random() * configs.length) % configs.length;

    let payload = { type: "new_timer", configId: configs[rIdx].id, timestamp: Date.now().valueOf()};
    return payload

}

async function handleMsgReceived(socket:WebSocket, data:string) {
    console.log(`Received: ${data}`)

    let msg = JSON.parse(data);
    let payload:object = null;

    switch (msg.type) {
        case "identified":
            console.log(`Server recognized ${CLIENT_ID}`)
            payload = { type: "get_timer_configs", timestamp: Date.now().valueOf()};
            break;
        case "unknown_client_error":
            console.log(`Server did not recognize ${CLIENT_ID}. Attempting to re-register`)
            setClientId("");
            payload = null;
            return await registerWithServer(socket);
        case "registration_id":
            setClientId(msg.Key);
            payload = { type: "new_timer", interval: 5, focus: 10, timestamp: Date.now().valueOf()};
            break;
        case "timer_created":
            payload = { type: "start_timer", timerId: msg.timerId, timestamp: Date.now().valueOf()};
            break;
        case "timer_interval_event":
            console.log(`Timer[${msg.timerId}] Interval Event. Elapsed: [${msg.elapsed}]`);
            break;
        case "timer_done_event":
            console.log(`Timer[${msg.timerId}] Complete Event. Elapsed: [${msg.elapsed}]`);
            break;
        case "timer_configs_result":
            console.log(`Result: ${console.table(msg.Result)}`)
            payload = createPayloadFromConfigs(msg.Result);
            break;
        default:
            console.log(`Unkown Msg: ${msg.type}`);
            payload = { type: "new_timer", interval: 5, focus: 10, timestamp: Date.now().valueOf()};
            console.log(msg);

    }

    if (payload) {
        sendMsg(socket, payload);
    }
}

let server:WebSocket = null;

async function Process() {
    let serverPromise = connect();

    CLIENT_ID = localStorage.getItem("clientId");

    server = await serverPromise

    if (!CLIENT_ID) {
        await registerWithServer(server);
    } else {
        await identifyWithServer(server, CLIENT_ID);
    }

    while (process) {
        if (msgQueue.length == 0) {
            await wait(1000);
        }
        let promises:Array<Promise<any>> = new Array<Promise<any>>();
        for (let i = 0; i < msgQueue.length; i++) {
            promises.push(handleMsgReceived(server, msgQueue.pop()));
        }

        console.log(`Waiting on ${promises.length}`);
        await Promise.all(promises);
    }
}

Process().then(() => console.log("Processing complete!")).catch(() => console.log("Error during processing!"));




