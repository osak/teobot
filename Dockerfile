FROM node:18-alpine

COPY src /opt/teobot/src
COPY package.json package-lock.json tsconfig.json /opt/teobot/
WORKDIR "/opt/teobot"

RUN npm install

ENTRYPOINT ["npm", "run", "cli-teokure", "server"]
