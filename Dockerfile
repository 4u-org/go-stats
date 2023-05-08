FROM golang:1.20-alpine as builder 
WORKDIR /project/go-docker/ 

# COPY go.mod, go.sum and download the dependencies
COPY go.* ./
RUN go mod download 

# COPY All things inside the project and build
COPY . .
RUN go build -o /project/go-docker/build/app . 

# Copy the build file to alpine image
FROM alpine:latest
WORKDIR /app

RUN apk --no-cache add tzdata
ENV TZ=Europe/London

COPY --from=builder /project/go-docker/build/app /app/gostats

ENTRYPOINT [ "/app/gostats" ]