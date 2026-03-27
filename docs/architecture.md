# P2P File Sharing System — Architecture & Design

## 1. System Overview

The P2P File Sharing System is a distributed application where each peer acts as both a client and server. Files are split into chunks, distributed across peers, and reassembled on download. Authentication is enforced at two levels: user login via web UI and network-level password for peer joining.

```mermaid
graph TB
    subgraph "Peer A :8080"
        WA[Web Browser] -->|HTTP| SA[HTTP Server]
        SA --> AUTH_A[Auth Middleware]
        AUTH_A --> HA[Handlers]
        HA --> PA[Peer State]
        PA --> FS_A[File Chunks]
    end

    subgraph "Peer B :8081"
        WB[Web Browser] -->|HTTP| SB[HTTP Server]
        SB --> AUTH_B[Auth Middleware]
        AUTH_B --> HB[Handlers]
        HB --> PB[Peer State]
        PB --> FS_B[File Chunks]
    end

    subgraph "Peer C :8082"
        WC[Web Browser] -->|HTTP| SC[HTTP Server]
        SC --> AUTH_C[Auth Middleware]
        AUTH_C --> HC[Handlers]
        HC --> PC[Peer State]
        PC --> FS_C[File Chunks]
    end

    SA <-->|P2P API + Network Password| SB
    SB <-->|P2P API + Network Password| SC
    SA <-->|P2P API + Network Password| SC

    style WA fill:#E4568B,color:white
    style WB fill:#E4568B,color:white
    style WC fill:#E4568B,color:white
    style AUTH_A fill:#F6C94D,color:#2A2A2A
    style AUTH_B fill:#F6C94D,color:#2A2A2A
    style AUTH_C fill:#F6C94D,color:#2A2A2A
```

---

## 2. Package Architecture

```mermaid
graph LR
    MAIN[main.go] --> AUTH[auth/]
    MAIN --> HANDLER[handler/]
    MAIN --> PEER[peer/]
    MAIN --> TRANSPORT[transport/]
    MAIN --> REGISTRY[registry/]

    AUTH --> |bcrypt| CRYPTO[golang.org/x/crypto]

    HANDLER --> AUTH
    HANDLER --> PEER
    HANDLER --> REGISTRY
    HANDLER --> TRANSPORT
    HANDLER --> FILE[file/]

    PEER --> FILE
    PEER --> REGISTRY
    PEER --> TRANSPORT

    TRANSPORT --> FILE

    subgraph "auth/"
        A1[auth.go — UserStore + bcrypt]
        A2[session.go — SessionManager]
        A3[middleware.go — RequireAuth]
        A4[network.go — NetworkAuth]
    end

    subgraph "handler/"
        H1[auth_handler.go — Login/Register/Logout]
        H2[api.go — Peer Registration + File API]
        H3[upload.go — File Upload]
        H4[distribute.go — Browse + Download]
    end

    subgraph "peer/"
        P1[peer.go — Peer struct + state]
        P2[downloader.go — Concurrent downloads]
    end

    subgraph "file/"
        F1[chunking.go — Split/Merge]
        F2[filemeta.go — FileMeta/ChunkMeta]
    end

    subgraph "transport/"
        T1[transport.go — Interface]
        T2[http_transport.go — HTTP impl]
    end

    subgraph "registry/"
        R1[registry.go — Peer list]
    end

    style AUTH fill:#E4568B,color:white
    style HANDLER fill:#5D7B3D,color:white
    style PEER fill:#A7C7E4,color:#2A2A2A
    style FILE fill:#F6C94D,color:#2A2A2A
    style TRANSPORT fill:#F29BB9,color:#2A2A2A
    style REGISTRY fill:#5D7B3D,color:white
```

---

## 3. Authentication Flow

```mermaid
sequenceDiagram
    participant B as Browser
    participant S as HTTP Server
    participant MW as Auth Middleware
    participant AH as Auth Handler
    participant US as UserStore
    participant SM as SessionManager
    participant BC as bcrypt

    Note over B,BC: User Registration
    B->>S: POST /api/auth/register {username, password, network_pw}
    S->>AH: Route to AuthRegisterHandler
    AH->>AH: Validate network password (bcrypt)
    AH->>BC: bcrypt.GenerateFromPassword(password)
    BC-->>AH: password_hash
    AH->>US: RegisterUser(username, hash)
    US-->>AH: OK
    AH->>SM: CreateSession(username)
    SM-->>AH: session_token
    AH-->>B: Set-Cookie: session_token + redirect

    Note over B,BC: User Login
    B->>S: POST /api/auth/login {username, password}
    S->>AH: Route to AuthLoginHandler
    AH->>US: GetUser(username)
    US-->>AH: User{password_hash}
    AH->>BC: bcrypt.CompareHashAndPassword(hash, password)
    BC-->>AH: match ✓
    AH->>SM: CreateSession(username)
    SM-->>AH: session_token
    AH-->>B: Set-Cookie: session_token + redirect

    Note over B,BC: Accessing Protected Route
    B->>S: GET /upload (Cookie: session_token)
    S->>MW: RequireAuth middleware
    MW->>SM: ValidateSession(token)
    SM-->>MW: Session{username, expiry} ✓
    MW->>S: Pass through to handler
    S-->>B: Serve upload.html
```

---

## 4. Peer Registration Flow (Network Password)

```mermaid
sequenceDiagram
    participant PA as Peer A (new)
    participant PB as Peer B (existing)
    participant NA as NetworkAuth

    Note over PA,NA: Peer A starts with --network-password "secret"
    PA->>PB: POST /api/register {address: "localhost:8081", network_password: "secret"}
    PB->>NA: ValidateNetworkPassword("secret")
    NA->>NA: bcrypt.CompareHashAndPassword(stored_hash, "secret")
    NA-->>PB: Valid ✓
    PB->>PB: registry.AddPeer("localhost:8081")
    PB-->>PA: {"status": "registered"}

    Note over PA,NA: Peer with WRONG password
    PA->>PB: POST /api/register {address: "localhost:8082", network_password: "wrong"}
    PB->>NA: ValidateNetworkPassword("wrong")
    NA->>NA: bcrypt.CompareHashAndPassword(stored_hash, "wrong")
    NA-->>PB: Invalid ✗
    PB-->>PA: 403 Forbidden
```

---

## 5. Concurrent File Download Flow

```mermaid
sequenceDiagram
    participant UI as Browser
    participant DH as DownloadHandler
    participant DL as Downloader
    participant G1 as Goroutine 1
    participant G2 as Goroutine 2
    participant G3 as Goroutine 3
    participant CH as Result Channel
    participant P1 as Peer A
    participant P2 as Peer B

    UI->>DH: GET /api/download?name=file.zip&peer=A
    DH->>DL: Download("file.zip", "peerA")
    DL->>P1: FetchFileMeta("file.zip")
    P1-->>DL: FileMeta{chunks: [0,1,2,3,4]}
    DL->>DL: findPeersWithFile() → [Peer A, Peer B]

    Note over DL,P2: Round-robin chunk assignment
    DL->>G1: go downloadChunk(PeerA, chunk0)
    DL->>G2: go downloadChunk(PeerB, chunk1)
    DL->>G3: go downloadChunk(PeerA, chunk2)
    DL->>G1: go downloadChunk(PeerB, chunk3)
    DL->>G2: go downloadChunk(PeerA, chunk4)

    par Concurrent Downloads
        G1->>P1: FetchChunk(hash0)
        G2->>P2: FetchChunk(hash1)
        G3->>P1: FetchChunk(hash2)
    end

    G1->>G1: SHA-256 verify ✓
    G1->>CH: chunkResult{index: 0, data, peer: A}
    G2->>G2: SHA-256 verify ✓
    G2->>CH: chunkResult{index: 1, data, peer: B}
    G3->>CH: chunkResult{index: 2, data, peer: A}

    Note over DL,CH: WaitGroup.Wait() → close(channel)
    DL->>DL: MergeChunks(sorted by index)
    DL-->>DH: DownloadResult{data, chunk_sources, peers_used}
    DH-->>UI: JSON response + save to disk
```

---

## 6. Concurrency Model

```mermaid
graph TB
    subgraph "Background Goroutines"
        G1[UserStore Auto-Save<br/>time.Ticker 30s]
        G2[Session Reaper<br/>time.Ticker 5min]
        G3[Peer Registration<br/>goroutine per peer]
    end

    subgraph "Request Goroutines"
        G4[HTTP Handler per request<br/>net/http default]
        G5[Chunk Download goroutines<br/>sync.WaitGroup + chan]
    end

    subgraph "Thread Safety"
        M1[UserStore.mu<br/>sync.RWMutex]
        M2[SessionManager.mu<br/>sync.RWMutex]
        M3[Peer.Mu<br/>sync.Mutex]
        M4[Registry.Mu<br/>sync.RWMutex]
    end

    G1 -.->|reads/writes| M1
    G2 -.->|reads/writes| M2
    G4 -.->|reads| M1
    G4 -.->|reads/writes| M2
    G5 -.->|writes| M3
    G3 -.->|writes| M4

    style G1 fill:#E4568B,color:white
    style G2 fill:#E4568B,color:white
    style G3 fill:#5D7B3D,color:white
    style G4 fill:#A7C7E4,color:#2A2A2A
    style G5 fill:#F6C94D,color:#2A2A2A
```

---

## 7. HTTP Route Map

| Method | Route | Auth | Handler | Purpose |
|--------|-------|------|---------|---------|
| GET | `/login` | ✗ | `LoginPageHandler` | Serve login page |
| POST | `/api/auth/login` | ✗ | `AuthLoginHandler` | Authenticate user (bcrypt) |
| POST | `/api/auth/register` | ✗ | `AuthRegisterHandler` | Register new user |
| POST | `/api/auth/logout` | ✗ | `AuthLogoutHandler` | Destroy session |
| GET | `/api/auth/status` | ✗ | `AuthStatusHandler` | Get current user info |
| GET | `/` | ✓ | Serve `home.html` | Dashboard page |
| GET | `/upload` | ✓ | Serve `upload.html` | Upload page |
| GET | `/download` | ✓ | `DownloadPageHandler` | Download page |
| POST | `/api/upload` | ✓ | `UploadHandler` | Upload & chunk file |
| GET | `/api/browse` | ✓ | `BrowseFilesHandler` | List network files |
| GET | `/api/download` | ✓ | `DownloadHandler` | Download & reassemble |
| POST | `/api/register` | Network PW | `RegisterPeerHandler` | Peer registration |
| GET | `/api/files` | ✗ | `FileListHandler` | List local files (P2P) |
| GET | `/api/filemeta` | ✗ | `FileMetaHandler` | File metadata (P2P) |
| GET | `/api/chunk` | ✗ | `ChunkHandler` | Serve chunk data (P2P) |
| GET | `/image.png` | ✗ | Static file | Background image |

---

## 8. Data Flow Summary

```mermaid
flowchart LR
    subgraph Upload
        A[Select File] --> B[POST /api/upload]
        B --> C[Parse Multipart]
        C --> D[SplitFile into Chunks]
        D --> E[SHA-256 Hash Each]
        E --> F[Store in Peer State]
    end

    subgraph Download
        G[Browse /api/browse] --> H[Query All Peers]
        H --> I[Select File]
        I --> J[FetchFileMeta]
        J --> K[Discover Peers with File]
        K --> L[Round-Robin Assignment]
        L --> M[Concurrent Goroutines]
        M --> N[Verify SHA-256]
        N --> O[MergeChunks]
        O --> P[Save to downloads/]
    end

    subgraph Auth
        Q[Login Page] --> R[POST credentials]
        R --> S[bcrypt Compare]
        S --> T[Create Session]
        T --> U[Set Cookie]
        U --> V[Access Protected Routes]
    end
```

---

## 9. Security Architecture

| Layer | Mechanism | Implementation |
|-------|-----------|----------------|
| **Password Storage** | bcrypt hash (cost 10) | `golang.org/x/crypto/bcrypt` |
| **Session Tokens** | 256-bit crypto/rand | `crypto/rand` + hex encoding |
| **Session Cookies** | HttpOnly, SameSite=Lax | `http.Cookie` configuration |
| **Network Auth** | bcrypt-hashed network password | Validated on peer registration |
| **Chunk Integrity** | SHA-256 hash verification | Verified on every chunk download |
| **Concurrent Safety** | RWMutex on all shared state | `sync.RWMutex` / `sync.Mutex` |
