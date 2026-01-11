# Environment Variables

This document lists the environment variables used by the Archiver services (API and Worker).

## API Service

| Variable | Default | Required | Description |
| :--- | :--- | :--- | :--- |
| `REDIS_URL` | - | **Yes** | Connection string for the Redis/Valkey instance (e.g., `redis://localhost:6379/0`). |
| `ARCHIVES_DIR` | - | **Yes** | Absolute path to the directory where `.wacz` archives are stored and served from. |

## Worker Service

| Variable | Default | Required | Description |
| :--- | :--- | :--- | :--- |
| `REDIS_URL` | - | **Yes** | Connection string for the Redis/Valkey instance. Must match the API configuration. |
| `ARCHIVES_DIR` | - | No* | Absolute path to the directory where generated archives should be saved. <br>*\*If not set, archives will not be persisted after crawling.* |
| `CRAWLER_TIMEOUT`| `30` | No | Maximum duration (in seconds) allowed for the underlying `browsertrix-crawler` process to run before timing out. |
