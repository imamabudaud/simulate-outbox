# Transactional Outbox sample project

## Order Finish worker

* It will do four things:
** update order status to FINISHED
** call `email-service` to send email to current customer related to the order
** call `notification-service` to send notification to current customer
** send marketing events to google analytics (order success metrics)*

## Structure

### email-service
* Located on `/email-service`
* Trigger via http call on `POST http://localhost:8081/send-email`, with parameter (recipients email, subject, body)
* Email service will put the data in the database with status PENDING and process later 
* Database is in memory database

### email-sender worker
* Cron worker, run each 10 second, fetch data in database with status PENDING
* Not actually sending the email, just print the content of data
* Update the data with status SENT

### notification-service
* Located on `/notification-service`
* Trigger via http call on `POST http://localhost:8082/send-notification`, with parameter (device id, message)
* Notification service will put the data in the database with status PENDING and process later 
* Database is in memory database

### notification-sender worker
* Cron worker, run each 10 second, fetch data in database with status PENDING
* Not actually sending the email, just print the content of data
* Update the data with status SENT

### google-analytics service
* Located on `/google-analytics` folder
* Trigger via http call on `POST http://localhost:9000/events` with parameter json payload (raw)
* No need to store to db, just return success
* This is an emulation of real Google Analytics

### order-basic
* Located on `/order-basic` folder, where our core logic for order finish
* Trigger the worker via http api call, `POST http://localhost:8080/finish-order` with parameter (order id, user name, user email, user device id). These parameters are to simplify the simulation, so that we dont have to provide user tables, or device tables. All required info are provided in this phase
* It will process immediately to:
** Update the order status to FINISHED
** Call email service and notification service for finish order
** Call google analytics for marketing metrics
* Add 30% random chance any of those four are failed, on failed, rollback db transaction (no compensation for three http api call, let it be)

### order-improved
* Located on `/order-improved` folder, 
* Trigger the worker via http api call, `POST http://localhost:8083/finish-order-improved` with same parameter of order-basic
* It will process:
** Update the order status to FINISHED
** Store to outbox: EMAIL, status PENDING
** Store to outbox: NOTIFY, status PENDING
** Store to outbox: ANALYTIC, status PENDING
* Give a 10% random failure for this process. On failure, just rollback and return to http caller the error

### outbox worker
* Located on `/outbox-worker`
* Cron task that run each 10 second, load all outbox message with status PENDING
* for each type, create each service for each type of the worker, so that we could emulate the retry, backoff period, etc (just put as comments and print to stdout)
* Add 30% random chance the process failed, if failed, just print to stdout, it will be picked up later
* type EMAIL, will call email-service just like `order-basic` flow
* type NOTIFY, will call notification-service like `order-basic` flow
* type ANALYTIC, will call google analitics (mock service)

## Outbox structure
Attributes:
* id, auto-increment, basic integer
* status, PENDING, PROCESSING, FINISHED, FAILED
* type, EMAIL, NOTIFY, ANALYTIC
* data, json data based on type 
** EMAIL: recipients (array), subject (string), body (string)
** NOTIFY: deviceId (array), message (text)
** ANALYTIC: payload (text of raw json)
* created_at, zoned datetime
* finished_at, zoned datetime

## Tech stacks
* Db is inmemory, each service has its own database
* Go 1.24, no generic, simple go, no module, no workspace
* Use echo as framework
* Log use built-in slog
* No need to create a unit test, the project should be lean, instead create a `/test-simulation` folder, where there are two commands that can be run inside: `go run /test-simulation basic 100`, where it will simulate 100 orders of `basic order` service. and another one `go run /test-simulation improved 100` where it will run 100 order simulation to `improved order` service
* Use godotenv to load .env 
* .env contains: service-name (that will be printed out when starting the service), server port, (for worker) cron period (in seconds)
* No readme.md or any markdown
* No comments on the code, make the code speak itself
* No docker, the code will be run via `cmd ./cmd/main.go {service-name}`
* Use viper for managing the entry point to run services/workers