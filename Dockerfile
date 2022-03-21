FROM golang:1.16 AS builder

WORKDIR /src

COPY scripts/crawler.go ./
RUN CGO_ENABLED=0 GOOS=linux go build -a -o crawler crawler.go

FROM debian:bullseye

ENV HOST localhost
ENV PORT 3000

WORKDIR /src

RUN apt-get update && \
    apt-get install -y python3-flask python3-pygraphviz python3-networkx cron && \
    apt-get clean

RUN mkdir -p /src/yggdrasil-map/ /var/run/

COPY --from=builder /src/crawler /src/yggdrasil-map/

COPY web /src/yggdrasil-map/web

COPY contrib/docker/web_config.cfg /src/yggdrasil-map/web/web_config.cfg

COPY contrib/docker/entrypoint.sh /usr/bin/entrypoint.sh
RUN chmod 0555 /usr/bin/entrypoint.sh

COPY contrib/docker/crontab /etc/cron.d/jobs
RUN chmod 0644 /etc/cron.d/jobs

ENTRYPOINT [ "/usr/bin/entrypoint.sh"]
