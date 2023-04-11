FROM golang:1.20.3-alpine AS build

WORKDIR /usr/src
RUN apk add --no-cache \
    gcc=12.2.1_git20220924-r4 \
    build-base=0.5-r3

COPY go.mod ./
COPY go.sum ./

RUN go mod download

COPY . ./

RUN CGO_ENABLED=0 go build \
    -a \
    -o nfs-client-provisioner \
    main.go

FROM gcr.io/distroless/static:nonroot
WORKDIR /
COPY --from=build /usr/src/nfs-client-provisioner .
USER 65532:65532

ENTRYPOINT [ "/nfs-client-provisioner" ]
