# PostgreSQL 16 with the PostGIS extension.
#
# The ready-made postgis/postgis image is built for Intel/AMD only. Installing
# PostGIS on top of the official postgres image works on Apple Silicon too.
FROM postgres:16-bookworm

RUN apt-get update \
    && apt-get install -y --no-install-recommends postgresql-16-postgis-3 \
    && rm -rf /var/lib/apt/lists/*
