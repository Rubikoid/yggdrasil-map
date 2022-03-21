#!/usr/bin/env sh

set -e

cron
cd /src/yggdrasil-map/web/ && /src/yggdrasil-map/crawler > /nodes.json && python3 updateGraph.py
python3 /src/yggdrasil-map/web/web.py --host $HOST --port $PORT
exit $?
