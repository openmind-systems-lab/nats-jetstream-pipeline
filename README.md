# NATS JetStream Distributed Pipeline

A high-performance, distributed messaging architecture utilizing a 3-node NATS JetStream cluster, PostgreSQL for persistent storage, and ZincSearch for data indexing.

## 🏗 Architecture

The system follows a producer-consumer pattern with high-availability and monitoring integrated into the stack.

```mermaid
%%{init: {
  'theme': 'base', 
  'look': 'handDrawn', 
  'themeVariables': { 
    'fontFamily': 'Comic Sans MS, cursive',
    'primaryColor': '#ffffff',
    'mainBkg': '#ffffff',
    'lineColor': '#444444'
  }
}}%%
%%{init: {'theme': 'neutral'}}%%
graph TD
    subgraph Publisher_Layer [Publisher Layer]
        PUB[Publisher Go App]
    end

    subgraph NATS_Cluster [NATS JetStream Cluster]
        N1[NATS Node 1]
        N2[NATS Node 2]
        N3[NATS Node 3]
        N1 <--> N2
        N2 <--> N3
        N3 <--> N1
    end

    subgraph Subscriber_Layer [Subscriber Layer]
        SUB[Main Subscriber]
        SUB_P[Subscriber Prime]
    end

    subgraph Storage_Layer [Storage Layer]
        DB[(PostgreSQL DB)]
        ZINC[[ZincSearch]]
    end

    subgraph Monitoring_Layer [Monitoring Layer]
        EXP[NATS Exporter]
        PROM[Prometheus]
        GRAF[Grafana]
    end

    %% Data Flow
    PUB -- "orders.new (Async)" --> NATS_Cluster
    NATS_Cluster -- "Batch Pull (1000)" --> SUB
    NATS_Cluster -- "Pull Subscribe (10)" --> SUB_P
    
    SUB -- "SQL Batch Insert" --> DB
    SUB_P -- "Enrichment + HTTP POST" --> ZINC
    
    %% Monitoring Flow
    NATS_Cluster -. "HTTP Metrics" .-> EXP
    EXP -. "Scrape" .-> PROM
    ZINC -. "Scrape" .-> PROM
    PROM -. "Query" .-> GRAF

    %% Styling
    style PUB fill:#e6ccb2,stroke:#a68a64,stroke-width:2px,rx:10,ry:10,color:#000
    style N1 fill:#ffcccc,stroke:#cc0000,stroke-width:2px,rx:10,ry:10,color:#000
    style N2 fill:#ffcccc,stroke:#cc0000,stroke-width:2px,rx:10,ry:10,color:#000
    style N3 fill:#ffcccc,stroke:#cc0000,stroke-width:2px,rx:10,ry:10,color:#000
    style SUB fill:#a2d2ff,stroke:#0056b3,stroke-width:2px,rx:10,ry:10,color:#000
    style SUB_P fill:#a2d2ff,stroke:#0056b3,stroke-width:2px,rx:10,ry:10,color:#000
    style DB fill:#d8f3dc,stroke:#2d6a4f,stroke-width:2px,rx:10,ry:10,color:#000
    style ZINC fill:#ffb703,stroke:#e85d04,stroke-width:2px,rx:10,ry:10,color:#000
    style EXP fill:#f96,stroke:#a33b00,stroke-width:2px,color:#000
    style PROM fill:#f96,stroke:#a33b00,stroke-width:3px,color:#000,font-weight:bold
    style GRAF fill:#d8f3dc,stroke:#2d6a4f,stroke-width:2px,rx:10,ry:10,color:#000
    
    style Publisher_Layer fill:#f0f8ff,stroke:#2563eb,stroke-width:2px
    style NATS_Cluster fill:#fff1f2,stroke:#e11d48,stroke-width:2px
    style Subscriber_Layer fill:#f0fdf4,stroke:#16a34a,stroke-width:2px
    style Storage_Layer fill:#fff7ed,stroke:#ea580c,stroke-width:2px
    style Monitoring_Layer fill:#f5f3ff,stroke:#7c3aed,stroke-width:2px
```

### Components
- **Publisher**: Asynchronous Go application featuring flow control via `PublishAsyncMaxPending(5000)` and a retry-on-stall loop to ensure 100% delivery of 100,000 messages. **Note: The publisher is responsible for creating the JetStream `ORDERS` stream.**
- **NATS Cluster**: 3-node cluster configured with JetStream, using `FileStorage` and 3-way replication (`Replicas: 3`) for high availability.
- **Main Subscriber**: Durable pull consumer that utilizes SQL transactions to process and save messages in batches of 1000 to PostgreSQL.
- **Subscriber Prime**: Secondary consumer that enriches data with random primes and indexes the results into ZincSearch.
- **Monitoring**: A full observability stack where Prometheus scrapes the NATS Exporter and ZincSearch, visualized through a specialized Grafana dashboard.

## 🚀 Getting Started

### Prerequisites
- Docker & Docker Compose
- Go 1.23+ (specified in mod files)

### ⚠️ Important Startup Sequence
Because the **Publisher** programmatically creates the `ORDERS` JetStream in NATS upon startup, the **Subscribers** must wait for the stream to exist before they can bind to it. 
If a subscriber starts before the stream is created, it will throw a `Subscription Error`. **Always ensure the publisher has initialized the stream first!**

### Deployment & Docker Compose Commands

1. **Prepare Host Directories**: Ensure your Windows host has the following paths for persistent volumes:
   - `C:/temp/nats-data-1`
   - `C:/temp/nats-data-2`
   - `C:/temp/nats-data-3`
   - `C:/temp/postgres-app-data`

2. **Build the infrastructure and applications**:
   ```bash
   docker-compose -f docker-compose-cluster.yml build
   ```

3. **Start the cluster and services in the background**:
   ```bash
   docker-compose -f docker-compose-cluster.yml up -d
   ```

4. **View live logs for a specific service** (e.g., to watch the publisher):
   ```bash
   docker logs -f nats-publisher-1
   ```

5. **Stop all services**:
   ```bash
   docker-compose -f docker-compose-cluster.yml down
   ```

6. **Hard Reset (Stop and remove volumes)** - *Use this if you want to wipe the database and NATS storage to start completely fresh:*
   ```bash
   docker-compose -f docker-compose-cluster.yml down -v
   ```

## 📊 Monitoring & Access
- **Grafana**: `http://localhost:3001` (Credentials: admin/admin)
- **Prometheus**: `http://localhost:9090`
- **ZincSearch**: `http://localhost:4080` (Credentials: admin/password)
- **Cobra NATS (UI)**: `http://localhost:8080`

## 🛠 Features
- **Data Integrity**: The publisher implements a blocking retry mechanism that stays on the same message index until confirmed by the cluster.
- **Efficiency**: The Main Subscriber uses prepared statements and batch commits (1000 messages) to minimize database overhead.
- **Resilience**: Durable subscriptions (`MAIN_BATCH_WORKER` and `PRIME_PROCESSOR`) allow consumers to resume processing exactly where they left off after a restart or crash.
