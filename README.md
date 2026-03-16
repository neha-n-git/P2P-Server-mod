# P2P File Sharing System

A BitTorrent-inspired peer-to-peer file sharing simulation built in Go. Multiple instances of the application act as independent peers, each capable of sending and receiving files by dividing them into chunks and distributing them across the network.

---

## Overview

In this P2P simulation, there is no central server. Each peer acts as both a client and a server, exchanging file chunks directly with other peers. The system demonstrates core distributed file sharing concepts including chunk-based transfer, peer discovery, and concurrent downloads.

---

## Features

- File upload via web interface, processed as a byte stream and split into fixed-size chunks
- Chunk integrity verification using hashes
- Peer discovery and registration across the network
- Concurrent chunk downloads from multiple peers in parallel
- File reconstruction from received chunks in correct order
- Basic load balancing across available peers
- JSON-based peer communication for interoperability
- Graceful error handling for network interruptions and incomplete transfers

---

## Architecture

```
p2p-server/
├── main.go               # Entry point, HTTP server setup
├── peer/
│   ├── peer.go           # Peer struct, registration, identity
│   └── peer_test.go      # Unit tests for peer logic
├── file/
│   ├── file.go           # File splitting, chunk generation, reconstruction
│   └── file_test.go      # Unit tests for file operations
├── chunk/
│   ├── chunk.go          # Chunk struct, hash verification
│   └── chunk_test.go     # Unit tests for chunk integrity
├── transfer/
│   └── transfer.go       # Peer communication interface, chunk transfer logic
├── static/
│   └── index.html        # Frontend upload form
└── README.md
```
![P2P Sample](p2p.png)

---

## Key Concepts Implemented

| Concept | Usage |
|---|---|
| Variables & data types | Peer identity, port, file size, chunk count, chunk size |
| Arrays | Fixed-size chunk hash storage |
| Slices | Dynamic storage of file chunks and connected peers |
| Structs | `Peer`, `File`, and `Chunk` types representing real-world components |
| Maps | File-to-peer mapping and chunk availability records |
| Functions | File upload, chunk generation, peer communication, file reconstruction |
| Interfaces | Modular peer communication abstraction |
| Pointers | Reference-based state updates for peer info and transfer progress |
| Control structures | Decision flows for peer existence checks, success/failure handling |
| Loops | Iterating over file data to generate chunks and over peers to distribute them |
| Error handling | Network interruptions, incomplete transfers, missing peers |
| JSON marshalling | Serialization of peer registration, file metadata, chunk info |
| Goroutines & channels | Concurrent chunk downloads from multiple peers in parallel |
| Unit testing | Validation of file splitting, chunk integrity, and metadata processing |

---

## Getting Started

### Prerequisites

- Go 1.21 or higher
- A modern web browser

### Running a peer instance

```bash
# Clone the repository
git clone https://github.com/your-username/p2p-server.git
cd p2p-server

# Run the first peer on port 8080
go run main.go --port 8080

# Run a second peer on port 8081 (in a new terminal)
go run main.go --port 8081

# Run a third peer on port 8082 (in a new terminal)
go run main.go --port 8082
```

Open `http://localhost:8080` in your browser to access the upload interface.

### Running tests

```bash
go test ./...
```

---

## How It Works

1. **Upload** — A user selects a file via the web form. The backend reads it as a byte stream and splits it into fixed-size chunks. Each chunk is hashed for integrity verification.

2. **Peer registration** — Each peer registers itself with other known peers, sharing its address, port, and the list of files it holds.

3. **Distribution** — The system iterates over available peers and distributes chunks across the network.

4. **Download** — When a peer requests a file, it queries the network for chunk availability. Multiple chunks are downloaded concurrently from different peers using goroutines, with a channel collecting results.

5. **Reconstruction** — Once all chunks are received, they are reassembled in the correct order to reconstruct the original file.

   

---

## API Endpoints

| Method | Endpoint | Description |
|---|---|---|
| `POST` | `/upload` | Upload a file and split into chunks |
| `GET` | `/peers` | List all registered peers |
| `POST` | `/register` | Register a new peer |
| `GET` | `/chunks/:fileID` | Get chunk availability for a file |
| `GET` | `/download/:fileID` | Download and reconstruct a file |

---

## Notes

- This is a simulation and does not implement the full BitTorrent protocol (no tracker, no `.torrent` files, no DHT).
- Peers must be running on the same machine or local network for discovery to work without additional configuration.
- Chunk size and peer list are configurable via environment variables or flags.
