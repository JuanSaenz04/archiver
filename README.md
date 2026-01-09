# Archiver

Archiver is a minimalist web archiving solution that crawls websites and generates WACZ (Web Archive Collection Zipped) files. It allows you to both create archives and browse them using an embedded viewer, all from within the same UI.

## Key Features

*   **Privacy & Ownership**: As a self-hosted application, you have full control over your data. All archives are stored locally on your machine.
*   **No Telemetry**: The application does not collect any telemetry or phone home. No data is sent to external servers, with the obvious exception of the websites you choose to crawl.
*   **No Lock-in**: Because the archives are stored in a standard format (WACZ) within a local directory, your files remain portable and can be opened with any compatible viewer.

## Getting Started

Follow these steps to set up and run the Archiver using Docker.

### 1. Configuration

Prepare the configuration files by copying the examples:

```bash
cp docker-compose.example.yml docker-compose.yml
cp .env.example .env
```

### 2. Environment Setup

Open the `.env` file and configure the necessary variables.

*   **Redis Password**: You **must** set a secure password for `REDIS_PASS`. You can generate a strong random password using `openssl`:

    ```bash
    openssl rand -hex 32
    ```

*   **Port**: The default application port is `1080`. You can change this by modifying `APP_PORT` if desired.

### 3. Run the Application

Build and start the services using Docker Compose:

```bash
docker compose up -d --build
```

Once the containers are running, the user interface will be accessible at:

`http://localhost:1080` (or your configured `APP_PORT`)

> [!IMPORTANT]
> **Security Note**: This application does not include built-in authentication or HTTPS. It is strongly recommended to:
> 1. Serve it behind a **Reverse Proxy** (like Nginx, Caddy, or Traefik) for HTTPS termination.
> 2. Use an **Authentication Proxy** (such as [Authelia](https://www.authelia.com/), [Authentik](https://goauthentik.io/), or [Tinyauth](https://tinyauth.app/)) to provide a login layer before accessing the application.

## License
This project is licensed under the AGPLv3 License - see the [LICENSE](LICENSE) file for details.
