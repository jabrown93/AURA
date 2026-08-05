---
layout: default
title: "Docker Compose Installation"
nav_order: 1
description: "Instructions for installing and running aura using Docker Compose."
parent: Installation
permalink: /installation/docker-compose
---

# Docker Compose Installation

To install aura using Docker Compose, follow these steps:

1. **Clone the Repository**: Start by cloning the aura repository from GitHub.

    ```bash
    git clone https://github.com/jabrown93/aura.git
    cd aura
    ```

2. **Tweak the Docker Compose File**: Open the `docker-compose.yaml` file in a text editor and adjust the settings to match your environment. You may need to set the correct paths for volumes and ports.

3. **Log in to ghcr.io** (if required): If you need to pull images from GitHub Container Registry, log in using:

```bash
    docker login ghcr.io
```

4. **Run the Application**: Use Docker Compose to build and run the application:

    ```bash
    docker-compose up --build
    ```

    The web interface will now be available at `http://localhost:3000`.

5. **Access the Web UI**: Open your web browser and navigate to `http://localhost:3000` to access the aura web interface.

**Note**: Ensure that Docker is installed and running on your system before executing these commands. You can find more information about Docker installation on the [official Docker website](https://docs.docker.com/get-docker/).

## HTTPS (optional)

aura can serve HTTPS directly if you provide your own TLS certificate and key. When both environment variables below are set, two extra listeners start alongside the plain-HTTP ones:

| Listener | HTTP  | HTTPS |
| -------- | ----- | ----- |
| Web UI   | 3000  | 3443  |
| API      | 8888  | 8443  |

1. Mount your certificate and key into the container (the `/config` volume is a convenient place, e.g. `/config/certs/`). The certificate file should contain the full chain (leaf plus intermediates) in PEM format.

2. Set the environment variables and publish the HTTPS ports in `docker-compose.yml`:

    ```yaml
    ports:
        - "3443:3443" # Web UI HTTPS
        - "8443:8443" # API HTTPS
    environment:
        - TLS_CERT_FILE=/config/certs/tls.crt
        - TLS_KEY_FILE=/config/certs/tls.key
    ```

3. Restart the container. The UI is now available at `https://<host>:3443` and the API at `https://<host>:8443`.

Notes:

- Both variables must be set; setting only one is treated as a configuration error and the container exits.
- The plain-HTTP ports stay active for internal UI-to-API traffic. If you only want HTTPS exposed, simply don't publish ports 3000/8888.
- Certificates are read at startup only — restart the container after rotating them.
- If you already run a reverse proxy (Traefik, Caddy, nginx, ...), keep terminating TLS there instead; this option is for setups without one.
