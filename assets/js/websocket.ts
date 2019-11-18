import { defaultNameResolver } from "webpack/lib/NamedChunksPlugin";

let socket:WebSocket = new WebSocket("http://localhost:3000/websocket");

let ready:boolean = false;

socket.onopen = (ev: Event):any => {
    console.log("Connection opened");
    ready = true;
};

async function checkReadiness():Promise<boolean> {
    return new Promise(resolve => {
        return setTimeout(() => resolve(ready), 500);
    });
}

function processWebsocket(socket:WebSocket) {

    await checkReadiness()
}


processWebsocket(socket);

