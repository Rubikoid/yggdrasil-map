#!/usr/bin/env sh

set -e

cd /src/yggdrasil-map/web/ && python3 updateGraph.py
python3 /src/yggdrasil-map/web/web.py --host $HOST --port $PORT
exit $?
