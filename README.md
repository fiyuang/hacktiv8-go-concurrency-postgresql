# Read postgres micro-batching with go concurrency

## About

Implementation of micro-batching, using Postgres DB with result of CSV benchmark test

## Configuring

Configure an environment variable for Postgres URL

```shell
DB_URL=postgres://user:password@server:port/database_name?sslmode=disable
```

### Running

Run the benchmark test using:

```shell
$ go run main.go --concurrency=8 --batch-size=1000 --batches=5000
```

Parameters:
* `concurrency`: Number of concurrent workers (goroutines).
* `batch-size`: Number of rows fetched in each batch.
* `batches`: Total number of batches to run.

