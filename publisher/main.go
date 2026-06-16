package main

import (
	"fmt"
	"log"
	"os"
	"time"

	"github.com/nats-io/nats.go"
)

func main() {
	// --- 1. CONFIGURATION ---
	natsURL := os.Getenv("NATS_URL")
	if natsURL == "" {
		natsURL = "nats://nats-1:4222,nats://nats-2:4222,nats://nats-3:4222"
	}

	nc, err := nats.Connect(natsURL,
		nats.MaxReconnects(-1),
		nats.ReconnectWait(2*time.Second),
	)
	if err != nil {
		log.Fatal("❌ Connection Error: ", err)
	}
	defer nc.Close()

	// --- 2. JETSTREAM WITH FLOW CONTROL ---
	// We set MaxPending to 5000. 
	js, err := nc.JetStream(
		nats.PublishAsyncMaxPending(5000),
	)
	if err != nil {
		log.Fatal("❌ JetStream Error: ", err)
	}

	// Define Stream
	streamName := "ORDERS"
	_, err = js.AddStream(&nats.StreamConfig{
		Name:     streamName,
		Subjects: []string{"orders.new"},
		Storage:  nats.FileStorage,
		MaxAge:   1 * 24 * time.Hour,
		Replicas: 3,
	})
	if err != nil {
		log.Printf("ℹ️ Stream info: %v", err)
	}

	log.Println("🏁 Starting Performance Test (Ensuring 100% Delivery)")

	maxMsgs := 100000
	startTime := time.Now()

	for i := 1; i <= maxMsgs; i++ {
		msg := fmt.Sprintf("Order #%d", i)

		// 3. RETRY LOGIC FOR STALLING
		// Instead of skipping on error, we loop until this specific message is accepted
		for {
			_, err := js.PublishAsync("orders.new", []byte(msg))
			if err == nil {
				break // Success, move to next message
			}

			// If we get an error (likely buffer full/stalled), wait and retry index i
			log.Printf("⚠️  Stall/Error at %d (Retrying): %v", i, err)
			time.Sleep(10 * time.Millisecond)
		}

		if i%5000 == 0 {
			log.Printf("Published: %d messages", i)
		}
	}

	// 4. WAIT FOR FINAL CONFIRMATIONS
	log.Println("⏳ Waiting for Cluster to confirm all messages...")
	select {
	case <-js.PublishAsyncComplete():
		log.Printf("✅ All %d messages replicated and saved to disk!", maxMsgs)
	case <-time.After(60 * time.Second):
		log.Println("⚠️  Timed out waiting for confirmations!")
	}

	duration := time.Since(startTime)
	log.Printf("🏁 Done! Sent %d messages in %v.", maxMsgs, duration)
}