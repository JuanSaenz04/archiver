# Environment Variables

This document lists the environment variables used by the Archiver services (API and Worker).

## API Service

| Variable | Default | Required | Description |
| :--- | :--- | :--- | :--- |
| `REDIS_URL` | - | **Yes** | Connection string for the Redis/Valkey instance (e.g., `redis://localhost:6379/0`). |
| `ARCHIVES_DIR` | - | **Yes** | Absolute path to the directory where `.wacz` archives are stored and served from. |
| `SQLITE_DIR` | `ARCHIVES_DIR` | No | Directory where the SQLite database file (`archive.db`) is stored. If omitted, it **defaults to `ARCHIVES_DIR`**. |
| `LOG_LEVEL` | `info` | No | Logging verbosity for structured logs. Supported values: `debug`, `info`, `warn`/`warning`, `error`. |

## Worker Service

| Variable | Default | Required | Description |
| :--- | :--- | :--- | :--- |
| `REDIS_URL` | - | **Yes** | Connection string for the Redis/Valkey instance. Must match the API configuration. |
| `ARCHIVES_DIR` | - | **Yes** | Absolute path to the directory where generated archives should be saved and where archive files are managed by the worker. |
| `SQLITE_DIR` | `ARCHIVES_DIR` | No | Directory where the SQLite database file (`archive.db`) is stored. If omitted, it **defaults to `ARCHIVES_DIR`**. |
| `CRAWLER_TIMEOUT`| `30` | No | Maximum duration (in seconds) allowed for the underlying `browsertrix-crawler` process to run before timing out. |
| `LOG_LEVEL` | `info` | No | Logging verbosity for structured logs. Supported values: `debug`, `info`, `warn`/`warning`, `error`. |
