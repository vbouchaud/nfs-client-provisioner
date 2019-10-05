FROM golang:1.13.1-alpine AS build
WORKDIR /usr/src
RUN apk add --no-cache gcc build-base upx
COPY . ./
RUN go build -o app -ldflags "-s -w"
RUN upx --best app

FROM alpine
WORKDIR /usr/src
COPY --from=build /usr/src/app /usr/src/
CMD ["/usr/src/app"]
