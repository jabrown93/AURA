############################################################################
##### Stage 1: Build the backend application
############################################################################
FROM dhi.io/golang:1.26.5-alpine3.24-dev AS backend-builder

# cgo build deps (gcc, musl-dev) plus ca-certificates + tzdata, which are copied
# into the shellless runtime image (which has no package manager of its own).
RUN apk add --no-cache gcc musl-dev ca-certificates tzdata

# Set the working directory
WORKDIR /backend

# Copy the go.mod and go.sum files
COPY backend/go.mod backend/go.sum ./

# Download the dependencies
RUN go mod download

# Copy the source code
COPY backend/ ./

# Get the version from build arguments/environment variables
ARG APP_VERSION=dev

# Enable CGO and build the application
ENV CGO_ENABLED=1
RUN go build -ldflags="-s -w -X main.APP_VERSION=$APP_VERSION" -o main .

############################################################################
##### Stage 2: Build the frontend application
############################################################################
FROM dhi.io/node:26.4.0-alpine3.24-dev AS frontend-builder

# Set the working directory
WORKDIR /frontend

# Copy the package.json and package-lock.json files
COPY frontend/package*.json ./

# Install the dependencies
RUN npm ci

# Copy the source code
COPY frontend/ ./

# Get the port number and version from build arguments/environment variables
ARG APP_VERSION=dev

# Set environment variables
ENV NEXT_PUBLIC_APP_VERSION=${APP_VERSION}
ENV NEXT_TELEMETRY_DISABLED=1

# Build the application
RUN npm run build || (echo "Build failed" && cat /frontend/.next/build-diagnostics.json 2>/dev/null || true && exit 1)

# Normalize runtime-readable permissions for static assets before they are copied
# into the shellless runtime image, which has no shell to run chmod. Local git
# checkouts can carry restrictive modes (for example from a 077 umask) that COPY
# would otherwise preserve.
RUN find /frontend/public -type d -exec chmod 755 {} + \
	&& find /frontend/public -type f -exec chmod 644 {} + \
	&& find /frontend/.next/static -type d -exec chmod 755 {} + \
	&& find /frontend/.next/static -type f -exec chmod 644 {} +

############################################################################
##### Stage 3: Build the final image
############################################################################
FROM dhi.io/node:26.4.0-alpine3.24 AS final

# Set the working directory
WORKDIR /app

# CA certificates (the Go backend makes HTTPS calls) and tzdata (cron/timezone
# support) come from the -dev builder stage: this runtime image is shellless and
# rootless, so there is no apk to install them here.
COPY --from=backend-builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/ca-certificates.crt
COPY --from=backend-builder /usr/share/zoneinfo /usr/share/zoneinfo

# Copy the backend application from the builder stage
COPY --from=backend-builder /backend/main /app/main

# Copy the frontend build from the builder stage (permissions were normalized in
# the frontend-builder stage, since there is no shell here to run chmod).
COPY --from=frontend-builder /frontend/.next/standalone /app/
COPY --from=frontend-builder /frontend/.next/static /app/.next/static
COPY --from=frontend-builder /frontend/public /app/public

# Process supervisor: node spawns and watches both the Go backend and the Next
# server. A shellless image can't run `sh -c "./main & node server.js"`, so node
# (the image's runtime) does the job instead.
COPY docker/launcher.mjs /app/launcher.mjs

# Set environment variables
ENV NODE_ENV=production
ENV HOME=/tmp
ENV XDG_CACHE_HOME=/tmp/.cache

# Expose the ports for both the backend and frontend
EXPOSE 3000
EXPOSE 8888

# Run both processes under the node supervisor (shellless, so no `sh -c`).
ENTRYPOINT ["node", "/app/launcher.mjs"]
