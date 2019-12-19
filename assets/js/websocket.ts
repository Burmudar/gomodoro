
let process = true;
let msgQueue = new Array<string>();

let timerStore = new Map<string, object>();

async function connect():Promise<WebSocket> {

    return new Promise( (resolve, reject) => {
        let socket:WebSocket = new WebSocket("ws://localhost:3000/api/v1/websocket");
        socket.onopen = (ev: Event):any => {
            console.log("Connection opened");
            socket.send(JSON.stringify({
                type: "register",
            }));
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

async function handleMsgReceived(socket:WebSocket, data:string) {
    console.log(`Received: ${data}`)

    let msg = JSON.parse(data);
    let payload:object = null;

    switch (msg.type) {
        case "registration_id":
            payload = { type: "new_timer", interval: 5, focus: 10, timestamp: Date.now()};
            break;
        case "timer_created":
            payload = { type: "start_timer", timerId: msg.timerId, timestamp: Date.now()};
            break;
        case "timer_interval_event":
            console.log(`Timer[${msg.timerId}] Interval Event. Elapsed: [${msg.Elapsed}]`);
            break;
        case "timer_done_event":
            console.log(`Timer[${msg.timerId}] Complete Event. Elapsed: [${msg.Elapsed}]`);
            break;
        default:
            console.log(msg);

    }
    sendMsg(socket, payload);
}

let server:WebSocket = null;

async function Process() {
    server = await connect();

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




