package main

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"sync/atomic"
	"time"
)

var count atomic.Int64

func main() {
	port := "9999"
	if p := os.Getenv("PORT"); p != "" {
		port = p
	}

	// 正常接收
	http.HandleFunc("/webhook", func(w http.ResponseWriter, r *http.Request) {
		n := count.Add(1)
		body, _ := io.ReadAll(r.Body)

		fmt.Printf("\n===== Webhook #%d received at %s =====\n", n, time.Now().Format("15:04:05"))
		fmt.Printf("Method:  %s\n", r.Method)
		fmt.Printf("Headers:\n")
		for k, v := range r.Header {
			if k == "X-Hookrelay-Signature" || k == "X-Hookrelay-Event" ||
				k == "X-Hookrelay-Event-Id" || k == "X-Hookrelay-Attempt" ||
				k == "X-Hookrelay-Timestamp" || k == "Content-Type" {
				fmt.Printf("  %s: %s\n", k, v[0])
			}
		}

		var pretty map[string]any
		if json.Unmarshal(body, &pretty) == nil {
			formatted, _ := json.MarshalIndent(pretty, "  ", "  ")
			fmt.Printf("Body:\n  %s\n", string(formatted))
		} else {
			fmt.Printf("Body: %s\n", string(body))
		}

		w.WriteHeader(200)
		w.Write([]byte(`{"status":"ok"}`))
	})

	// 模拟失败（测试重试）
	http.HandleFunc("/webhook/fail", func(w http.ResponseWriter, r *http.Request) {
		n := count.Add(1)
		fmt.Printf("\n===== Webhook #%d → returning 500 =====\n", n)
		w.WriteHeader(500)
		w.Write([]byte(`{"error":"simulated failure"}`))
	})

	// 模拟超时（测试 timeout）
	http.HandleFunc("/webhook/slow", func(w http.ResponseWriter, r *http.Request) {
		n := count.Add(1)
		fmt.Printf("\n===== Webhook #%d → sleeping 60s =====\n", n)
		time.Sleep(60 * time.Second)
		w.WriteHeader(200)
	})

	fmt.Printf("Test receiver listening on :%s\n", port)
	fmt.Printf("  /webhook       → 200 OK\n")
	fmt.Printf("  /webhook/fail  → 500 Error (test retry)\n")
	fmt.Printf("  /webhook/slow  → 60s delay (test timeout)\n\n")
	log.Fatal(http.ListenAndServe(":"+port, nil))
}
