# Multi-stage build: Node builds the static files, nginx serves them.

# ---- Stage 1: build ----
FROM node:22-slim AS build
WORKDIR /app

COPY package.json package-lock.json ./
RUN npm ci --no-audit --no-fund

COPY . .

# Parcel puts API_URL into the JavaScript bundle at build time, so it must be
# known here. It is the backend address as seen from the user's browser.
ARG API_URL=http://localhost:8080
ENV API_URL=$API_URL
RUN npm run build

# ---- Stage 2: serve ----
FROM nginx:1.29-alpine
COPY --from=build /app/dist /usr/share/nginx/html
EXPOSE 80
