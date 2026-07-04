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
