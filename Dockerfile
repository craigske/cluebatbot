FROM golang:alpine

COPY bin/cluebatbot.linux.amd64 cluebatbot-config.json /opt/cluebatbot/
WORKDIR /opt/cluebatbot/

ENTRYPOINT [ "./cluebatbot.linux.amd64" ]