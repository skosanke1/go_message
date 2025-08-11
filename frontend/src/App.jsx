import React, { useState, useRef, useEffect } from "react";

/*
Simple WebSocket UI:
- Connect to server (enter ws URL or default to ws://<host>:8080/ws)
- Send JSON messages {type, sender, payload}
- Show logs (incoming/outgoing)
*/

export default function App() {
  const [serverAddr, setServerAddr] = useState("");
  const [connected, setConnected] = useState(false);
  const [input, setInput] = useState("");
  const [logs, setLogs] = useState([]);
  const wsRef = useRef(null);
  const clientId = useRef("user-" + Math.floor(Math.random() * 10000));

  // default server if none provided
  useEffect(() => {
    if (!serverAddr) {
      const host = window.location.hostname || "localhost";
      setServerAddr(`ws://${host}:8081/ws`);
    }
  }, []);

  function addLog(entry) {
    setLogs((l) => [...l, { ...entry, ts: new Date().toLocaleTimeString() }]);
  }

  function connect() {
    if (wsRef.current) return;
    try {
      const ws = new WebSocket(serverAddr);
      ws.onopen = () => {
        wsRef.current = ws;
        setConnected(true);
        addLog({ sender: "system", payload: "connected" });
      };
      ws.onmessage = (ev) => {
        try {
          const msg = JSON.parse(ev.data);
          addLog({ sender: msg.sender || "server", payload: msg.payload, type: msg.type });
        } catch (e) {
          addLog({ sender: "server", payload: ev.data });
        }
      };
      ws.onclose = () => {
        wsRef.current = null;
        setConnected(false);
        addLog({ sender: "system", payload: "disconnected" });
      };
      ws.onerror = (e) => {
        console.error(e);
        addLog({ sender: "system", payload: "error" });
      };
    } catch (err) {
      addLog({ sender: "system", payload: "failed to connect: " + err.message });
    }
  }

  function disconnect() {
    if (wsRef.current) {
      wsRef.current.close();
      wsRef.current = null;
    }
    setConnected(false);
  }

  function send() {
    if (!wsRef.current || wsRef.current.readyState !== WebSocket.OPEN) {
      addLog({ sender: "system", payload: "not connected" });
      return;
    }
    const msg = { type: "message", sender: clientId.current, payload: input };
    wsRef.current.send(JSON.stringify(msg));
    addLog({ sender: "me", payload: input });
    setInput("");
  }

  return (
    <div style={{ maxWidth: 800, margin: "24px auto", fontFamily: "sans-serif" }}>
      <h1>Go WebSocket Game — Demo</h1>
      <div style={{ marginBottom: 12 }}>
        <input
          style={{ width: "70%", padding: 8 }}
          value={serverAddr}
          onChange={(e) => setServerAddr(e.target.value)}
        />
        {!connected ? (
          <button onClick={connect} style={{ marginLeft: 8, padding: "8px 12px" }}>
            Connect
          </button>
        ) : (
          <button onClick={disconnect} style={{ marginLeft: 8, padding: "8px 12px" }}>
            Disconnect
          </button>
        )}
        <span style={{ marginLeft: 12, color: connected ? "green" : "gray" }}>
          {connected ? "connected" : "disconnected"}
        </span>
      </div>

      <div style={{ display: "flex", gap: 8, marginBottom: 12 }}>
        <input
          style={{ flex: 1, padding: 8 }}
          placeholder="Type a message..."
          value={input}
          onChange={(e) => setInput(e.target.value)}
          onKeyDown={(e) => e.key === "Enter" && send()}
        />
        <button onClick={send} style={{ padding: "8px 12px" }}>
          Send
        </button>
      </div>

      <div style={{ border: "1px solid #ddd", padding: 12, height: 360, overflow: "auto", background: "#fafafa" }}>
        <ul style={{ listStyle: "none", padding: 0, margin: 0 }}>
          {logs.map((l, i) => (
            <li key={i} style={{ padding: "6px 0", borderBottom: "1px dashed #eee" }}>
              <strong>[{l.ts}] {l.sender}:</strong> {l.payload}
              {l.type ? <span style={{ marginLeft: 8, color: "#888" }}>({l.type})</span> : null}
            </li>
          ))}
        </ul>
      </div>
    </div>
  );
}
