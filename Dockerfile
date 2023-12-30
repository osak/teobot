FROM node:18-alpine

COPY src /opt/teobot/src
COPY build/env.json package.json package-lock.json tsconfig.json /opt/teobot/
WORKDIR "/opt/teobot"

RUN npm install

ENTRYPOINT ["npm", "run", "cli-teokure", "server"]
