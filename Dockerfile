FROM oven/bun:1 AS bun-env
COPY ui /app
WORKDIR /app
RUN bun install
RUN bun run build


FROM golang:1.25.1-alpine AS go-env

#build project
COPY . /app
COPY --from=bun-env /app/dist /app/dist
WORKDIR /app
RUN go build -o /bin/goenc main.go

FROM alpine:latest
RUN apk add ffmpeg
WORKDIR /app
COPY --from=go-env /bin/goenc /bin/goenc
CMD ["/bin/goenc"]