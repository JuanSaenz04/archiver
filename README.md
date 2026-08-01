# Archiver

Archiver is a minimalist web archiving solution that crawls websites and generates WACZ (Web Archive Collection Zipped) files. It allows you to both create archives and browse them using an embedded viewer, all from within the same UI.

## Key Features

*   **Privacy & Ownership**: As a self-hosted application, you have full control over your data. All archives are stored locally on your machine.
*   **No Telemetry**: The application does not collect any telemetry or phone home. No data is sent to external servers, with the obvious exception of the websites you choose to crawl.
*   **No Lock-in**: Because the archives are stored in a standard format (WACZ) within a local directory, your files remain portable and can be opened with any compatible viewer.

## Getting Started

Follow these steps to set up and run Archiver using the pre-built Docker images published to GHCR:

* `ghcr.io/juansaenz04/archiver-api`
* `ghcr.io/juansaenz04/archiver-worker`

### 1. Configuration

Prepare the configuration files by copying the examples:

```bash
cp docker-compose.example.yml docker-compose.yml
cp .env.example .env
```

### 2. Environment Setup

Open the `.env` file and configure the necessary variables. For a full list of available options, see the [Environment Variables documentation](docs/env_variables.md).

*   **Redis Password**: You **must** set a secure password for `REDIS_PASS`. You can generate a strong random password using `openssl`:

    ```bash
    openssl rand -hex 32
    ```

*   **Origins**: Set `APP_PUBLIC_URL` and `REPLAY_PUBLIC_URL` to two different public origins. For local use, the defaults are `http://localhost:1080` and `http://localhost:1081`.
*   **Ports**: The frontend/API use `1080`; the isolated archive replay server uses `1081`. The example Compose file publishes both through `APP_PORT` and `REPLAY_PORT`.

### 3. Run the Application

Pull the pre-built images and start the services using Docker Compose:

```bash
docker compose pull
docker compose up -d
```

Docker Compose will use the `latest` GHCR images by default. Once the containers are running, the user interface and replay server will be accessible at:

`http://localhost:1080` (or your configured `APP_PORT`)

`http://localhost:1081` (or your configured `REPLAY_PORT`)

For production, route separate HTTPS origins to the two ports, for example:

```text
https://archiver.example.com -> container:1080
https://replay.example.com   -> container:1081
```

Both origins should be protected by your authentication proxy. If the reverse proxy connects directly to the container network, the ports do not need to be published on the host.

For frontend development with `pnpm dev`, run the Go API with `APP_PUBLIC_URL=http://localhost:5173` and `REPLAY_PUBLIC_URL=http://localhost:1081`. Vite proxies `/api` to port `1080`, while the iframe connects directly to the replay server.

> [!IMPORTANT]
> **Security Note**: This application does not include built-in authentication or HTTPS. It is strongly recommended to:
> 1. Serve it behind a **Reverse Proxy** (like Nginx, Caddy, or Traefik) for HTTPS termination.
> 2. Use an **Authentication Proxy** (such as [Authelia](https://www.authelia.com/), [Authentik](https://goauthentik.io/), or [Tinyauth](https://tinyauth.app/)) to provide a login layer before accessing the application.
>
> **Archive viewer trust model**: Archived pages can contain JavaScript. Archiver isolates replay on the origin configured by `REPLAY_PUBLIC_URL`; never route that origin to port `1080`, or the main origin to port `1081`. The replay server intentionally exposes only viewer assets and read-only archive delivery. State-changing API requests are accepted only from `APP_PUBLIC_URL`.

## License
This project is licensed under the AGPLv3 License - see the [LICENSE](LICENSE) file for details.
