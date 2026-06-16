package main

import (
	"context"
	"fmt"
	"log"
	"math/big"
	"math/rand"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/nats-io/nats.go"
)

func getRandPrime() int64 {
	for {
		n := int64(rand.Intn(998) + 2)
		if big.NewInt(n).ProbablyPrime(0) {
			return n
		}
	}
}

func main() {
	natsURL := os.Getenv("NATS_URL")
	if natsURL == "" {
		natsURL = "nats://nats-1:4222,nats://nats-2:4222,nats://nats-3:4222"
	}

	nc, err := nats.Connect(natsURL, nats.MaxReconnects(-1))
	if err != nil {
		log.Fatal(err)
	}
	defer nc.Close()

	// --- 1. INITIALIZE JETSTREAM ---
	js, err := nc.JetStream()
	if err != nil {
		log.Fatal("❌ Failed to create JetStream context: ", err)
	}

	// --- 2. DURABLE PULL SUBSCRIPTION ---
	// We use a Durable name so NATS remembers where this app stopped
	sub, err := js.PullSubscribe("orders.new", "PRIME_PROCESSOR", nats.BindStream("ORDERS"))
	if err != nil {
		log.Fatal("❌ Failed to subscribe to Stream: ", err)
	}

	log.Printf("🚀 Prime JetStream Subscriber started. Waiting for messages on ORDERS stream...")

	httpClient := &http.Client{Timeout: 5 * time.Second}

	for {
		// --- FIX: Create a context with a 5-second deadline ---
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		
		// Fetch messages in batches
		msgs, err := sub.Fetch(10, nats.Context(ctx))
		
		// Always call cancel to release context resources
		cancel()

		if err != nil {
			if err == nats.ErrTimeout || err == context.DeadlineExceeded {
				// This is normal; it just means there are no new messages right now
				continue 
			}
			log.Printf("⚠️ Fetch error: %v", err)
			time.Sleep(1 * time.Second) // Small backoff on actual errors
			continue
		}

		for _, m := range msgs {
			prime := getRandPrime()
			log.Printf("📩 [Seq: %d] Processing: %s", getSequence(m), string(m.Data))

			payload := fmt.Sprintf(`{"message": "%s", "random_prime": %d, "timestamp": "%s"}`,
				string(m.Data), prime, time.Now().Format(time.RFC3339))

			// Index to ZincSearch
			url := "http://zincsearch:4080/api/nats_data_prime/_doc"
			req, _ := http.NewRequest("POST", url, strings.NewReader(payload))
			req.SetBasicAuth("admin", "password")
			req.Header.Set("Content-Type", "application/json")

			resp, err := httpClient.Do(req)
			if err == nil {
				if resp.StatusCode == 200 {
					// --- 3. ACKNOWLEDGE MESSAGE ---
					// Only tell NATS we are done if ZincSearch accepted it
					m.Ack() 
				}
				resp.Body.Close()
			} else {
				log.Printf("❌ ZincSearch Error: %v", err)
				m.Nak() // Tell NATS to retry this specific message later
			}
		}
	}
}

// Helper to get sequence number for logging
func getSequence(m *nats.Msg) uint64 {
	meta, _ := m.Metadata()
	return meta.Sequence.Stream
}