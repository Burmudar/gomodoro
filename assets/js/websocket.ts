
let ready:boolean = false;

async function connect():Promise<WebSocket> {

    return new Promise( (resolve, reject) => {
        let socket:WebSocket = new WebSocket("ws://localhost:3000/api/v1/websocket");
        socket.onopen = (ev: Event):any => {
            console.log("Connection opened");
            resolve(socket);
        };

        socket.onerror = (ev: Event):any => {
            console.log(`Connection error! ${ev}`);
            reject(ev);
        }
    });

}



async function checkReadiness():Promise<boolean> {
    return new Promise( resolve => setTimeout(() => resolve(ready), 1));

}

async function processWebsocket(socket:WebSocket) {
    while (!await checkReadiness()) { console.log("Not ready");}

    console.log("Websocket is ready");

}

async function sendMsg(msg:string) {
}

async function msgReceived(msg:string) {
}

let server = Promise.resolve(connect());

async function handleMsgReceived(server:WebSocket):Promise<void> {
    new Promise((resolve, reject) => {

    });
}

