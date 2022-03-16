FROM debian:stretch

ENV HOST localhost
ENV PORT 3000

WORKDIR /src

RUN apt-get update && \
    apt-get install -y git python-flask python-flup python-mysqldb python-pygraphviz python-networkx cron && \
    apt-get clean

# RUN git clone https://github.com/Arceliar/yggdrasil-map.git yggdrasil-map
RUN mkdir -p /src/yggdrasil-map/

COPY web /src/yggdrasil-map/web

COPY contrib/docker/web_config.cfg /src/yggdrasil-map/web/web_config.cfg

COPY contrib/docker/entrypoint.sh /usr/bin/entrypoint.sh
RUN chmod 0555 /usr/bin/entrypoint.sh

COPY contrib/docker/crontab /etc/cron.d/jobs
RUN chmod 0644 /etc/cron.d/jobs

ENTRYPOINT [ "/usr/bin/entrypoint.sh"]
