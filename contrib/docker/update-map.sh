#!/usr/bin/env bash

cd /src/yggdrasil-map/web/ && /src/yggdrasil-map/crawler > /nodes.json && python3 updateGraph.py
