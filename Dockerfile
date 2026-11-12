# Build: docker build -t knitnox .
#        docker build --platform linux/arm64 -t knitnox .     (ARM server; cross-compiles, no emulation)
# Run:   docker run -p 8080:8080 -v knitnox-data:/data -e KNITNOX_SECRET=change-me knitnox
FROM --platform=$BUILDPLATFORM golang:1.26-alpine AS build
ARG TARGETOS TARGETARCH
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=$TARGETOS GOARCH=$TARGETARCH go build -trimpath -ldflags="-s -w" -o /knitnox ./cmd/knitnox

FROM alpine:3.21
RUN adduser -D -H knitnox && mkdir /data && chown knitnox /data
COPY --from=build /knitnox /usr/local/bin/knitnox
USER knitnox
ENV KNITNOX_DATA=/data KNITNOX_ADDR=:8080
VOLUME /data
EXPOSE 8080
ENTRYPOINT ["knitnox"]
