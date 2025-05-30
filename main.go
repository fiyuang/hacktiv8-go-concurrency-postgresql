package main

import (
	"context"
	"encoding/csv"
	"flag"
	"fmt"
	"log"
	"math/rand"
	"os"
	"runtime"
	"strconv"
	"sync"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

var (
	dbURL         string
	concurrency   = flag.Int("concurrency", 10, "Number of concurrent workers")
	batchSize     = flag.Int("batch-size", 1000, "Number of rows per batch")
	numBatches    = flag.Int("batches", 5000, "Total number of batches")
	noConcurrency = flag.Bool("no-concurrency", false, "Disable concurrency (no goroutines)")
)

func init() {
	flag.Parse()
	dbURL = os.Getenv("DB_URL")
	if dbURL == "" {
		log.Fatal("Missing required environment variable: DB_URL")
	}
}

func fetchBatch(ctx context.Context, pool *pgxpool.Pool, ids []string) float64 {
	start := time.Now()
	query := `SELECT pk, payload FROM random_read_test WHERE pk = ANY($1)`
	rows, err := pool.Query(ctx, query, ids)
	if err != nil {
		log.Printf("Query error: %v", err)
		return 0
	}
	defer rows.Close()

	for rows.Next() {
		var id, payload string
		_ = rows.Scan(&id, &payload)
	}
	return time.Since(start).Seconds() * 1000 // ms
}

func printPoolStats(pool *pgxpool.Pool, label string) {
	stats := pool.Stat()
	fmt.Printf("[POOL %s] Total: %d | InUse: %d | Idle: %d | Max: %d\n",
		label,
		stats.TotalConns(),
		stats.AcquiredConns(),
		stats.IdleConns(),
		stats.MaxConns())
}

func main() {
	ctx := context.Background()

	config, err := pgxpool.ParseConfig(dbURL)
	if err != nil {
		log.Fatalf("Failed to parse DB URL: %v", err)
	}

	pool, err := pgxpool.NewWithConfig(ctx, config)
	if err != nil {
		log.Fatalf("Unable to connect to database: %v", err)
	}
	defer pool.Close()

	rand.Seed(time.Now().UnixNano())
	var totalDuration float64

	fmt.Println(">> Initial Runtime:")
	printPoolStats(pool, "START")
	fmt.Printf("Goroutines: %d\n\n", runtime.NumGoroutine())

	startAll := time.Now()

	if *noConcurrency {
		for i := 0; i < *numBatches; i++ {
			ids := make([]string, *batchSize)
			for i := range ids {
				ids[i] = strconv.FormatInt(int64(rand.Intn(3_000_000)+1), 10)
			}
			dur := fetchBatch(ctx, pool, ids)
			totalDuration += dur

			if i%100 == 0 {
				fmt.Printf("[Batch %d] Goroutines: %d\n", i, runtime.NumGoroutine())
				printPoolStats(pool, fmt.Sprintf("BATCH %d", i))
			}
		}
	} else {
		var durLock sync.Mutex
		wg := sync.WaitGroup{}
		sem := make(chan struct{}, *concurrency)

		for i := 0; i < *numBatches; i++ {
			wg.Add(1)
			sem <- struct{}{}

			go func(batchNum int) {
				defer wg.Done()
				defer func() { <-sem }()
				ids := make([]string, *batchSize)
				for i := range ids {
					ids[i] = strconv.FormatInt(int64(rand.Intn(3_000_000)+1), 10)
				}
				dur := fetchBatch(ctx, pool, ids)

				durLock.Lock()
				totalDuration += dur
				durLock.Unlock()

				if batchNum%100 == 0 {
					fmt.Printf("[Batch %d] Goroutines: %d\n", batchNum, runtime.NumGoroutine())
					printPoolStats(pool, fmt.Sprintf("BATCH %d", batchNum))
				}
			}(i)
		}
		wg.Wait()
	}

	endAll := time.Since(startAll)
	effectiveAvg := endAll.Seconds() * 1000 / float64(*numBatches)

	fmt.Println("\n>> Final Runtime:")
	printPoolStats(pool, "END")
	fmt.Printf("Goroutines: %d\n", runtime.NumGoroutine())

	// Output summary
	fmt.Println("========== Benchmark Result ==========")
	fmt.Printf("Total batches             : %d\n", *numBatches)
	fmt.Printf("Total time                : %.3fs\n", endAll.Seconds())
	fmt.Printf("Avg per batch (raw)       : %.3fms\n", totalDuration/float64(*numBatches))
	fmt.Printf("Avg per batch (effective) : %.3fms\n", effectiveAvg)
	if *noConcurrency {
		fmt.Println("Concurrency Level         : 0 (Non-Concurrent)")
	} else {
		fmt.Printf("Concurrency Level         : %d\n", *concurrency)
	}
	fmt.Println("CSV summary saved to: benchmark_result.csv")
	fmt.Println("======================================")

	// Write to CSV
	summaryFile := "benchmark_result.csv"
	writeHeader := false
	if _, err := os.Stat(summaryFile); os.IsNotExist(err) {
		writeHeader = true
	}

	f, err := os.OpenFile(summaryFile, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		log.Printf("Failed to write CSV summary: %v", err)
		return
	}
	defer f.Close()

	writer := csv.NewWriter(f)
	defer writer.Flush()

	if writeHeader {
		writer.Write([]string{
			"timestamp", "concurrency", "batch_size", "total_batches",
			"total_time_sec", "avg_per_batch_ms", "effective_avg_batch_ms", "no_concurrency",
		})
	}

	writer.Write([]string{
		time.Now().Format("2006-01-02 15:04:05"),
		func() string {
			if *noConcurrency {
				return "0"
			}
			return strconv.Itoa(*concurrency)
		}(),
		strconv.Itoa(*batchSize),
		strconv.Itoa(*numBatches),
		fmt.Sprintf("%.3f", endAll.Seconds()),
		fmt.Sprintf("%.3f", totalDuration/float64(*numBatches)),
		fmt.Sprintf("%.3f", effectiveAvg),
		strconv.FormatBool(*noConcurrency),
	})
}
