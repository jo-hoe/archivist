# ---- build stage ------------------------------------------------------------
# Static, CGO-free build so the binary runs on scratch/distroless on any OS/arch.
FROM golang:1.26-alpine AS build

WORKDIR /src

# Cache dependencies first.
COPY go.mod go.sum ./
RUN go mod download

# Build.
COPY . .
ARG TARGETOS=linux
ARG TARGETARCH=amd64
RUN CGO_ENABLED=0 GOOS=${TARGETOS} GOARCH=${TARGETARCH} \
    go build -trimpath -ldflags="-s -w" -o /out/archivist ./cmd/archivist

# ---- runtime stage ----------------------------------------------------------
# Distroless static: no shell, minimal attack surface, nonroot by default.
FROM gcr.io/distroless/static-debian12:nonroot AS runtime

COPY --from=build /out/archivist /usr/local/bin/archivist

# Default I/O locations; mount volumes here in compose.
WORKDIR /data
ENV ARCHIVIST_SOURCE=/data/in \
    ARCHIVIST_TARGET=/data/out \
    ARCHIVIST_NO_PREVIEW=true

ENTRYPOINT ["/usr/local/bin/archivist"]
CMD ["run"]
