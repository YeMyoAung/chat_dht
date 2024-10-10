package main

import (
    "bufio"
    "crypto/sha1"
    "encoding/hex"
    "encoding/json"
    "fmt"
    "log"
    "net"
    "os"
    "strings"
    "sync"
)

type Node struct {
    ID      string
    Address string
    Port    int
}

type DHT struct {
    self     Node
    peers    map[string]Node
    peerLock sync.RWMutex
}

type ChatMessage struct {
    Type    string `json:"type"`
    Sender  string `json:"sender"`
    Content string `json:"content"`
    Port    int    `json:"port"`
}

func NewDHT(address string) (*DHT, error) {
    id := generateID(address)
    host, port, _ := net.SplitHostPort(address)
    portInt := 0
    _, err := fmt.Sscanf(port, "%d", &portInt)
    if err != nil {
        return nil, err
    }
    return &DHT{
        self:  Node{ID: id, Address: host, Port: portInt},
        peers: make(map[string]Node),
    }, nil
}

func generateID(addr string) string {
    hash := sha1.Sum([]byte(addr))
    return hex.EncodeToString(hash[:])
}

func (dht *DHT) addPeer(node Node) {
    dht.peerLock.Lock()
    defer dht.peerLock.Unlock()
    if _, exists := dht.peers[node.ID]; !exists {
        dht.peers[node.ID] = node
        log.Printf("Added peer: %s at %s:%d\n", node.ID, node.Address, node.Port)
    }
}

func (dht *DHT) getPeers() []Node {
    dht.peerLock.RLock()
    defer dht.peerLock.RUnlock()
    peers := make([]Node, 0, len(dht.peers))
    for _, peer := range dht.peers {
        peers = append(peers, peer)
    }
    return peers
}

func (dht *DHT) listen() {
    addr, err := net.ResolveUDPAddr("udp", fmt.Sprintf("%s:%d", dht.self.Address, dht.self.Port))
    if err != nil {
        log.Fatalf("Error resolving address: %v", err)
    }

    conn, err := net.ListenUDP("udp", addr)
    if err != nil {
        log.Fatalf("Error listening: %v", err)
    }
    defer func(conn *net.UDPConn) {
        err := conn.Close()
        if err != nil {
            log.Println("Error closing connection:", err)
        }
    }(conn)

    log.Printf("Listening on %s:%d\n", dht.self.Address, dht.self.Port)

    buffer := make([]byte, 1024)
    for {
        n, remoteAddr, err := conn.ReadFromUDP(buffer)
        if err != nil {
            log.Printf("Error reading from UDP: %v", err)
            continue
        }

        var msg ChatMessage
        err = json.Unmarshal(buffer[:n], &msg)
        if err != nil {
            log.Printf("Error unmarshalling message: %v", err)
            continue
        }

        switch msg.Type {
        case "ANNOUNCE":
            newNode := Node{ID: msg.Sender, Address: remoteAddr.IP.String(), Port: msg.Port}
            dht.addPeer(newNode)
            response := ChatMessage{Type: "ACK", Sender: dht.self.ID, Content: dht.self.Address, Port: dht.self.Port}
            responseBytes, _ := json.Marshal(response)
            _, err = conn.WriteToUDP(responseBytes, remoteAddr)
            if err != nil {
                log.Printf("Error sending ACK: %v", err)
            }
        case "ACK":
            dht.addPeer(Node{ID: msg.Sender, Address: msg.Content, Port: msg.Port})
        case "CHAT":
            if msg.Content != "" {
                fmt.Printf("Message from %s: %s\n", msg.Sender, msg.Content)
            }
        }
    }
}

func (dht *DHT) bootstrap(knownPeer string) error {
    conn, err := net.Dial("udp", knownPeer)
    if err != nil {
        return fmt.Errorf("error connecting to bootstrap peer: %v", err)
    }
    defer func(conn net.Conn) {
        err := conn.Close()
        if err != nil {
            log.Println("Error closing connection:", err)
        }
    }(conn)

    announce := ChatMessage{Type: "ANNOUNCE", Sender: dht.self.ID, Content: dht.self.Address, Port: dht.self.Port}
    announceBytes, _ := json.Marshal(announce)
    _, err = conn.Write(announceBytes)
    if err != nil {
        return fmt.Errorf("error sending ANNOUNCE: %v", err)
    }

    buffer := make([]byte, 1024)
    n, err := conn.Read(buffer)
    if err != nil {
        return fmt.Errorf("error reading response: %v", err)
    }

    var response ChatMessage
    err = json.Unmarshal(buffer[:n], &response)
    if err != nil {
        return fmt.Errorf("error unmarshalling response: %v", err)
    }

    if response.Type == "ACK" {
        dht.addPeer(Node{ID: response.Sender, Address: response.Content, Port: response.Port})
    }

    log.Println("Bootstrap complete")
    return nil
}

func (dht *DHT) sendMessage(content string) {
    content = strings.TrimSpace(content)
    if content == "" {
        return
    }

    msg := ChatMessage{
        Type:    "CHAT",
        Sender:  dht.self.ID,
        Content: content,
        Port:    dht.self.Port,
    }
    msgBytes, _ := json.Marshal(msg)

    peers := dht.getPeers()
    if len(peers) == 0 {
        fmt.Println("No peers connected. Message not sent.")
        return
    }

    for _, peer := range peers {
        go Handler(peer, msgBytes)
    }
}

func Handler(peer Node, msgBytes []byte) {
    addr := fmt.Sprintf("%s:%d", peer.Address, peer.Port)
    conn, err := net.Dial("udp", addr)
    if err != nil {
        log.Printf("Error connecting to peer %s: %v", peer.ID, err)
        return
    }
    defer func(conn net.Conn) {
        err := conn.Close()
        if err != nil {
            log.Println("Error closing connection:", err)
        }
    }(conn)
    _, err = conn.Write(msgBytes)
    if err != nil {
        log.Printf("Error sending message to peer %s: %v", peer.ID, err)
    } else {
        log.Printf("Message sent to peer %s at %s", peer.ID, addr)
    }
}

func main() {
    if len(os.Args) < 2 {
        log.Fatalf("Usage: %s <listen_address> [bootstrap_peer]", os.Args[0])
    }

    listenAddr := os.Args[1]
    dht, err := NewDHT(listenAddr)
    if err != nil {
        log.Fatalf("Error creating DHT: %v", err)
    }

    go dht.listen()

    if len(os.Args) == 3 {
        bootstrapPeer := os.Args[2]
        err := dht.bootstrap(bootstrapPeer)
        if err != nil {
            log.Printf("Error bootstrapping: %v", err)
        }
    }

    log.Println("Chat application started. Type your messages and press Enter to send.")
    log.Println("Type '/quit' to exit the application.")

    scanner := bufio.NewScanner(os.Stdin)
    for scanner.Scan() {
        message := scanner.Text()

        if message == "/quit" {
            log.Println("Exiting chat application...")
            break
        }

        dht.sendMessage(message)
    }

    if err := scanner.Err(); err != nil {
        log.Printf("Error reading input: %v", err)
    }
}
