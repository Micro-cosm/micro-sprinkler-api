

FROM	golang:1.16-buster as builder

ARG		SERVICE
ENV		SERVICE=${SERVICE}

WORKDIR	/app

COPY	go.* ./
RUN		go mod download
COPY	./main.go ./main.go

RUN		go build -v -o ${SERVICE}



FROM	debian:buster-slim

RUN		set -x && apt-get update && DEBIAN_FRONTEND=noninteractive apt-get install -y ca-certificates &&  rm -rf /var/lib/apt/lists/*
																														# Copy the binary to the production image from the builder stage.
COPY	--from=builder /app/${SERVICE} /app/${SERVICE}
COPY	docker-entrypoint.sh /

EXPOSE	8080

ENTRYPOINT ["/docker-entrypoint.sh"]