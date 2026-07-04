---
layout: default
title: "Dockerfile Installation"
nav_order: 2
description: "Instructions for installing and running aura using a Dockerfile."
parent: Installation
permalink: /installation/docker-file
---

# Dockerfile Installation

To install aura using a Dockerfile, follow these steps:

1. **Clone the Repository**: Start by cloning the aura repository from GitHub.

    ```bash
    git clone https://github.com/jabrown93/aura.git
    cd aura
    ```

2. **Build the Docker Image**: Use the following command to build the Docker image:

    ```bash
    docker build -t aura .
    ```

3. Run the Docker Container (adjust the volume paths and ports as needed):

    ```sh
    docker run -d -p 3000:3000 -p 8888:8888 -v '/mnt/user/appdata/aura/':'/config':'rw' -v '/mnt/user/data/media/':'/data/media':'rw' aura
    ```

4. **Access the Web UI**: Open your web browser and go to `http://localhost:3000` to access the aura web interface.

**Note**: Make sure you have Docker installed on your system before executing these commands. You can check the [official documentation](https://docs.docker.com/get-docker/) for more details.
