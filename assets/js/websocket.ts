
let process = true;
let msgQueue = new Array<string>();

async function connect():Promise<WebSocket> {

    return new Promise( (resolve, reject) => {
        let socket:WebSocket = new WebSocket("ws://localhost:3000/api/v1/websocket");
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

async function wait(timeInMs:number):Promise<any> {
    return new Promise(resolve => setTimeout(resolve, timeInMs));
}

async function handleMsgReceived(socket:WebSocket, data:string) {
    wait(500).then( () => {
        console.log(`Received: ${data}`)
        let payload = Date.now().toString();
        socket.send(payload);
        console.log(`Sent: ${payload}`)
    });
}

let server:WebSocket = null;
connect().then( (socket) => {
    return new Promise<WebSocket>( resolve => {
        setTimeout(() => socket.send(Date.now().toLocaleString()), 10000);
        resolve(socket);
        }
    );
}).then((socket) => server = socket);


async function Process() {
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




