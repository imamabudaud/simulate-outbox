# Transactional Outbox Pattern Simulation

This project demonstrates the transactional outbox pattern vs. direct service calls for handling distributed transactions in microservices.

## Architecture

- **email-service** (port 8081) - Receives email requests, stores in DB with PENDING status
- **notification-service** (port 8082) - Receives notification requests, stores in DB with PENDING status
- **google-analytics** (port 9000) - Mock service for analytics events
- **order-basic** (port 8080) - Basic order processing with direct API calls
- **order-improved** (port 8083) - Improved order processing using outbox pattern
- **email-worker** - Cron worker processing PENDING emails
- **notification-worker** - Cron worker processing PENDING notifications
- **outbox-worker** - Cron worker processing outbox messages

## Prerequisites

- Go 1.24+
- Copy `env.example` to `.env` and adjust configuration if needed

## Running Services

### Start all services in separate terminals:

```bash
# Terminal 1 - Email Service
make email-service

# Terminal 2 - Notification Service  
make notification-service

# Terminal 3 - Google Analytics
make google-analytics

# Terminal 4 - Basic Order Service
make order-basic

# Terminal 5 - Improved Order Service
make outbox-worker
```

### Start workers in separate terminals:

```bash
# Terminal 6 - Email Worker
make email-worker

# Terminal 7 - Notification Worker
make notification-worker

# Terminal 8 - Outbox Worker
make outbox-worker
```

## Running Simulations

### Basic Order Simulation (Direct API calls):
```bash
make test-basic 100
```

### Improved Order Simulation (Outbox pattern):
```bash
make test-improved 100
```

## API Endpoints

### Order Services
- `POST /finish-order` (order-basic) - Process order with direct service calls
- `POST /finish-order-improved` (order-improved) - Process order with outbox pattern

### Email Service
- `POST /send-email` - Store email request
- `GET /emails` - List all emails

### Notification Service
- `POST /send-notification` - Store notification request
- `GET /notifications` - List all notifications

### Google Analytics
- `POST /events` - Process analytics event

## Key Differences

**Basic Order Service:**
- Makes direct HTTP calls to external services
- 30% random failure chance for each service call
- Transaction rollback on any failure
- No compensation for failed external calls

**Improved Order Service:**
- Uses outbox pattern to store messages
- 10% random failure chance for order processing
- Messages processed asynchronously by workers
- Guarantees eventual consistency

## Database Schema

Each service has its own in-memory SQLite database. The outbox table structure:

```sql
CREATE TABLE outbox (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    status TEXT NOT NULL DEFAULT 'PENDING',
    type TEXT NOT NULL,
    data TEXT NOT NULL,
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    finished_at DATETIME
);
```

## Failure Simulation

- **30% random failure** for external service calls (basic service)
- **10% random failure** for order processing (improved service)
- **30% random failure** for outbox message processing
- Workers retry failed messages automatically

## Monitoring

Check service status and data:
- `GET http://localhost:8080/orders` - Basic orders
- `GET http://localhost:8083/orders` - Improved orders  
- `GET http://localhost:8083/outbox` - Outbox messages
- `GET http://localhost:8081/emails` - Email records
- `GET http://localhost:8082/notifications` - Notification records
