from flask import Flask, render_template, request

import argparse

parser = argparse.ArgumentParser()
parser.add_argument("--host", type=str, help="host to listen on (default 'localhost')", default="localhost")
parser.add_argument("--port", type=int, help="port to listen on (default '3000')", default=3000)

app = Flask(__name__)
app.config.from_pyfile("web_config.cfg")


def get_ip():
    try:
        ip = request.headers["x-real-ip"]
    except KeyError:
        ip = None
    return ip


@app.context_processor
def add_ip():
    return dict(ip=get_ip())


@app.route("/")
@app.route("/network")
def page_network():
    return render_template("network.html", page="network")


@app.route("/about")
def page_about():
    return render_template("about.html", page="about")


@app.after_request
def add_header(response):
    if response.content_type == "application/json":
        response.cache_control.max_age = 30
    return response


if __name__ == "__main__":
    args = parser.parse_args()
    app.run(host=args.host, port=args.port)
