FROM golang:latest

COPY bin/cluebatbot.linux.amd64 cluebatbot-config.json /opt/cluebatbot/
WORKDIR /opt/cluebatbot/
RUN ls -lah

ENTRYPOINT [ "./cluebatbot.linux.amd64" ]