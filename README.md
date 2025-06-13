# Lightfile6 Insights Gateway

A data ingestion service for collecting software usage insights and error reports, then forwarding them to S3.

## Overview

Lightfile6 Insights Gateway is a high-performance data collection service that:
- Receives usage reports, error reports, and specimen files via HTTP API
- Temporarily caches data locally for reliability
- Aggregates and compresses data periodically
- Uploads processed data to Amazon S3 or compatible storage

## Features

- **HTTP API Endpoints** for data ingestion
- **Local caching** for reliability and performance
- **Automatic aggregation** of usage and error reports
- **Compression** using gzip for efficient storage
- **Graceful shutdown** to prevent data loss
- **S3 compatible** storage support (AWS S3, MinIO, etc.)

## Installation

### From Binary

Download the latest release from the [releases page](https://github.com/ideamans/lightfile6-insights-gateway/releases).

### From Source

```bash
go install github.com/ideamans/lightfile6-insights-gateway/cmd/gateway@latest
```

## Configuration

Create a configuration file (see `config.example.yml` for reference):

```yaml
cache_dir: /var/lib/lightfile6-insights-gateway

aws:
  region: ap-northeast-1

s3:
  usage_bucket: lightfile6-usage
  error_bucket: lightfile6-error
  specimen_bucket: lightfile6-specimen

aggregation:
  usage_interval: 10m
  error_interval: 10m
```

## Usage

```bash
lightfile6-insights-gateway -p 8080 -c /path/to/config.yml
```

Options:
- `-p`: Port number (required)
- `-c`: Configuration file path (default: `/etc/lightfile6/config.yml`)

## API Endpoints

### PUT /usage
Upload usage report data.

```bash
curl -X PUT http://localhost:8080/usage \
  -H "USER_TOKEN: username" \
  -H "Content-Type: application/json" \
  -d '{"event": "startup", "timestamp": "2024-01-01T00:00:00Z"}'
```

### PUT /error
Upload error report data.

```bash
curl -X PUT http://localhost:8080/error \
  -H "USER_TOKEN: username" \
  -H "Content-Type: application/json" \
  -d '{"error": "null pointer exception", "timestamp": "2024-01-01T00:00:00Z"}'
```

### PUT /specimen
Upload specimen files.

```bash
curl -X PUT "http://localhost:8080/specimen?uri=screenshot.png" \
  -H "USER_TOKEN: username" \
  -H "Content-Type: image/png" \
  --data-binary @screenshot.png
```

### GET /health
Health check endpoint.

```bash
curl http://localhost:8080/health
```

## Data Flow

1. **Reception**: Data is received via HTTP API
2. **Caching**: Files are temporarily stored in local cache directory
3. **Aggregation**: Usage and error reports are periodically aggregated
4. **Compression**: Aggregated data is compressed using gzip
5. **Upload**: Compressed files are uploaded to S3

## S3 Structure

### Usage/Error Data
```
s3://bucket/prefix/YYYY/MM/DD/HH/YYYYMMDDHH.hostname.jsonl.gz
```

### Specimen Files
```
s3://bucket/prefix/username/YYYY/MM/DD/filename.timestamp.ext
```

## Development

### Building

```bash
go build -o lightfile6-insights-gateway ./cmd/gateway
```

### Testing

```bash
go test ./...
```

### Integration Testing

```bash
go test -tags=integration ./test/integration
```

## License

[License information to be added]