FROM debian:bullseye

ENV HOST localhost
ENV PORT 3000

WORKDIR /src

RUN apt-get update && \
    apt-get install -y python3-flask python3-pygraphviz python3-networkx && \
    apt-get clean

RUN mkdir -p /src/yggdrasil-map/

COPY web /src/yggdrasil-map/web

COPY contrib/docker/web_config.cfg /src/yggdrasil-map/web/web_config.cfg

COPY contrib/docker/entrypoint.sh /usr/bin/entrypoint.sh
RUN chmod 0555 /usr/bin/entrypoint.sh

RUN python3 -V

ENTRYPOINT [ "/usr/bin/entrypoint.sh"]
