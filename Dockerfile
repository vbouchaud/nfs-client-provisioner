FROM golang:1.18.2-alpine AS build

WORKDIR /usr/src
RUN apk add --no-cache \
    gcc=10.3.1_git20211027-r0 \
    build-base=0.5-r2

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
