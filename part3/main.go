package main

import (
	"bufio"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	_ "github.com/joho/godotenv/autoload"

	"github.com/davecgh/go-spew/spew"
	"github.com/gorilla/mux"
)

const (
	difficulty = 1
)

var (
	mu = new(sync.Mutex)
)

type Block struct {
	Index      int
	Timestamp  string
	BPM        int
	Hash       string
	PrevHash   string
	Difficulty int
	Nonce      string
}

var Blockchain []Block

func calculateHash(block Block) string {
	record := strconv.Itoa(block.Index) + block.Timestamp + strconv.Itoa(block.BPM) + block.PrevHash + block.Nonce
	sum := sha256.Sum256([]byte(record))
	return hex.EncodeToString(sum[:])
}

func generateBlock(oldBlock Block, BPM int) Block {

	var newBlock Block

	t := time.Now()

	newBlock.Index = oldBlock.Index + 1
	newBlock.Timestamp = t.String()
	newBlock.BPM = BPM
	newBlock.PrevHash = oldBlock.Hash
	newBlock.Difficulty = difficulty

	for i := 0; ; i++ {
		hex := fmt.Sprintf("%x", i)
		newBlock.Nonce = hex
		if !isHashValid(calculateHash(newBlock), newBlock.Difficulty) {
			fmt.Println(calculateHash(newBlock), " do more work!")
			time.Sleep(time.Second)
			continue
		} else {
			fmt.Println(calculateHash(newBlock), " work done!")
			newBlock.Hash = calculateHash(newBlock)
			break
		}
	}

	return newBlock
}

func isBlockValid(newBlock, oldBlock Block) bool {
	if oldBlock.Index+1 != newBlock.Index {
		return false
	}

	if oldBlock.Hash != newBlock.PrevHash {
		return false
	}

	return calculateHash(newBlock) == newBlock.Hash
}

func replaceChain(newBlocks []Block) {
	if len(newBlocks) > len(Blockchain) {
		Blockchain = newBlocks
	}
}

func run() error {
	r := makeMuxRouter()
	port := os.Getenv("PORT")
	log.Println("Listening on", port)
	server := &http.Server{
		Addr:           ":" + port,
		Handler:        r,
		ReadTimeout:    10 * time.Second,
		WriteTimeout:   10 * time.Second,
		MaxHeaderBytes: 1 << 20,
	}

	if err := server.ListenAndServe(); err != nil {
		return err
	}

	return nil
}

func makeMuxRouter() http.Handler {
	r := mux.NewRouter()
	r.HandleFunc("/", handleGetBlockchain).Methods("GET")
	r.HandleFunc("/", handleWriteBlock).Methods("POST")
	return r
}

func handleGetBlockchain(w http.ResponseWriter, r *http.Request) {
	bytes, err := json.MarshalIndent(Blockchain, "", "  ")
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Write(bytes)
}

type Message struct {
	BPM int
}

func handleWriteBlock(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	var m Message

	decoder := json.NewDecoder(r.Body)
	if err := decoder.Decode(&m); err != nil {
		respondWithJSON(w, r, http.StatusBadRequest, r.Body)
		return
	}
	defer r.Body.Close()

	mu.Lock()
	newBlock := generateBlock(Blockchain[len(Blockchain)-1], m.BPM)
	mu.Unlock()

	if isBlockValid(newBlock, Blockchain[len(Blockchain)-1]) {
		newBlockchain := append(Blockchain, newBlock)
		replaceChain(newBlockchain)
		spew.Dump(Blockchain)
	}

	respondWithJSON(w, r, http.StatusCreated, newBlock)
}

func respondWithJSON(w http.ResponseWriter, r *http.Request, code int, payload interface{}) {
	response, err := json.MarshalIndent(payload, "", "  ")
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte("HTTP 500: Internal Server Error"))
		return
	}
	w.WriteHeader(code)
	w.Write(response)
}

func isHashValid(hash string, difficulty int) bool {
	prefix := strings.Repeat("0", difficulty)
	return strings.HasPrefix(hash, prefix)
}

var bcServer chan []Block

func handleConn(conn net.Conn) {
	defer conn.Close()

	io.WriteString(conn, "Enter a new BPM:")

	sc := bufio.NewScanner(conn)

	go func() {
		for sc.Scan() {
			bpm, err := strconv.Atoi(sc.Text())
			if err != nil {
				log.Printf("%v not a number: %v", sc.Text(), err)
				continue
			}
			newBlock := generateBlock(Blockchain[len(Blockchain)-1], bpm)
			if isBlockValid(newBlock, Blockchain[len(Blockchain)-1]) {
				newBlockchain := append(Blockchain, newBlock)
				replaceChain(newBlockchain)
			}

			bcServer <- Blockchain
			io.WriteString(conn, "\nEnter a new BPM:")
		}
	}()

	go func() {
		for {
			time.Sleep(30 * time.Second)
			bytes, err := json.Marshal(Blockchain)
			if err != nil {
				log.Fatal(err)
			}
			conn.Write(bytes)
		}
	}()

	for range bcServer {
		spew.Dump(Blockchain)
	}
}

func main() {

	bcServer = make(chan []Block)

	t := time.Now()
	genesisBlock := Block{0, t.String(), 0, "", "", difficulty, ""}
	spew.Dump(genesisBlock)
	Blockchain = append(Blockchain, genesisBlock)

	log.Fatal(run())

	// server, err := net.Listen("tcp", ":"+os.Getenv("PORT"))
	// if err != nil {
	// 	log.Fatal(err)
	// }
	// defer server.Close()

	// for {
	// 	conn, err := server.Accept()
	// 	if err != nil {
	// 		log.Fatal(err)
	// 	}
	// 	go handleConn(conn)
	// }
}
