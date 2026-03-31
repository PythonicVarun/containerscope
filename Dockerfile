ARG GO_VERSION=1.26

FROM golang:${GO_VERSION}-alpine AS build
WORKDIR /src
COPY go.mod ./
COPY cmd ./cmd
COPY internal ./internal
COPY public ./public
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /out/containerscope ./cmd/containerscope

FROM scratch
ARG VERSION=dev
ARG BUILD_DATE=unknown
ARG VCS_REF=unknown

LABEL org.opencontainers.image.title="ContainerScope" \
      org.opencontainers.image.description="Live Docker container log viewer for Docker hosts" \
      org.opencontainers.image.version="${VERSION}" \
      org.opencontainers.image.created="${BUILD_DATE}" \
      org.opencontainers.image.revision="${VCS_REF}"

WORKDIR /app
ENV CONTAINER_SCOPE_PORT=4000
COPY --from=build /out/containerscope /app/containerscope
COPY public /app/public
EXPOSE 4000
CMD ["/app/containerscope"]
