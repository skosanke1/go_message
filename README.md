Run locally on Ubuntu VM — step-by-step
Below are two flows: development (fast iteration) and production (serve React static files from Go binary).

Prereqs (on the Ubuntu VM)

bash
Copy
Edit
# update & install Go + Node
sudo apt update
sudo apt install -y golang-go nodejs npm git

# (Optional) If apt's node is old, install Node 18/20 from NodeSource for Vite:
# curl -fsSL https://deb.nodesource.com/setup_20.x | sudo -E bash -
# sudo apt-get install -y nodejs
Create project (example)

bash
Copy
Edit
# on VM
mkdir -p ~/projects/gowebsocketgame
cd ~/projects/gowebsocketgame

# create backend and frontend folders and paste the files above into them
# or git clone if you put the files into a repo
Dev mode (run backend + Vite dev server)
Backend

bash
Copy
Edit
cd backend
go mod init github.com/yourname/gowebsocketgame
go get github.com/gorilla/websocket
go run main.go -addr=":8080" -mode=echo
# server will listen on :8080 and serve static at ../frontend/dist (if exists)
Frontend

bash
Copy
Edit
cd ../frontend
npm install
npm run dev      # Vite dev server (usually on port 5173)
Open browser (on host or VM):

If you're running browser inside VM: open http://localhost:5173

If from host machine to VM over LAN: use VM IP or set VirtualBox port forwarding (see below). In the UI set the WebSocket server address to ws://<VM_IP>:8080/ws.

Production mode (build frontend and serve from Go)
Build frontend

bash
Copy
Edit
cd frontend
npm install
npm run build   # Vite outputs to ./dist by default
Start Go server pointing static to built files (from backend dir)

bash
Copy
Edit
cd ../backend
# ensure -static points to the dist folder created
go build -o gowebsocket
./gowebsocket -addr=":8080" -static="../frontend/dist" -mode=broadcast
Visit in browser: http://<VM_IP>:8080 (serves the built React app). WebSocket endpoint is ws://<VM_IP>:8080/ws.

Make it accessible from your host (VirtualBox tips)
If your Ubuntu VM is in NAT mode, configure VirtualBox port forwarding:

VirtualBox Manager → select VM → Settings → Network → Adapter → Advanced → Port Forwarding.

Add a rule: Host Port 8080 → Guest Port 8080 (TCP). Now from host open http://localhost:8080.

Alternatively, set the VM network to Bridged Adapter so the VM gets an IP on your LAN, then use that IP from host: http://192.168.x.y:8080.

Firewall
If ufw is enabled on the VM:

bash
Copy
Edit
sudo ufw allow 8080/tcp
How to test (quick)
Start backend in broadcast mode: ./gowebsocket -addr=":8080" -mode=broadcast -static="../frontend/dist"

Open two browser windows to http://<VM_IP>:8080 (or host forwarded port).

Click Connect in both, send a message from one — everyone should see it (broadcast). In echo mode only the sender sees an echo.

How to extend / next steps
New game: implement Game interface (OnConnect, OnMessage, OnDisconnect) and swap mode= or inject new game.

Authentication / usernames: authenticate via HTTP POST before upgrading to WebSocket; store user info in Client.

Room support: add room field in client and hub, maintain map[string]map[*Client]bool to broadcast per room.

Better writePump: switch to NextWriter to batch messages if needed (current implementation is simple and robust).

Production security: tighten CheckOrigin, use TLS (serve via reverse proxy like Caddy/Nginx with certs).

Deployment: Use systemd unit to run the Go binary; use Nginx as reverse proxy and TLS.

Quick notes & caveats
WebSocket connections are not subject to the same CORS policy as XHR/fetch, but the HTTP Upgrade still has Origin header — keep CheckOrigin strict in production.

In dev we allow all origins for convenience (CheckOrigin: true). Don’t do that in production.

For production TLS, you can front the Go server with Nginx/Caddy or implement TLS in Go directly.

